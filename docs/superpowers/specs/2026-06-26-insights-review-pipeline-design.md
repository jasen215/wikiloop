# Insights 审核流水线设计

## 背景

WikiLoop 知识库当前只能通过人工放置文件到 `raw/` 来增长。引入 `kb_add` 工具后，外部 Agent 可以在对话结束时把洞察写入 `raw/insights/`，但这些内容质量参差不齐——有真正有价值的跨文档综合结论，也有无意义的对话记录。

本设计实现一条**异步 LLM 审核流水线**：Agent 通过 `kb_add` 把洞察写入 `raw/insights/`（作为 inbox 目录），由审核 worker 自动搜索知识库验证、评估质量，通过后 promote 到 `raw/reviewed/<分类>/`，触发正式蒸馏。`raw/insights/` 里的文件保留 1 年，不立即删除。

---

## insights 对知识库的价值定位

基于知识库实际数据分析，知识库存在三类不足，insights 对每类的价值不同：

| 不足类型 | 现状数据 | insights 价值 |
|---------|---------|--------------|
| **综合不足** | 1187 个综合页中 55% 仅引用 ≤2 篇 source-note，平均引用 2.6 篇 | ✅ 高：Agent 的跨文档综合结论直接补充综合页 sources |
| **文章关系不足** | 518 篇 source-note 之间 related_to 仅 39 条（平均每篇 0.08 条） | ✅ 高：Agent 发现文档间关联，补充 related_to 字段 |
| **内容不足** | 局部话题缺失（MemOS、新工具等） | ⚠️ 低：内容缺口靠新增原始文章，不靠 insights |

**结论：insights 的核心价值是弥补综合不足和关系不足，而不是补充原始内容。**

---

## insights 文件格式：一文件三 section

Agent 回答一个问题后，把所有发现写入**同一个文件**，按三类发现分 section。同一次对话上下文的内容放在一起，InsightReviewer 能看到完整语境，分类处理每个 section。

```markdown
# [问题主题] 查询洞察

## 跨文档关联发现
<!-- Agent 发现了两篇文档之间的关联，但它们的 related_to 字段未互指 -->
- [wiki/source-notes/A.md] 和 [wiki/source-notes/B.md] 在 X 概念上有直接关联

## 综合结论
<!-- Agent 综合了 3 篇以上文档，得出现有 comparison/concept 页没有覆盖的新角度 -->
综合 A、B、C 三篇关于 X 的内容，得出：...

## 外部补充
<!-- Agent 使用了知识库以外的信息（训练知识、外部搜索）补充了回答 -->
论文 X 的实验数据显示：...（来源：训练知识）

## 引用来源
- [wiki/source-notes/...]
- [wiki/source-notes/...]
```

**三类 section 的触发条件（写入 serverInstructions）：**

| Section | 何时写 | 不写的情况 |
|---------|--------|----------|
| **跨文档关联发现** | 发现两篇文档在某概念上相关，但 related_to 未互指 | 已有 comparison 页覆盖了这个关系 |
| **综合结论** | 综合 ≥3 篇文档，得出现有综合页没有的新角度 | 回答完全复述了已有 concept/comparison 页的内容 |
| **外部补充** | 使用了知识库以外的信息（训练知识/外部搜索） | 回答完全来自知识库已有文档 |

**不需要写 insights 的最常见情况：** Agent 的回答是对知识库已有 comparison/concept/decision 页的重新组织——这类内容知识库里已有，写入只会造成冗余。

InsightReviewer 按 section 差异化处理：
- **跨文档关联** → 验证两篇文档确实存在且关联合理 → promote 后蒸馏时补充 related_to
- **综合结论** → 验证现有综合页确实未覆盖该角度 → promote 为新综合页或补充已有页的 sources
- **外部补充** → 验证内容准确性 → promote 为新 source-note（`raw/reviewed/<category>/`）

---

## 整体数据流

