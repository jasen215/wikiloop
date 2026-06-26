# Insights 审核流水线设计

## 背景

WikiLoop 知识库当前只能通过人工放置文件到 `raw/` 来增长。引入 `kb_add` 工具后，外部 Agent 可以在对话结束时把洞察写入 `raw/insights/`，但这些内容质量参差不齐——有真正有价值的跨文档综合结论，也有无意义的对话记录。

本设计实现一条**异步 LLM 审核流水线**：Agent 写入的 insights 先进 inbox，由审核 worker 自动搜索知识库验证、评估质量、格式化，通过后自动 promote 到 `raw/reviewed/<分类>/`，触发正式蒸馏。原始 insights 文件作为过程文件，处理后删除。

---

## 整体数据流

```
Agent 对话结束
  └─ kb_add(filename="insights/YYYY-MM-DD-<slug>.md", content="...")
       └─ 写入 raw/insights/（inbox）
            ├─ watcher 触发：FTS 索引（可搜索）
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
    path        TEXT PRIMARY KEY,          -- raw/ 相对路径，如 insights/2026-06-26-xxx.md
    status      TEXT NOT NULL DEFAULT 'pending',  -- pending | processing | promoted | rejected
    category    TEXT,                      -- LLM 分配的分类，如 references、wechat-tech
    reviewed_path TEXT,                    -- promote 后的目标路径，如 reviewed/references/xxx.md
    reason      TEXT,                      -- LLM 审核理由（promoted 或 rejected 均填写）
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_error  TEXT,
    queued_at   INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);
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

请根据以下标准评估：

1. **准确性**：与参考资料是否一致？有无明显错误或与已知事实矛盾？
2. **必要性**：知识库中是否已有等价内容？若已有，该洞察是否有显著补充？
3. **质量**：内容是否具体、有价值？排除以下无意义内容：
   - 纯粹的对话记录（"今天讨论了X"）
   - 无具体结论的泛泛总结
   - 字数少于 100 字的片段
4. **分类**：适合放在 raw/reviewed/ 的哪个子目录？常见分类：
   - references（技术参考、方法论）
   - wechat-tech（微信公众号技术文章）
   - insights-synthesis（跨文档综合结论）
   - decisions（技术决策记录）
   - project（项目文档、会议纪要）

请输出 JSON（不要包含在代码块中）：
{
  "promote": true 或 false,
  "reason": "一句话说明原因",
  "category": "子目录名（仅 promote=true 时填写，否则空字符串）",
  "formatted_content": "重新格式化为标准 Markdown 的完整内容（仅 promote=true 时填写，否则空字符串）"
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

## 关键边界与约束

**insights 目录不触发正式蒸馏：**
`FindNewFiles` 和 `Enqueue` 需要跳过 `raw/insights/` 路径，防止 insights 文件被普通 distill worker 处理。在 `FindNewFiles` 里加一行路径前缀判断即可。

**formatted_content 由 LLM 生成，写入前不做二次验证：**
直接写入 `raw/reviewed/<category>/`，交给后续正式蒸馏流程处理。如果 LLM 输出格式有问题，蒸馏阶段会自然处理（已有健壮的 frontmatter 解析逻辑）。

**InsightWorker 固定 1 个 goroutine：**
避免并发写同名文件冲突。insights 审核不是高吞吐场景，1 个 worker 足够。

**FTS 搜索参考资料上限 5 篇：**
超过 5 篇 prompt 过长，LLM 可能忽略后半部分。取 BM25 top-5 摘要（title + description + snippet）即可，不需要全文。

**promote 失败不重试超过 3 次：**
3 次均失败（LLM API 不可用等）后标记 `failed`，原文件删除（insights 是 inbox，不值得无限保留）。

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
