# Installation

## Download Pre-built Binary

| Platform | File |
|---|---|
| macOS Apple Silicon (ARM64) | `WikiLoop-<version>-macos-arm64.dmg` |
| Linux x86_64 | `wikiloop-<version>-linux-amd64.tar.gz` |
| Linux ARM64 | `wikiloop-<version>-linux-arm64.tar.gz` |
| Windows x86_64 | `wikiloop-<version>-windows-amd64.zip` |

Download from [GitHub Releases](https://github.com/jasen215/wikiloop/releases).

> **macOS Intel (x86_64):** No pre-built release. Build from source: `CGO_ENABLED=1 go build -tags fts5 -o wikiloop ./cmd/wikiloop/`

## macOS

Open the DMG and drag WikiLoop to Applications. The app runs as a menubar icon.

## Linux

```bash
tar -xzf wikiloop-<version>-linux-amd64.tar.gz -C /path/to/install/
sudo ln -sf /path/to/install/wikiloop /usr/local/bin/wikiloop
```

## Windows

Extract the zip and run `wikiloop.exe serve` (or `wikiloop.exe stdio` for MCP). Add the directory to `PATH` for convenience.

## Build from Source

Requires Go 1.25+.

```bash
# macOS / Linux
go build -tags fts5 -o wikiloop ./cmd/wikiloop/

# Windows
go build -tags fts5 -o wikiloop.exe ./cmd/wikiloop/
```

Or use the multi-platform build script:

```bash
./scripts/build.sh [version] [target...]
```
