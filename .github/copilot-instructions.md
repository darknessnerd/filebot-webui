# filebot-webui — Copilot Instructions

## What This App Does

Web UI: authenticate via Plex, list completed Deluge torrents, run FileBot to rename+move files to media folder, delete torrent+data on success, refresh Plex library.

## Architecture

Layer rule: `handler → service → domain ← repository`

| Component | Purpose | Location |
|-----------|---------|----------|
| `cmd/` | Entry point — wires all layers | `cmd/webui-be/main.go` |
| `internal/domain` | Value types + sentinel errors. Zero external imports. | `internal/domain/` |
| `internal/service` | Business logic. Imports `domain` only. | `internal/service/` |
| `internal/repository` | Data access. Implements service interfaces. | `internal/repository/` |
| `internal/handler` | HTTP/Gin layer. Imports `service` only. | `internal/handler/` |
| `internal/config` | Config loading from env. | `internal/config/` |
| `internal/logger` | Logger interface + zerolog impl. | `internal/logger/` |

**Rule:** ALL implementation packages MUST live under `/internal/`.

Data flow: `HTTP request → handler → service → repository → DB → response`

External calls (Deluge JSON-RPC, Plex API, FileBot exec) happen inside service layer only.

### Key Boundaries

- `domain` has no outbound imports
- `service` imports `domain` only — never `repository` directly, never `database/sql`
- `handler` imports `service` only — never `repository`, never `domain` directly
- `cmd/main.go` is the ONLY file that wires all layers together
- Interfaces belong in the **consuming** package
- Logger injected via constructor — no global `logger.Log`

### External Integrations

| Service | How | Where |
|---------|-----|-------|
| Deluge | JSON-RPC over HTTP | `internal/service/deluge/` |
| Plex | REST API (PIN auth + library refresh) | `internal/service/plex/` |
| FileBot | `exec.Command` with allowlisted args | `internal/service/filebot/` |
| SQLite/Postgres | `database/sql` | `internal/repository/` |

## Conventions

### Naming

- Files: `snake_case`
- Packages: lowercase single word
- Exported types/funcs: `PascalCase`
- Unexported: `camelCase`
- Interfaces: noun phrase (`Repository`, `Notifier`) — never `IRepository` or `RepositoryInterface`
- Constructors: `New<Type>` returning the interface, not the concrete type

### Interfaces

Interfaces belong in the **consuming** package, not the implementing package.

```go
// RIGHT — interface in consuming package (service)
package service
type userStore interface {
    FindByPlexID(ctx context.Context, plexID string) (*domain.User, error)
}
```

Never define interfaces before they are used. Never store `context.Context` in a struct.

### File Layout

```
cmd/webui-be/
  main.go
internal/
  domain/          ← value types only (structs, sentinel errors — no interfaces)
  service/         ← business logic; defines interfaces it needs
    auth/
    plex/
    deluge/
    filebot/
  repository/      ← implements service interfaces implicitly
  handler/         ← HTTP; defines interfaces it needs from service
  config/
  logger/
```

One file per logical concern. No `utils.go`, no `helpers.go`.

### Error Handling

- Wrap: `fmt.Errorf("service.GetTorrents: %w", err)`
- Log only at handler boundary
- Sentinel errors in `domain/errors.go`
- Never `panic` outside `init()`
- Never swallow with `_ = f()`

### Import Order

```go
import (
    // 1. stdlib
    "context"

    // 2. internal
    "github.com/darknessnerd/filebot-webui/internal/domain"

    // 3. external
    "github.com/gin-gonic/gin"
)
```

### Forbidden Patterns

- No global mutable state (no `var Log = ...` at package level)
- No `interface{}` / `any` where a typed interface suffices
- No `init()` with I/O or side effects
- Constructors accept interfaces, not concrete types
- No `// nolint` on security warnings without documented reason

## Testing

Standard `testing` + `testify/assert`. Integration tests use real SQLite (no mocks for DB).

### Coverage Expectations

- All exported functions in `domain/` and `service/`
- All error paths
- All HTTP handler routes: happy path + 400 + 401 + 500
- Auth, token, and FileBot exec paths mandatory

### Test Layout

```
internal/
  service/
    auth/
      auth.go
      auth_test.go      ← white-box, same package
  handler/
    torrent_test.go     ← black-box, package handler_test
test/
  integration/          ← tagged //go:build integration
```

### What Must NOT Be Mocked

- SQLite repository — use real in-memory SQLite
- FileBot exec — use a stub binary or skip with build tag

### Running Tests

```bash
go test ./...
go test -race ./...
go test -tags=integration ./test/integration/...
```

## Security

### Hard Rules

- Never hardcode credentials, tokens, or keys
- Never write to `.env` files
- No plaintext password storage
- No TLS verification disabling
- Parameterized queries only — no SQL string concatenation
- FileBot exec: allowlist commands and args — **never pass raw user input to `exec.Command`**

### FileBot Job Allowlist

No real FileBot binary exec — `internal/service/filebot/internal_engine.go` is a native Go re-implementation
(builds target paths with `fmt.Sprintf`, not FileBot binding syntax). `executor.go` still validates job args
against an allowlist before the engine runs:

| Arg | Allowed values |
|-----|---------------|
| `--db` | `TheMovieDB`, `TheMovieDB::TV`, `AniDB` (`TheTVDB`, `AcoustID`, `OMDb` rejected — unsupported by the native engine) |
| `--action` | `move`, `copy`, `symlink`, `hardlink`, `test` |
| `--conflict` | `skip`, `replace`, `auto`, `index`, `fail` |
| `--output` | Must be under `MEDIA_ROOT` — validate with `filepath.Clean` **and** require `clean == mediaRoot \|\| strings.HasPrefix(clean, mediaRoot+"/")` |
| `--filter`, `--q` | Reject if contains shell metacharacters |
| `source_paths` | Each path validated against shell metacharacters |

`--log` and `--format` are not supported — no underlying FileBot process to pass them to.

### Auth

- JWT signed with HS256, secret from env (`JWT_SECRET`)
- No default fallback for `JWT_SECRET` — fail fast if unset
- Tokens in HttpOnly cookies only — never in URL params or localStorage
- Plex PIN flow: cookies expire in 5 minutes

### Path Traversal

Validate all paths with `filepath.Clean` and confirm they are under the expected root before use.

## What Copilot Can Touch

- **Read:** anything
- **Write/Edit:** source files, configs (non-secret)
- **Run:** `go build`, `go test`, `go vet`, `go fmt`, `go mod`, git read commands
- **Never:** force-push, drop tables, write `.env` files, exec user input

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
