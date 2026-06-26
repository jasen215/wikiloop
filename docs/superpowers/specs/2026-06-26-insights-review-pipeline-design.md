# Insights 推荐日志流水线设计

## 背景

WikiLoop 知识库当前只能通过人工放置文件到 `raw/` 来增长。引入 `kb_add` 工具后，外部 Agent 可以在对话结束时把洞察写入 `raw/insights/`，作为推荐日志。

`raw/insights/` 定位是**推荐日志目录**，不是正式知识来源：
- Agent 觉得有价值就写，写错了也没关系
- 定期有 worker 扫描，有增量价值的内容自动进入正式流程
- 没有价值的内容 3 个月后自动删除
- 不投入过大资源，整体是轻量的"有则用，无则丢"机制

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
            ├─ watcher：FTS 索引（Agent 可搜到历史 insights）
            └─ 不触发 distill_queue（跳过正式蒸馏）

InsightWorker（定时，每周一次）
  └─ 扫描 raw/insights/ 所有文件
       └─ 对每个文件：
            ├─ FTS 搜索知识库，找相关 source-note（≤5 篇）作为参考
            ├─ 调 LLM 判断：对知识库是否有增量价值？
            │
            ├─ 有价值
            │    └─ 写入 raw/reviewed/<category>/<slug>.md
            │         └─ distill_queue 入队（触发正式蒸馏）
            │
            └─ 无价值 / 重复
                 └─ 跳过（文件原地保留，等 3 个月到期删除）

定时清理（每天一次）
  └─ 删除 raw/insights/ 中超过 90 天的文件
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

`raw/insights/` 路径只触发 FTS 索引，不进 `distill_queue`：

```go
if strings.HasPrefix(rel, "insights/") {
    // 只索引，不蒸馏
    kb.IndexFiles(db, kbRoot)
    return
}
// 原有逻辑
```

### 4. 定时清理

在 `RunInsightWorker` 的每周扫描循环里附带清理：

```go
// 删除超过 90 天的 raw/insights/ 文件
cutoff := time.Now().AddDate(0, -3, 0)
filepath.Walk(insightsDir, func(path string, info os.FileInfo, err error) error {
    if err == nil && !info.IsDir() && info.ModTime().Before(cutoff) {
        os.Remove(path)
    }
    return nil
})
```

---

## 文件改动汇总

| 文件 | 类型 | 改动说明 |
|------|------|----------|
| `internal/distill/insights_worker.go` | 新建 | InsightWorker：定时扫描 + LLM 审核 + promote |
| `internal/distill/distill.go` | 改动 | `FindNewFiles` 跳过 `raw/insights/` |
| `internal/watcher/watcher.go` | 改动 | `raw/insights/` 只索引不蒸馏 |
| `internal/mcp/server.go` | 已完成 | serverInstructions KNOWLEDGE CAPTURE 段 |

**不需要：**
- ~~`insights_queue` 表~~（无状态机）
- ~~schema.go / db.go 改动~~
- ~~service.go 改动~~

---

## 关键约束

**insights 不触发正式蒸馏：** `FindNewFiles` 和 watcher 均跳过 `raw/insights/`。

**InsightWorker 每周一次：** insights 是低优先级后台任务，不需要实时处理。

**FTS 参考资料 ≤5 篇：** prompt 长度控制，取 BM25 top-5 摘要（title + description + snippet）。

**promote 失败静默跳过：** LLM API 不可用时跳过该文件，下次周期再试，不需要重试队列。

**3 个月自动清理：** 按文件 ModTime 判断，无需额外字段。

---

## serverInstructions 引导（已完成）

`internal/mcp/server.go` 的 KNOWLEDGE CAPTURE 段已引导 Agent：
- 触发条件：发现文档关联未互指、综合出新角度、使用了外部信息
- 不写条件：回答完全来自已有综合页（最常见情况）
- 格式：一文件三 section，只写有内容的 section
