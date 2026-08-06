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
5. **Execute** — media engine validates request, renames and moves files to `MEDIA_ROOT`
6. **On move success (per torrent)** — each successfully moved torrent is deleted from Deluge; failed torrents are kept
7. **See results** — per-file success/failure, per-torrent moved/deleted status, raw FileBot output toggle

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

See [doc/architecture.md — Rename/Move Execution Flow](doc/architecture.md#sequence-diagram--renamemove-execution-flow) for the full sequence diagram.

```
Login (Plex PIN)
  → completed torrents list (polls Deluge every 30s)
  → select torrents → FileBot form
  → POST /filebot/execute
      → validate allowlist
      → resolve metadata (TMDB / AniDB)
      → rename + move to MEDIA_ROOT
      → delete each moved torrent from Deluge
      → refresh Plex
  → result page (per-file OK/ERR · per-torrent status)
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
| `DELUGE_HOST` | Deluge daemon hostname or IP |
| `DELUGE_PASSWORD` | Deluge web UI password |

### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_HOST` | `0.0.0.0` | Bind address |
| `SERVER_PORT` | `8080` | HTTP port |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `DEBUG` | `false` | Pretty-print logs |
| `DEV_MODE` | `false` | Bypass auth and mock Deluge/Plex — metadata resolvers (TMDB, AniDB, scheduler) stay real |
| `JWT_EXPIRES_IN` | `24h` | JWT token lifetime |
| `JWT_ISSUER` | `filebot-webui` | JWT issuer claim |
| `DELUGE_PORT` | `8112` | Deluge JSON-RPC port |
| `DELUGE_USERNAME` | _(empty)_ | Deluge username (if required) |
| `TMDB_ACCESS_TOKEN` | _(empty)_ | TMDB v4 bearer token (required only when using TheMovieDB / TheMovieDB::TV) |
| `ANIDB_CLIENT` | _(empty)_ | Registered AniDB HTTP API client id (required for AniDB lookups) |
| `ANIDB_CLIENTVER` | _(empty)_ | Registered AniDB client version (required for AniDB lookups) |
| `ANIDB_PROTOVER` | `1` | AniDB HTTP API protocol version |
| `ANIDB_BASE_URL` | `http://api.anidb.net:9001/httpapi` | AniDB HTTP API endpoint |
| `ANIDB_TITLES_FILE` | _(empty)_ | Optional local AniDB titles XML file for title→AID resolution |
| `ANIDB_TITLES_URL` | _(empty)_ | Optional URL to download AniDB titles XML from |
| `ANIDB_REFRESH_TITLES_ON_START` | `false` | When `true`, refreshes `ANIDB_TITLES_FILE` at startup |
| `ANIDB_SCHEDULER_ENABLED` | `false` | When `true` (and `ANIDB_TITLES_URL` + `ANIDB_TITLES_FILE` set), runs a background scheduler that keeps the titles file up to date |
| `ANIDB_SCHEDULER_INTERVAL` | `12h` | How often the scheduler wakes up to check whether a remote download is needed |
| `ANIDB_MIN_FETCH_INTERVAL` | `24h` | Minimum age of the local file before a remote download is triggered (AniDB policy: ≥ 24h) |
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
| `--db` | dropdown | `TheMovieDB`, `TheMovieDB::TV`, `AniDB` |
| `--action` | dropdown | `move`, `copy`, `symlink`, `hardlink`, `test` |
| `--conflict` | dropdown | `skip`, `replace`, `auto`, `index`, `fail` |
| `--filter` | text input | optional Groovy expression |
| `--q` | text input | override query (`aid:<id>` preferred for AniDB; title query requires `ANIDB_TITLES_FILE`) |
| `-r` | checkbox | recursive mode |

---

## Native Engine Scope

### Output paths

| DB | Output layout |
|----|--------------|
| `TheMovieDB` | `Movies/<Title (Year)>/<Title>.<ext>` |
| `TheMovieDB::TV` | `TV/<Show>/Season N/<Show> - SxxEyy.<ext>` |
| `AniDB` | `Anime/<Title>/Season N/<Title> - SxxEyy.<ext>` |

### Episode / title detection

**TV** — episode marker extracted from filename, in priority order:

| Format | Example |
|--------|---------|
| `SxxEyy` / `SxxEyyEzz` / `SxxEyy-Ezz` | `Show.S01E01.mkv`, `S01E01-E02` |
| `S01.E01` / `S01 E01` | `Show.S01.E01.mkv` |
| `NxYY` | `Show.2x05.mkv` |

**Movie** — year extracted from filename; title normalized by stripping codec noise (`1080p`, `BluRay`, `x265`, `DTS`, …), language tags, release group brackets, and audio channel specs.

**Anime** — episode marker extracted in priority order:

| Format | Example |
|--------|---------|
| `SxxEyy` / `NxYY` | `Show.S02E04.mkv` |
| `EP01` / `E01` / `E01-E13` | `[Group] Show - E01.mkv` |
| `[N/Total]` / `[N-M/Total]` | `Show [03/14].mkv` |
| `- 01` / `- 01-26` (bare after dash) | `[SubsPlease] Show - 01 (1080p).mkv` |
| `#01` / `#01-05` | `Show #01.mkv` |
| `OVA N` / `SP N` / `Special N` | `Hellsing OVA 3.mkv` |
| `Part N` / `Part III` | `Tenchi Muyo Part II.mkv` |

Season inferred from `Season N` / `Stagione N` in path; defaults to 1.

### AniDB lookup

- `--q aid:<id>` — deterministic direct lookup (recommended)
- Title-based lookup when `ANIDB_TITLES_FILE` configured; title cleaned of episode markers before index search
- Startup refresh when `ANIDB_REFRESH_TITLES_ON_START=true`, `ANIDB_TITLES_URL`, and `ANIDB_TITLES_FILE` set
- Background scheduler when `ANIDB_SCHEDULER_ENABLED=true`, `ANIDB_TITLES_URL`, and `ANIDB_TITLES_FILE` set (see below)

### Other constraints

- `--filter` unsupported in native engine

---

## AniDB Titles Scheduler

The scheduler keeps the local AniDB titles file (`ANIDB_TITLES_FILE`) up to date in the background, without manual intervention.

### Behavior

- The scheduler wakes every `ANIDB_SCHEDULER_INTERVAL` (default `12h`).
- On each tick it checks the modification time of `ANIDB_TITLES_FILE`.
- A remote download is performed **only** when the file is older than `ANIDB_MIN_FETCH_INTERVAL` (default `24h`).
- If a download is already running (e.g. a very slow network), the next tick is skipped — no concurrent downloads.
- The scheduler shuts down cleanly when the process receives `SIGINT` or `SIGTERM`.

### AniDB Policy Compliance

[AniDB](https://anidb.net) requires that the anime-titles dump is **not fetched more than once per 24 hours**.

This is enforced by the `ANIDB_MIN_FETCH_INTERVAL` setting (default `24h`).  
**Do not set `ANIDB_MIN_FETCH_INTERVAL` below `24h`** — doing so violates AniDB's terms of service and may result in your IP being banned.

### Quick-Start

```dotenv
ANIDB_TITLES_FILE=./data/anime-titles.xml
ANIDB_TITLES_URL=https://anidb.net/api/anime-titles.xml.gz
ANIDB_SCHEDULER_ENABLED=true
ANIDB_SCHEDULER_INTERVAL=12h
ANIDB_MIN_FETCH_INTERVAL=24h
```

Set `ANIDB_REFRESH_TITLES_ON_START=true` alongside to also populate the file immediately at first boot (before the first scheduler tick fires).

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

See [doc/architecture.md](doc/architecture.md) for:
- C4 container diagram (external integrations, owned boundary)
- Rename/Move execution sequence diagram
- AniDB titles scheduler sequence diagram

### Component Map

```
┌─────────────────────────────────────────────────────────────────┐
│  Browser (HTMX + Alpine.js)                                     │
└────────────────────────┬────────────────────────────────────────┘
                         │ HTTP
┌────────────────────────▼────────────────────────────────────────┐
│  handler/                                                        │
│  ├── AuthHandler       Plex PIN OAuth · JWT cookie              │
│  ├── TorrentHandler    list completed Deluge torrents           │
│  ├── FileBotHandler    form + execute + result                  │
│  └── PlexHandler       library refresh                          │
└────────┬───────────────┬──────────────────┬─────────────────────┘
         │               │                  │
┌────────▼──────┐ ┌──────▼──────┐ ┌────────▼────────────────────┐
│ service/auth  │ │service/deluge│ │ service/filebot              │
│ JWT · Plex    │ │ JSON-RPC    │ │ InternalEngine               │
│ PIN flow      │ └──────┬──────┘ │  ├── MovieResolver (TMDB)    │
└───────┬───────┘        │        │  ├── TVResolver    (TMDB)    │
        │           Deluge RPC    │  └── AnimeResolver (AniDB)   │
┌───────▼───────┐        │        └────────┬────────────────────-┘
│ repository/   │        │                 │
│ UserRepo      │        │       ┌─────────▼──────────────────────┐
│ (SQLite/PG)   │        │       │ service/anidb                  │
└───────────────┘        │       │  Client · title index (RWMutex)│
                         │       │  Scheduler (background goroutine│
                         │       └─────────┬──────────────────────┘
                         │                 │
                    Deluge host       AniDB HTTP API
                                      + titles dump
```

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
| Media engine | Native Go implementation; TMDB and AniDB metadata lookups via HTTP APIs |

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
| TMDB database selected but no token | `TMDB_ACCESS_TOKEN` not set | Set `TMDB_ACCESS_TOKEN` to a TMDB v4 bearer token — required only for `TheMovieDB` / `TheMovieDB::TV` |
| Torrents not showing | Deluge unreachable | Check `DELUGE_HOST`, `DELUGE_PORT`, `DELUGE_PASSWORD` |
| Media rename "file not found" | Volume mount mismatch | Mirror paths between Deluge and filebot-webui containers |
| Media rename "outside MEDIA_ROOT" | Output path rejected | Ensure `--output` is under `MEDIA_ROOT` |
| Login loop | JWT cookie not set | Check `PLEX_REDIRECT_URL` matches actual app URL exactly |
| Plex refresh fails | Server unreachable or wrong token | Check Plex server reachable from container network |
| `connection refused` on port 8112 | `DELUGE_HOST` empty | Set `DELUGE_HOST` to your Deluge machine IP/hostname |
