# Agentic - Dual Output Protocol

This agent uses a **Dual Output Protocol** to separate human conversation from machine execution.

## Protocol
The LLM returns JSON:
```json
{
  "reply": "Short message for user",
  "ir": {
    "action": "act_now",
    "tools": [{"name": "tool_name", "args": {...}}]
  }
}
```

## Setup & Run
1. **Configuration**: 
   - Ensure `config.json` exists (see `config.example.json`).
   - Set `TELEGRAM_TOKEN` env var.
2. **Build**:
   ```bash
   go build -o agent ./cmd/agent
   ```
3. **Run**:
   ```bash
   ./agent
   ```

## Modules
- **Router**: `internal/router`. Deterministic intent matching (e.g., `/help`, `nota:`).
- **IR**: `internal/ir`. JSON structs and validation.
- **Scheduler**: `internal/scheduler`. File-backed job persistence (`jobs.json`) + Cron.
- **Tools**: `internal/tools`. Registry of tools (HTTP, Shell, Code, Notes).

## Adding a Tool
1. Create a struct implementing `tools.Tool`.
2. Register it in `cmd/agent/main.go`:
   ```go
   toolRegistry.Register(NewMyTool())
   ```