```
Agent 对话结束
  └─ kb_add(filename="insights/YYYY-MM-DD-<slug>.md", content="...")
       └─ 写入 raw/insights/
            ├─ watcher 触发：FTS 索引（可搜索，Agent 可搜到历史 insights）
            ├─ watcher 触发：insights_queue 入队（status=pending）
            └─ 不触发 distill_queue（跳过正式蒸馏）

InsightWorker（后台 goroutine，1个）
  └─ 取 insights_queue pending 任务
       ├─ FTS 搜索知识库，找相关 source-note 作为参考资料
       ├─ 调 LLM 审核（准确性 + 必要性 + 质量 + 分类）
       │
       ├─ promote=true
       │    ├─ 写入 raw/reviewed/<category>/<slug>.md（格式化后的标准内容）
       │    ├─ distill_queue 入队（触发正式蒸馏）
       │    └─ 删除 raw/insights/<slug>.md
       │
       └─ promote=false
            ├─ insights_queue 标记 rejected（保留 reason）
            └─ 删除 raw/insights/<slug>.md
```

---

## 组件设计

### 1. `insights_queue` 表（新增）

```sql
CREATE TABLE IF NOT EXISTS insights_queue (
    path         TEXT PRIMARY KEY,   -- raw/ 相对路径，如 insights/2026-06-26-xxx.md
    status       TEXT NOT NULL DEFAULT 'pending',
                                     -- pending | processing | voted | promoted | rejected | failed | expired
    section_type TEXT,               -- link | synthesis | external（LLM 审核后填写）
    fingerprint  TEXT,               -- 建议语义指纹（LLM 生成，如 "related_to::A-B"）
    vote_count   INTEGER NOT NULL DEFAULT 1,  -- 同 fingerprint+section_type 的累计次数
    category     TEXT,               -- LLM 分配的分类，如 references、insights-synthesis
    reviewed_path TEXT,              -- promote 后的目标路径
    reason       TEXT,               -- LLM 审核理由
    retry_count  INTEGER NOT NULL DEFAULT 0,
    last_error   TEXT,
    queued_at    INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL    -- queued_at + 365天，到期标记 expired 并删除文件
);

CREATE INDEX IF NOT EXISTS idx_insights_fingerprint
    ON insights_queue(fingerprint, section_type);
```

### 2. `internal/distill/insights.go`（新文件，~150 行）

**职责：** LLM 审核逻辑，包含搜索、prompt 构建、结果解析、文件操作。

```go
// ReviewInsight 审核单个 insights 文件。
// 1. FTS 搜索相关 source-note 作为参考资料（最多 5 篇摘要）
// 2. 调 LLM 评估：准确性 + 必要性 + 质量 + 分类
// 3. promote=true → 格式化写入 raw/reviewed/<category>/，入队 distill_queue，删原文件
// 4. promote=false → 删原文件
// 返回 (promoted bool, category string, reason string, err error)
func ReviewInsight(cfg Config, kbRoot string, insightPath string) (bool, string, string, error)
```

**LLM prompt 结构：**

系统提示：
```
你是知识库质量审核员。你的任务是评估从外部 Agent 对话中提取的洞察片段，
决定是否值得正式入库，并在值得时重新整理为标准格式。
```

用户提示（动态构建）：
```
## 待审核洞察

<insights 文件全文>

## 知识库参考资料（FTS 搜索结果，最多 5 篇）

<source-note 标题 + description + snippet>

## 审核任务

该 insights 文件包含最多三类 section，请分别评估每个存在的 section：

**§ 跨文档关联发现**（若存在）
- 验证提到的两篇文档在知识库中确实存在
- 判断关联是否合理（非牵强）
- 判断现有综合页是否已覆盖这个关系
- 若有价值：输出需要补充 related_to 的文档对

**§ 综合结论**（若存在）
- 判断是否完全是对已有 concept/comparison 页的重复组织
- 若是重复 → reject（这是最常见的 reject 原因）
- 若有新角度：判断应补充到哪个现有综合页，还是新建一个

**§ 外部补充**（若存在）
- 验证内容与知识库参考资料是否一致，有无矛盾
- 判断是否有具体数据/事实（排除泛泛总结）
- 若有价值：确定写入 raw/reviewed/ 的子目录

请输出 JSON（不要包含在代码块中）：
{
  "promote": true 或 false,
  "reason": "一句话说明原因",
  "category": "子目录名（仅 promote=true 时填写）：references | insights-synthesis | decisions | project",
  "related_to_pairs": [["doc_a_path", "doc_b_path"]],  // 跨文档关联发现，空数组表示无
  "formatted_content": "重新格式化为标准 Markdown（仅 promote=true 且有外部补充或综合结论时填写，否则空字符串）"
}
```

