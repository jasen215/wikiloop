# 快速入门

## 1. 初始化知识库

```bash
export WIKILOOP_KB=/path/to/your-kb

wikiloop init    # 初始化目录结构，复制 schema/templates
wikiloop serve   # 启动服务：MCP + Web UI + 文件监听
```

> macOS 用户可直接双击 WikiLoop.app 以菜单栏图标启动。

## 2. 在 Agent 中配置 MCP

**HTTP 模式**（推荐 — 单进程共享给所有 Agent）：

在 `~/.claude.json` 的 `mcpServers` 中添加：

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

`x-api-key` 对应 `config.yaml` 中的 `server.api_key`，未设置 api_key 时可省略 `headers`。

**stdio 模式**（适用于托管环境）：

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

## 3. 添加内容

将任意 Markdown 文件放入 `raw/` 目录，watcher 会自动提炼。

```bash
cp my-notes.md $WIKILOOP_KB/raw/
# watcher 检测到文件变化后自动触发提炼 + 重建索引
```

## 4. 搜索

```bash
wikiloop search "你的查询"
```

或让 Agent 使用 MCP 工具：

```text
kb_search("关键词 A")              → 发现相关文档
kb_search("关键词 B")              → 换角度覆盖
kb_page(["id1", "id2"])           → 深度阅读最相关的文档
```
