# MCP 工具参考

WikiLoop 为 Agent 暴露两个 MCP 工具。

## kb_search

用关键词或短语搜索知识库。

**参数：**

| 参数 | 类型 | 必填 | 描述 |
|---|---|---|---|
| `query` | string | 是 | 搜索关键词或短语 |
| `kind` | string | 否 | 过滤页面类型：`source-note`、`concept`、`comparison`、`decision` |
| `layer` | string | 否 | 过滤层：`wiki`、`raw`、`schema` |
| `limit` | number | 否 | 最大结果数（默认 10） |

**返回：** 按相关度排序的匹配页面列表，每条包含：
- `id` — 页面标识符，用于 `kb_page`
- `title`、`snippet` — 匹配内容预览
- `kind`、`layer` — 页面分类
- `related` — 关联文档，用于图谱导航

## kb_page

通过 ID 获取一个或多个页面的完整内容。

**参数：**

| 参数 | 类型 | 必填 | 描述 |
|---|---|---|---|
| `ids` | array | 是 | `kb_search` 结果中的文档 ID（1–5 个） |
| `full` | boolean | 否 | 返回完整不截断文本（仅对单个 ID 有效） |

**返回：** 每个请求页面的完整 Markdown 内容。