**formatted_content 格式要求（注入 prompt）：**
```
formatted_content 必须是完整的 Markdown 文件，包含 YAML frontmatter：
  type: source-note
  title: <简洁标题>
  description: <1-2 句摘要，含具体技术术语>
  tags: [<3-6 个领域分类标签>]
  doc_type: <技术文章 | 分析报告 | 会议纪要 | 项目文档 之一>
  authority: <1-5，Agent 综合结论通常为 2-3>
  resource: ""
  sources: ["__RAW_SOURCE__"]
  timestamp: <ISO-8601，取 insights 文件中的日期，如无则用今天>
```

### 3. `internal/distill/worker.go`（改动）

在 `RunWorkers` 同时启动 `InsightWorker`（固定 1 个 goroutine，避免并发写文件冲突）：

```go
func RunWorkers(ctx context.Context, cfg Config, kbRoot string, n int) {
    // 原有 distill worker
    runWorkersWithFn(ctx, cfg, kbRoot, n, DistillFile)
    // 新增 insight review worker（固定 1 个）
    go insightWorkerLoop(ctx, cfg, kbRoot)
    <-ctx.Done()
}
```

`insightWorkerLoop` 复用 `workerLoop` 模式：取队列 → 审核 → 标记结果，失败指数退避，最多重试 3 次（LLM 审核比蒸馏轻量，3 次足够）。

### 4. `internal/watcher/watcher.go`（改动）

文件变化时，根据路径前缀分流：

```
raw/insights/ → FTS 索引 + insights_queue 入队（不进 distill_queue）
raw/（其他）  → 原有逻辑：FTS 索引 + distill_queue 入队
```

### 5. `internal/kb/service.go`（改动）

`KBStatus` 的 `StatusResult` 增加 `InsightsQueue` 字段：

```go
type StatusResult struct {
    Documents    int            `json:"documents"`
    ByLayer      map[string]int `json:"by_layer"`
    ByKind       map[string]int `json:"by_kind"`
    IndexPath    string         `json:"index_path"`
    IndexSize    int64          `json:"index_size"`
    DistillQueue map[string]int `json:"distill_queue"`
    InsightsQueue map[string]int `json:"insights_queue"` // 新增
}
```

`insightsQueueStats()` 内联 SQL，避免 distill↔kb 循环引用（与 `distillQueueStats()` 同模式）。

---

## 文件改动汇总

| 文件 | 类型 | 改动说明 |
|------|------|----------|
| `internal/kb/schema.go` | 改动 | 新增 `insights_queue` 表 DDL |
| `internal/kb/db.go` | 改动 | 新增 `insights_queue` migration |
| `internal/kb/service.go` | 改动 | `StatusResult` 增加 `InsightsQueue`，增加 `insightsQueueStats()` |
| `internal/distill/insights.go` | 新建 | `ReviewInsight` + prompt 构建 + 文件操作 |
| `internal/distill/worker.go` | 改动 | `RunWorkers` 启动 `insightWorkerLoop` |
| `internal/distill/queue.go` | 改动 | 新增 insights 队列操作函数（EnqueueInsight/NextPendingInsight/MarkInsightDone/MarkInsightFailed） |
| `internal/distill/distill.go` | 改动 | `FindNewFiles` 跳过 `raw/insights/` 路径 |
| `internal/watcher/watcher.go` | 改动 | `raw/insights/` 路径分流，不进 distill_queue |
| `internal/mcp/server.go` | 改动（已做）| serverInstructions 加 KNOWLEDGE CAPTURE 段，引导 Agent 写 insights |

---

## 投票机制：防止 LLM 抖动

单次 LLM 输出不可靠（抖动、幻觉、偶发错误），同一类建议需要被**多次独立观察**才 promote。

