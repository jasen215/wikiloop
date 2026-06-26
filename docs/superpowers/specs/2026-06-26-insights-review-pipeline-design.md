# Insights 推荐日志流水线设计

## 背景

WikiLoop 知识库当前只能通过人工放置文件到 `raw/` 来增长。引入 `kb_add` 工具后，外部 Agent 可以在对话结束时把洞察写入 `raw/insights/`，作为推荐日志。

`raw/insights/` 定位是**推荐日志目录**，不是正式知识来源：
- Agent 觉得有价值就写，写错了也没关系
- 定期有 worker 扫描，处理完立即删除（无论采用还是跳过）
- 不投入过大资源，整体是轻量的"有则用，无则丢"机制
- worker 审核采用**严格标准**：宁可不用，不要用错

---

## insights 对知识库的价值

基于知识库实际数据，insights 主要弥补以下两类不足：

| 不足类型 | 现状 | insights 能补充什么 |
|---------|------|-------------------|
| **文章关系不足** | 518 篇 source-note，related_to 仅 39 条 | Agent 发现两篇文档有关联但未互指 |
| **综合不足** | 55% 综合页仅引用 ≤2 篇 source-note | Agent 综合多篇得出现有综合页未覆盖的新角度 |

**不适合写 insights 的内容：** 对已有 concept/comparison/decision 页的重新组织——这类内容写入只会造成冗余。

---

## insights 文件格式

Agent 回答一个问题后，把所有发现写入**同一个文件**，按发现类型分 section，只写有内容的 section：

```markdown
# [问题主题] 查询洞察

## 跨文档关联发现
<!-- 发现两篇文档有关联，但 related_to 未互指 -->
- [wiki/source-notes/A.md] 和 [wiki/source-notes/B.md] 在 X 概念上有直接关联

## 综合结论
<!-- 综合 ≥3 篇文档，得出现有综合页没有覆盖的新角度 -->
综合 A、B、C 三篇关于 X 的内容，得出：...

## 外部补充
<!-- 使用了知识库以外的信息（训练知识、外部搜索） -->
论文 X 的实验数据显示：...

## 引用来源
- [wiki/source-notes/...]
```

---

## 整体数据流

```
Agent 对话结束
  └─ kb_add(filename="insights/YYYY-MM-DD-<slug>.md", content="...")
       └─ 写入 raw/insights/
            └─ 不触发 distill_queue（跳过正式蒸馏，不做 FTS 索引）

InsightWorker（定时，每周一次）
  └─ 扫描 raw/insights/ 所有文件
       └─ 对每个文件：
            ├─ FTS 搜索知识库，找相关 source-note（≤5 篇）作为参考
            ├─ 调 LLM 严格审核：对知识库是否有确定的增量价值？
            │   （宁可不用，不要用错）
            │
            ├─ 有确定价值
            │    ├─ 写入 raw/reviewed/<category>/<slug>.md
            │    ├─ distill_queue 入队（触发正式蒸馏）
            │    └─ 删除 raw/insights/<slug>.md（立即）
            │
            └─ 无价值 / 不确定 / 重复
                 └─ 删除 raw/insights/<slug>.md（立即）
```

---

## 组件设计

### 1. `internal/distill/insights_worker.go`（新文件，~80 行）

```go
// RunInsightWorker 每周扫描一次 raw/insights/，有价值的内容 promote 到 raw/reviewed/。
func RunInsightWorker(ctx context.Context, cfg Config, kbRoot string)

// reviewInsightFile 审核单个文件。
// 返回 (category, formattedContent, skip, err)
// skip=true 表示无增量价值，直接跳过。
func reviewInsightFile(cfg Config, kbRoot string, path string) (string, string, bool, error)
```

**LLM prompt 结构：**

系统提示：
```
你是知识库质量审核员。评估从外部 Agent 对话中提取的洞察，
判断对知识库是否有增量价值。
```

用户提示：
```
## 待审核洞察
<insights 文件全文>

## 知识库参考资料（FTS 搜索结果，≤5 篇）
<source-note 标题 + description + snippet>

## 审核标准

判断以下任一条件是否成立：
1. 发现了两篇文档的关联，但知识库中未建立 related_to 链接
2. 综合结论有新角度，现有 concept/comparison/decision 页未覆盖
3. 包含知识库以外的具体数据或事实

不符合以上任一条件（如：仅是已有综合页的重新组织）→ skip

输出 JSON：
{
  "skip": true 或 false,
  "reason": "一句话",
  "category": "references | insights-synthesis | decisions（skip=false 时填）",
  "formatted_content": "标准 Markdown 全文（skip=false 时填，否则空字符串）"
}

formatted_content 需包含 YAML frontmatter：
  type: source-note
  title, description, tags, doc_type, authority(2-3), resource:"", sources:["__RAW_SOURCE__"], timestamp
```

### 2. `internal/distill/distill.go`（改动）

`FindNewFiles` 跳过 `raw/insights/` 路径，防止被普通蒸馏 worker 处理：

```go
if strings.HasPrefix(rel, "insights"+string(filepath.Separator)) {
    return nil
}
```

### 3. `internal/watcher/watcher.go`（改动）

`raw/insights/` 路径**跳过所有处理**（不索引、不蒸馏）：

```go
if strings.HasPrefix(rel, "insights/") {
    return // 跳过，由 InsightWorker 定期处理
}
// 原有逻辑
```

---

## 文件改动汇总

| 文件 | 类型 | 改动说明 |
|------|------|----------|
| `internal/distill/insights_worker.go` | 新建 | InsightWorker：每周扫描 + LLM 严格审核 + promote + 立即删除 |
| `internal/distill/distill.go` | 改动 | `FindNewFiles` 跳过 `raw/insights/` |
| `internal/watcher/watcher.go` | 改动 | `raw/insights/` 跳过所有处理（不索引不蒸馏） |
| `internal/mcp/server.go` | 已完成 | serverInstructions KNOWLEDGE CAPTURE 段 |

**不需要：**
- ~~`insights_queue` 表~~（无状态机）
- ~~schema.go / db.go 改动~~
- ~~service.go 改动~~

---

## 关键约束

**insights 不触发蒸馏也不做 FTS 索引：** `FindNewFiles`、watcher、FTS 索引均跳过 `raw/insights/`，文件仅供 InsightWorker 读取。

**InsightWorker 每周一次：** insights 是低优先级后台任务，不需要实时处理。

**处理完立即删除：** 无论 promote 还是跳过，文件处理后立即删除。无需定时清理任务，无需记录历史。

**严格审核，宁可不用：** LLM prompt 中明确要求——不确定时输出 skip=true。有价值的判断标准必须满足其中一条：发现未记录的文档关联、综合出现有综合页未覆盖的新角度、包含知识库外的具体可验证数据。

**FTS 参考资料 ≤5 篇：** prompt 长度控制，取 BM25 top-5 摘要（title + description + snippet）。

**LLM API 不可用时静默跳过：** 不重试，下次周期再处理剩余文件。

---

## serverInstructions 引导（已完成）

`internal/mcp/server.go` 的 KNOWLEDGE CAPTURE 段已引导 Agent：
- 触发条件：发现文档关联未互指、综合出新角度、使用了外部信息
- 不写条件：回答完全来自已有综合页（最常见情况）
- 格式：一文件三 section，只写有内容的 section
