# IRon (Intermediate Representation on Telegram)

IRon is a Telegram Orchestrator designed for **token efficiency** and **structured reliability**.

Unlike chatty chatbots that burn tokens on conversational fluff, IRon uses a strict **Dual-Output Protocol**. It forces the LLM to "think" in structured actions (IR) and "speak" in concise replies (Human Layer).

## Core Philosophy: The Token Economy

Every token costs latency and money. To optimize this:

1.  **Deterministic Router**: High-confidence inputs (like `/help` or `note: buy milk`) never touch the LLM. Zero cost.
2.  **Strict IR Protocol**: The LLM outputs a single JSON block containing both the machine instruction and the human reply. No follow-up clarification loops unless explicitly requested via `action="ask"`.
3.  **Concise Context**: We inject only relevant metadata (Time, User ID, active directory) instead of dumping massive history.

## The Dual-Output Protocol

Every interaction with the Intelligence Layer results in a strictly validated JSON packet:

```json
{
  "reply": "Scheduled for Friday at 19:00.",
  "ir": {
    "action": "schedule",
    "intent": "budget.close_weekly",
    "risk": "low",
    "when": "5 0 * * *", 
    "tools": [
      {
        "name": "schedule", 
        "args": {
          "spec": "30m",
          "message": "Check budget",
          "target": "USER_ID"
        }
      }
    ],
    "confidence": 0.95
  }
}
```

*   **`reply`**: User-facing text. Short, direct, max 2 lines.
*   **`ir`**: Machine-readable payload. Executed by the Go orchestrator.

## Architecture

*   **Router (`internal/router`)**: regex/prefix matcher for instant responses.
*   **Orchestrator (`cmd/agent`)**: Manages the pipeline (Route -> LLM -> Validate -> Execute). It handles the "Repair Loop" if the LLM outputs broken JSON.
*   **Scheduler (`internal/scheduler`)**: Handles time-based triggers using standard `cron` expressions or Go `time.Duration` structs. Persistent via `jobs.json`.
*   **Tools (`internal/tools`)**: Go functions exposed to the IR layer (e.g., File I/O, Shell execution).

## BlueprintDSL

For complex code generation or multi-step reasoning, we don't stream code directly to the chat. Instead, the IR contains a **BlueprintDSL**: a compact, declarative description of the intended system state.

*(Future implementation: The Orchestrator will expand BlueprintDSL into actual file modifications)*

## Getting Started

1.  **Clone**: `git clone https://github.com/iagomussel/IRon`
2.  **Config**: Set `TELEGRAM_TOKEN` and your LLM endpoint in `config.json`.
3.  **Run**: `go build -o iron ./cmd/agent && ./iron`
