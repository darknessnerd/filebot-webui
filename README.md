# filebot-webui

[![CI](https://github.com/darknessnerd/filebot-webui/actions/workflows/build.yaml/badge.svg)](https://github.com/darknessnerd/filebot-webui/actions/workflows/build.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/darknessnerd/filebot-webui)](https://goreportcard.com/report/github.com/darknessnerd/filebot-webui)
[![Docker Image](https://img.shields.io/docker/v/darknessnerd/filebot-webui?label=docker&sort=semver)](https://hub.docker.com/r/darknessnerd/filebot-webui)
[![Go Version](https://img.shields.io/badge/go-1.24-blue)](go.mod)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

> Your download finished. FileBot knows what to do. This UI makes sure you never have to open a terminal to tell it.

Self-hosted web UI that connects **Deluge + FileBot + Plex** into a single workflow — pick completed torrents, rename and move them to your media library, delete the torrent, refresh Plex. One click.

---

## What it does

1. **Login** via Plex PIN OAuth (single-user, no local accounts)
2. **View completed torrents** fetched live from Deluge
3. **Select** one or more finished torrents
4. **Configure FileBot** — DB, format, action, conflict resolution
5. **Execute** — FileBot service validates request, current CLI adapter renames and moves files to `MEDIA_ROOT`
6. **On success** — torrent + data deleted from Deluge; Plex library refresh triggered
7. **See results** — per-file success/failure, raw FileBot output toggle

---

## What it does NOT do

- No directory browser — source always comes from Deluge download path
- No output path picker — destination is always `MEDIA_ROOT` (locked down for security)
- No multi-user — single Plex account owns the instance
- No multi-Deluge-server UI — one Deluge via env vars
- No Plex library browser — only triggers refresh after move

If you want a full-featured dashboard, this is not it. This is a surgical tool.

---

## User Flow

```
[Login with Plex]
       ↓
[Completed torrents list]  ←  polls Deluge every 30s
       ↓  (select one or more)
[FileBot form]
  - Source:  /downloads/<torrent name>   (from Deluge)
  - Output:  MEDIA_ROOT                  (fixed, validated)
  - DB, Format, Action, Conflict, Log
       ↓  (submit)
[FileBot renames + moves]
       ↓  (all files moved successfully + action=move)
[Delete torrent + data from Deluge]
[Refresh Plex library]
       ↓
[Result: successes / per-file errors / raw output]
```

---

## Quick Start

### Docker Compose (recommended)

```bash
cp .env.example .env
# edit .env — set JWT_SECRET, PLEX_*, DELUGE_*, MEDIA_ROOT
docker compose up -d
```

Open `http://localhost:8080`.

### Full stack example

```yaml
services:
  filebot-webui:
    image: darknessnerd/filebot-webui:latest
    ports:
      - "8080:8080"
    environment:
      JWT_SECRET: change-me-long-random-string
      PLEX_CLIENT_ID: filebot-webui
      PLEX_REDIRECT_URL: http://your-host:8080/auth/plex/forward
      DELUGE_HOST: 192.168.1.x
      DELUGE_PORT: 8112
      DELUGE_PASSWORD: your-password
      MEDIA_ROOT: /media
      TMDB_ACCESS_TOKEN: your-tmdb-bearer-token
    volumes:
      - /data/downloads:/downloads   # must match Deluge's download path
      - /data/media:/media
    restart: unless-stopped
```

> **No external binaries required** — media renaming is handled natively via the TMDB API. Set `TMDB_ACCESS_TOKEN` to a TMDB v4 bearer token (read-only access token from your TMDB account settings).

> **Critical:** Deluge and filebot-webui must mount the **same paths** at the same container paths. If Deluge stores files at `/downloads/Movie.mkv`, filebot-webui must also see `/downloads/Movie.mkv`. Mismatch = file not found.

---

## Local Development

```bash
# prereqs: Go 1.24+, filebot binary installed on host (or stub), Deluge running somewhere
# note: Docker image bundles FileBot — local dev requires it separately
cp .env.example .env
# edit .env — minimum: JWT_SECRET, MEDIA_ROOT, DELUGE_HOST, DELUGE_PASSWORD

mkdir -p data

# option A: hot reload (recommended)
go install github.com/air-verse/air@latest
air

# option B: plain run
make run

# option C: manual
go run ./cmd/webui-be
```

### Run tests

```bash
make test        # go test ./...
make test-race   # go test -race ./...
make lint        # go vet + staticcheck
```

---

## Environment Variables

### Required

| Variable | Description |
|----------|-------------|
| `JWT_SECRET` | JWT signing secret — long random string, no default, app refuses to start without it |
| `MEDIA_ROOT` | Absolute path FileBot moves files into — all output paths validated against this |
| `PLEX_CLIENT_ID` | Plex app name / client identifier |
| `PLEX_REDIRECT_URL` | Full URL of `/auth/plex/forward` on this app |
| `TMDB_ACCESS_TOKEN` | TMDB v4 read-only bearer token — app refuses to start without it (in non-dev mode) |
| `DELUGE_HOST` | Deluge daemon hostname or IP |
| `DELUGE_PASSWORD` | Deluge web UI password |

### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_HOST` | `0.0.0.0` | Bind address |
| `SERVER_PORT` | `8080` | HTTP port |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `DEBUG` | `false` | Pretty-print logs |
| `JWT_EXPIRES_IN` | `24h` | JWT token lifetime |
| `JWT_ISSUER` | `filebot-webui` | JWT issuer claim |
| `DELUGE_PORT` | `8112` | Deluge JSON-RPC port |
| `DELUGE_USERNAME` | _(empty)_ | Deluge username (if required) |
| `DB_TYPE` | `sqlite` | `sqlite` or `postgres` |
| `DB_DATABASE` | `./data/app.db` | SQLite path or PostgreSQL DB name |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USERNAME` | _(empty)_ | PostgreSQL user |
| `DB_PASSWORD` | _(empty)_ | PostgreSQL password |
| `DB_SSL_MODE` | `disable` | PostgreSQL SSL mode |

---

## Media Engine Parameters

All values validated against an allowlist before execution. Raw user input is never interpolated into commands.

| Parameter | UI control | Allowed values |
|-----------|-----------|----------------|
| `--db` | dropdown | `TheMovieDB`, `TheMovieDB::TV`, `TheTVDB`, `AniDB`, `AcoustID`, `OMDb` |
| `--action` | dropdown | `move`, `copy`, `symlink`, `hardlink`, `test` |
| `--conflict` | dropdown | `skip`, `replace`, `auto`, `index`, `fail` |
| `--log` | dropdown | `all`, `fine`, `info`, `warning`, `off` |
| `--format` | text input | free-form; shell metacharacters rejected |
| `--filter` | text input | optional Groovy expression |
| `--q` | text input | optional override query |
| `-r` | checkbox | recursive mode |

---

## Native Engine Scope

Current internal implementation targets TMDB-backed flows first:

- `TheMovieDB` → movie rename / move into `Movies/<Title (Year)>/<Title>.<ext>`
- `TheMovieDB::TV` → TV rename / move into `TV/<Show>/Season N/<Show> - SxxEyy.<ext>`
- `--q` supported as manual override
- `--format` supports default / `{plex}` only
- `--filter` unsupported in native engine for now

CLI adapter stays wired today. Native engine can replace it later in `cmd/webui-be/main.go`.

---

## Architecture

Clean layered architecture — strict import direction enforced, no shortcuts:

```
handler → service → domain ← repository
```

| Package | Role |
|---------|------|
| `internal/domain/` | Value types + sentinel errors — zero external imports |
| `internal/service/` | Business logic — imports domain only, defines its own interfaces |
| `internal/repository/` | DB access — implements service interfaces implicitly |
| `internal/handler/` | HTTP — imports service only via local interfaces |
| `cmd/webui-be/main.go` | Wires all layers — only file allowed to import across them |

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.24 |
| HTTP | `net/http` stdlib (Go 1.22 ServeMux) |
| Frontend | HTMX + Alpine.js |
| Templates | `html/template` stdlib, embedded via `embed.FS` |
| Auth | Plex PIN OAuth + JWT HS256 (HttpOnly cookie) |
| Database | SQLite (default) / PostgreSQL |
| Logging | zerolog behind injected interface |
| FileBot | Service validates request; temporary CLI adapter uses `exec.CommandContext`; bundled in Docker image (v5.1.7 portable + OpenJDK 11) |

---

## Security

- FileBot service validates args before execution; CLI adapter never receives raw shell input
- `source_paths` validated against shell metacharacters before exec — Deluge paths containing `$`, `` ` ``, `;`, `|` etc. are rejected
- `--output` validated to be under `MEDIA_ROOT` via `filepath.Clean` with separator guard (prevents `/media2` bypassing a `/media` prefix check)
- `JWT_SECRET` required at startup — no insecure fallback
- JWT lives in HttpOnly cookie only — never in URL params or localStorage
- Parameterized SQL queries throughout
- `Secure` cookie flag auto-enabled behind HTTPS / `X-Forwarded-Proto` proxy

---

## Releasing

```bash
git tag v1.0.0
git push origin v1.0.0
```

CI builds and pushes `:1.0.0`, `:1.0`, and `:latest` to Docker Hub.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `JWT_SECRET is required` at startup | Env var missing | Set `JWT_SECRET` |
| `MEDIA_ROOT is required` at startup | Env var missing | Set `MEDIA_ROOT` |
| `TMDB_ACCESS_TOKEN is required` at startup | Env var missing | Set `TMDB_ACCESS_TOKEN` to a TMDB v4 bearer token |
| Torrents not showing | Deluge unreachable | Check `DELUGE_HOST`, `DELUGE_PORT`, `DELUGE_PASSWORD` |
| Media rename "file not found" | Volume mount mismatch | Mirror paths between Deluge and filebot-webui containers |
| Media rename "outside MEDIA_ROOT" | Output path rejected | Ensure `--output` is under `MEDIA_ROOT` |
| Login loop | JWT cookie not set | Check `PLEX_REDIRECT_URL` matches actual app URL exactly |
| Plex refresh fails | Server unreachable or wrong token | Check Plex server reachable from container network |
| `connection refused` on port 8112 | `DELUGE_HOST` empty | Set `DELUGE_HOST` to your Deluge machine IP/hostname |
