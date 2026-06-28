# MCP Server

WikiLoop exposes KB tools via the MCP protocol. Two transport modes are supported.

## HTTP Mode (Recommended)

One WikiLoop process shared by all agents — Claude Code, Cursor, VS Code (Copilot), Windsurf, and others.

**Start WikiLoop:**

```bash
export WIKILOOP_KB=/path/to/wikiloop-kb
wikiloop serve
```

**Configure each agent:**

```json
{
  "mcpServers": {
    "wikiloop": {
      "type": "http",
      "url": "http://127.0.0.1:8766/mcp",
      "headers": {
        "x-api-key": "${WIKILOOP_API_KEY}"
      }
    }
  }
}
```

`x-api-key` corresponds to `server.api_key` in `config.yaml`. Omit `headers` if no api_key is set.

## stdio Mode

For hosted environments (Hermes, OpenClaw, etc.) where WikiLoop runs as a subprocess.

```json
{
  "mcpServers": {
    "wikiloop": {
      "type": "stdio",
      "command": "/path/to/wikiloop",
      "args": ["stdio"],
      "env": {
        "WIKILOOP_KB": "/path/to/your-kb"
      }
    }
  }
}
```

The KB directory is created automatically on first launch. No manual `init` needed.

## Available Tools

| Tool | Description |
|---|---|
| `kb_search` | FTS keyword search, returns ranked results with related links |
| `kb_page` | Fetch full page content by ID |

Admin operations (`status`, `reindex`, `lint`) are available via the Web UI or CLI.
