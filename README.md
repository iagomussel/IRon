AGENTIC Telegram Codex Agent (Go)

Overview
- Receives prompts from Telegram, runs codex exec with full permissions, and sends the response back.
- Uses Codex CLI --last to resume the most recent session.
- Exposes a tools HTTP server so Codex can call tools (http_fetch, shell_exec, docker_exec, code_exec).
- Supports adapters and addons so you can extend behavior without editing core code.
- Scheduler runs tasks on cron and sends results through adapters.

Quick start
1) Build
   - Install Go 1.22+.
   - Build: go build ./cmd/agent

2) Configure
   - Create config.json in this folder (see example below).

3) Run
   - ./agent

config.json example
{
  "telegram_token": "YOUR_BOT_TOKEN",
  "allowed_chat_ids": [123456789],
  "codex_command": ["codex", "exec", "--dangerously-bypass-approvals-and-sandbox", "-"],
  "codex_env": ["CODEX_SOMETHING=1"],
  "data_dir": "data",
  "tools_addr": ":8089",
  "max_response_size": 3500,
  "tasks": [
    {
      "id": "daily-summary",
      "cron": "0 9 * * *",
      "prompt": "Resuma as prioridades do dia.",
      "adapter": "telegram",
      "targets": ["123456789"]
    }
  ],
  "addons": [
    {
      "name": "my-tool",
      "type": "tool",
      "repo": "git@github.com:org/my-tool.git",
      "build": ["bash", "-lc", "make build"],
      "binary": "bin/my-tool",
      "tool_name": "my_tool"
    }
  ]
}

Environment overrides
- TELEGRAM_TOKEN
- TELEGRAM_ALLOWED_CHAT_IDS (comma-separated)
- CODEX_COMMAND (string, space-separated)
- CODEX_ENV (comma-separated)
- DATA_DIR
- TOOLS_ADDR
- MAX_RESPONSE_SIZE

Tools server
- GET /tools/list -> {"tools":[...]} 
- POST /tools/execute with {"name":"tool","input":{...}}

Builtin tools
- http_fetch: GET a URL
- shell_exec: run local commands
- docker_exec: run docker CLI
- code_exec: run short Go/Python/Bash snippets

Telegram commands
- /tools: lists available tools
- /help: shows help

Notes
- Codex CLI handles session continuity via --last.
