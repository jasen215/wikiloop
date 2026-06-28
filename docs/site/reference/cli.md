# CLI Reference

All commands accept a global `--kb <path>` flag (defaults to `$WIKILOOP_KB`, then `~/wikiloop-kb`).

## Commands

| Command | Description |
|---|---|
| `wikiloop init [--force]` | Scaffold KB dirs and copy bundled schema/templates |
| `wikiloop serve` | Start the long-running server: HTTP MCP + Web UI + file watcher. Default when no subcommand is given. |
| `wikiloop index` | Build/update the FTS index from `wiki/` and `raw/` markdown |
| `wikiloop search <query>` | FTS keyword search; prints ranked hits with paths and snippets |
| `wikiloop synthesize [--topic X] [--full]` | Generate concept/comparison/decision pages from source-notes |
| `wikiloop synthesize --gaps --topic X` | Knowledge-gap analysis for a topic |
| `wikiloop import-lark <URL>` | Import a Lark/Feishu Wiki page into `raw/lark/` |
| `wikiloop lint` | Health-check wiki pages: missing frontmatter, broken source links |
| `wikiloop status` | Print index stats (document counts, index size) |
| `wikiloop service <install\|uninstall\|start\|stop\|status\|logs>` | Manage the OS service (launchd / systemd) |

## System Service

To make WikiLoop start on boot and run in the background:

```bash
wikiloop service install --kb /path/to/your-kb
wikiloop service status
wikiloop service uninstall
```

Logs: `{WIKILOOP_KB}/index/watcher.log`
