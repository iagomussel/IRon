# Repository Guidelines

## Project Structure & Module Organization
- `cmd/agent/`: main entrypoint for the Telegram agent binary.
- `internal/`: core packages (config, codex client, adapters, tools server, scheduler).
- `config.json`: runtime configuration file (see `README.md` example).
- `agent`: compiled binary produced by `go build` (do not commit).

## Build, Test, and Development Commands
- `go build ./cmd/agent`: builds the `agent` binary in the repo root.
- `./agent`: runs the compiled binary using `config.json`.
- `go test ./...`: runs all Go tests (currently none; keep command for future).

## Coding Style & Naming Conventions
- Go 1.22+ module (`go.mod`); follow standard Go formatting via `gofmt`.
- Packages are organized by responsibility under `internal/` (e.g., `internal/tools`).
- Prefer clear, short package names and keep exported identifiers PascalCase.

## Testing Guidelines
- No `_test.go` files are present yet; add tests alongside packages in `internal/`.
- Use Go’s standard `testing` package unless a new framework is justified.
- Name tests `TestXxx` and use table-driven patterns where it improves coverage.

## Commit & Pull Request Guidelines
- This directory is not a Git repository, so no commit history is available.
- If you initialize Git, keep commit messages short and imperative (e.g., “Add tools server timeout”).
- PRs should describe changes, include config or behavior impacts, and note any new env vars.

## Security & Configuration Tips
- Keep secrets out of `config.json`; prefer env overrides (e.g., `TELEGRAM_TOKEN`).
- Avoid committing `data/` contents; it stores session state.
- When adding tools or addons, document required binaries and permissions in `README.md`.
