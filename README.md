# affer

`affer` is a single-binary, model-agnostic coding-agent harness for Go 1.23+. It provides a JSONC-configured provider registry, OpenAI-compatible and Anthropic streaming transports, a native Gemini adapter, a permission-gated filesystem/shell tool registry, SQLite session history, and a Bubble Tea UI.

## Quick start

```sh
go run ./cmd/affer models --help
go run ./cmd/affer models

affer run "inspect the repository and add a version command" --model ollama/qwen3 --auto
```

Configuration is read from `~/.config/affer/config.jsonc` and the project `.affer.jsonc`, with the project taking precedence. API keys may be supplied with `{env:NAME}` references or environment variables. Keys are never printed by `providers list`.

```jsonc
{
  "model": "openai/gpt-5.1-codex",
  "provider": {
    "openai": {
      "type": "openai-compatible",
      "options": {
        "baseURL": "https://api.openai.com/v1",
        "apiKey": "{env:OPENAI_API_KEY}",
        "headers": {}
      }
    }
  },
  "permission": { "bash": "ask", "edit": "ask", "write": "ask", "webfetch": "allow", "*": "ask" }
}
```

`affer models` uses a 24-hour cache at `~/.cache/affer/models.json`; use `--refresh` to query configured OpenAI-compatible endpoints. Sessions and audit records live in `~/.local/share/affer/affer.db`.

## Milestone status

The initial implementation includes CLI/config/catalog, the two primary streaming transports plus Gemini, sessions/stats, and the read/write/edit/bash/glob/grep tool loop. LSP wire management, MCP discovery, OAuth/device login, WASM/JS hooks, web search gateway integration, full side-by-side diff rendering, self-upgrade downloads, and formatter-on-save orchestration are deliberately isolated follow-up milestones; see the end-of-turn report for details.