### 规则

- **同 section 类型**的相似建议计数，跨 section 不累计（因为三类 section 更新知识库的不同部分）
- 同类建议累计 **≥2 次**时 promote（2 次足以过滤偶发抖动，不需要等 3 次）
- insights 文件**保留 1 年**后清理（作为历史投票记录，不立即删除）

### 实现

`insights_queue` 表新增字段：

```sql
ALTER TABLE insights_queue ADD COLUMN section_type TEXT;   -- link | synthesis | external
ALTER TABLE insights_queue ADD COLUMN fingerprint TEXT;    -- 建议的语义指纹（LLM 生成的 slug）
ALTER TABLE insights_queue ADD COLUMN vote_count INTEGER NOT NULL DEFAULT 1;
ALTER TABLE insights_queue ADD COLUMN expires_at INTEGER; -- queued_at + 365天
```

InsightReviewer 处理流程变更：

```
取 insights_queue pending 任务
  └─ FTS 搜索知识库（参考资料，≤5 篇）
  └─ FTS 搜索 raw/insights/（历史 insights，≤5 篇）
  └─ 调 LLM 审核，新增输出字段：
       section_type: "link" | "synthesis" | "external"
       fingerprint:  建议的语义指纹（如 "related_to::A-B", "synthesis::FTS优于向量"）
  └─ 写回 insights_queue（section_type + fingerprint）
  └─ 查询：同 fingerprint + 同 section_type 的 promoted=false 记录数
       count >= 2 → promote（写入 raw/reviewed/ + 触发蒸馏）
       count < 2  → 标记 voted（等待下次同类建议）
```

### `insights_queue` status 扩展

```
pending     → 待审核
processing  → 审核中
voted       → 已记录投票，等待累计（保留文件）
promoted    → 已 promote（写入 raw/reviewed/）
rejected    → 审核不通过（删除文件）
failed      → 重试耗尽
expired     → 超过 1 年清理
```

### 清理策略

- `promoted` / `rejected` 文件：审核完成后**保留原文件 1 年**再删除
- `voted` 文件：等待同类建议累计，1 年内未达到 ≥2 次则标记 `expired` 并删除
- `kb_lint` 输出 `voted` 类建议的当前计数，方便查看哪些建议正在积累中

---

## 关键边界与约束

**insights 目录不触发正式蒸馏：**
`FindNewFiles` 和 `Enqueue` 需要跳过 `raw/insights/` 路径，防止 insights 文件被普通 distill worker 处理。在 `FindNewFiles` 里加一行路径前缀判断即可。

**formatted_content 由 LLM 生成，写入前不做二次验证：**
直接写入 `raw/reviewed/<category>/`，交给后续正式蒸馏流程处理。如果 LLM 输出格式有问题，蒸馏阶段会自然处理（已有健壮的 frontmatter 解析逻辑）。

**InsightWorker 固定 1 个 goroutine：**
避免并发写同名文件冲突。insights 审核不是高吞吐场景，1 个 worker 足够。

**FTS 搜索参考资料上限 5 篇：**
超过 5 篇 prompt 过长，LLM 可能忽略后半部分。取 BM25 top-5（知识库参考）+ top-5（历史 insights）各自搜索，分两段注入 prompt。

**重试上限 3 次：**
3 次均失败（LLM API 不可用等）后标记 `failed`，原文件保留至 expires_at 再删除。

---

## serverInstructions 引导（已完成）

`internal/mcp/server.go` 中 `KNOWLEDGE CAPTURE` 段已加入，引导 Agent：
- 综合结论不在知识库中时，调 `kb_add(filename="insights/YYYY-MM-DD-<slug>.md", ...)`
- 明确说明不应写回的场景（已有内容、不确定、session 级琐碎信息）

---

## 不在本设计范围内

- WebUI 展示 insights_queue 状态（可后续加到 status 页面，当前 `KBStatus` API 已包含数据）
- 人工干预 promote（当前全自动，如需人工审核可后续加 `wikiloop insights list/approve` 命令）
- insights 内容的向量索引（无向量模块，FTS 已足够）
