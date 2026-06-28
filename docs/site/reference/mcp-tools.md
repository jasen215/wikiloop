# MCP Tools Reference

WikiLoop exposes two MCP tools for agents.

## kb_search

Search the knowledge base with a keyword or phrase.

**Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `query` | string | yes | Search keyword or phrase |
| `kind` | string | no | Filter page kind: `source-note`, `concept`, `comparison`, `decision` |
| `layer` | string | no | Filter layer: `wiki`, `raw`, or `schema` |
| `limit` | number | no | Maximum results (default 10) |

**Returns:** Ranked list of matching pages, each with:
- `id` — page identifier for use with `kb_page`
- `title`, `snippet` — preview of the matched content
- `kind`, `layer` — page classification
- `related` — linked documents for graph navigation

**Example:**

```json
{
  "query": "RAG retrieval augmented generation",
  "limit": 5
}
```

## kb_page

Fetch full content of one or more pages by ID.

**Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `ids` | array | yes | Document IDs from `kb_search` results (1–5) |
| `full` | boolean | no | Return complete untruncated text (only with single ID) |

**Returns:** Full Markdown content of each requested page.

**Example:**

```json
{
  "ids": ["source-notes/my-doc", "concepts/rag-overview"],
  "full": false
}
```
