# filebot-webui — Claude Config

Read in ~30 seconds. Detailed rules in `.claude/rules/`.

## What This App Does

Web UI: authenticate via Plex, list completed Deluge torrents, run FileBot to rename+move files to media folder, delete torrent+data on success, refresh Plex library.

## Architecture

See `.claude/rules/01-architecture.md`.

Layer rule: `handler → service → domain ← repository`
- `internal/domain/` — value types + sentinel errors. Zero external imports.
- `internal/service/` — business logic. Imports `domain` only.
- `internal/repository/` — DB access. Implements service interfaces.
- `internal/handler/` — HTTP (Gin). Imports `service` only.
- `cmd/webui-be/main.go` — wires all layers. Only file allowed to import across layers.

## Conventions

See `.claude/rules/02-conventions.md`.

## Testing

See `.claude/rules/03-testing.md`.

## Security

See `.claude/rules/04-security.md`.

Critical: FileBot exec uses an allowlist — never pass raw user input to exec.Command.

## What Claude Can Touch

Controlled via `.claude/settings.json`.
- Read: anything
- Write/Edit: source files, configs (non-secret)
- Run: go build, go test, go vet, go fmt, go mod, git read commands
- Never: force-push, drop tables, write .env files, exec user input

## MCP Connections

See `.mcp.json`: GitHub, PostgreSQL.
Credentials from env vars only.

## Environment Variables (Required)

```
JWT_SECRET=
PLEX_CLIENT_ID=
PLEX_CLIENT_SECRET=
PLEX_REDIRECT_URL=
DELUGE_HOST=
DELUGE_PORT=
DELUGE_USERNAME=
DELUGE_PASSWORD=
MEDIA_ROOT=
DB_TYPE=sqlite
DB_DATABASE=./data/app.db
```
