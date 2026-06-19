# FileBot-WebUI

Self-hosted web UI that connects Deluge + FileBot + Plex into a single workflow: pick completed torrents, rename and move them to your media library, delete the torrent, refresh Plex — all in one click.

---

## What it does

1. **Login** via Plex PIN OAuth (single-user, no local accounts)
2. **View completed torrents** fetched live from Deluge
3. **Select one or more** completed torrents
4. **FileBot form** pre-fills source path from Deluge download location; user picks database, format, action
5. **Execute** — FileBot renames and moves files to `MEDIA_ROOT`
6. **On success** — torrent + data deleted from Deluge; Plex library refresh triggered
7. **Result page** shows per-file success/failure

---

## What it does NOT do

- No directory browser — source path always comes from Deluge
- No output path picker — destination is always `MEDIA_ROOT` (env var)
- No multi-user — single Plex account owns the instance
- No multi-Deluge-server UI — one Deluge configured via env vars
- No Plex library browser or recently-added dashboard — only refresh after move

---

## User Flow

```
[Login with Plex]
       ↓
[Completed torrents list]  ←  polls Deluge
       ↓  (select one or more)
[FileBot form]
  - Source:  /downloads/<torrent name>   (from Deluge)
  - Output:  MEDIA_ROOT                  (fixed)
  - DB, Format, Action, Conflict, etc.
       ↓  (submit)
[FileBot executes rename + move]
       ↓  (all files moved successfully)
[Delete torrent + data from Deluge]
[Refresh Plex library]
       ↓
[Result: success / per-file errors]
```

---

## FileBot Parameters

| Parameter | UI control | Allowed values |
|-----------|-----------|----------------|
| `--db` | dropdown | `TheMovieDB`, `TheTVDB`, `AniDB`, `AcoustID` |
| `--action` | dropdown | `move`, `copy`, `symlink`, `hardlink`, `test` |
| `--conflict` | dropdown | `skip`, `replace`, `auto`, `index`, `fail` |
| `--log` | dropdown | `all`, `fine`, `info`, `warning`, `off` |
| `--format` | text input | free-form; shell metacharacters rejected |
| `--filter` | text input | optional groovy expression |
| `--q` | text input | optional override query |
| `-r` | checkbox | recursive mode |

All values are validated against an allowlist before being passed to `exec.Command`. Raw user input is never interpolated into shell commands.

---

## Quick Start (Docker Compose)

```yaml
services:
  deluge:
    image: lscr.io/linuxserver/deluge
    container_name: deluge
    ports:
      - "8112:8112"
    volumes:
      - /data/downloads:/downloads
      - /data/media:/media

  filebot-webui:
    image: darknessnerd/filebot-webui:latest
    container_name: filebot-webui
    depends_on: [deluge]
    ports:
      - "8888:8080"
    environment:
      - JWT_SECRET=change-me-long-random-string
      - PLEX_CLIENT_ID=FilebotWebui
      - PLEX_CLIENT_SECRET=your-plex-secret
      - PLEX_REDIRECT_URL=http://your-host:8888/auth/plex/forward
      - DELUGE_HOST=deluge
      - DELUGE_PORT=8112
      - DELUGE_PASSWORD=deluge
      - MEDIA_ROOT=/media
      - FILEBOT_LICENSE_PATH=/config/license.psm
    volumes:
      - /path/to/license.psm:/config/license.psm:ro
      - /data/downloads:/downloads
      - /data/media:/media
    restart: unless-stopped
```

**Critical:** Deluge and filebot-webui must mount the **same paths** at the same container paths. If Deluge stores files at `/downloads/Movie.mkv`, filebot-webui must also see `/downloads/Movie.mkv`.

---

## Environment Variables

### Required

| Variable | Description |
|----------|-------------|
| `JWT_SECRET` | JWT signing secret — long random string, never default |
| `PLEX_CLIENT_ID` | Plex app name / client identifier |
| `PLEX_CLIENT_SECRET` | Plex OAuth secret |
| `PLEX_REDIRECT_URL` | Full URL of `/auth/plex/forward` on this app |
| `DELUGE_HOST` | Deluge daemon hostname |
| `DELUGE_PASSWORD` | Deluge web UI password |
| `MEDIA_ROOT` | Absolute path where FileBot moves files to |

### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_HOST` | `0.0.0.0` | Bind address |
| `SERVER_PORT` | `8080` | HTTP port |
| `DEBUG` | `false` | Enable debug logging + Gin debug mode |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `JWT_EXPIRES_IN` | `24h` | JWT token lifetime |
| `JWT_ISSUER` | `filebot-webui` | JWT issuer claim |
| `DELUGE_PORT` | `8112` | Deluge JSON-RPC port |
| `DELUGE_USERNAME` | _(empty)_ | Deluge username (if required) |
| `FILEBOT_PATH` | `filebot` | Path to filebot binary |
| `FILEBOT_LICENSE_PATH` | _(empty)_ | Path to `license.psm` inside container |
| `DB_TYPE` | `sqlite` | `sqlite` or `postgresql` |
| `DB_DATABASE` | `./data/app.db` | SQLite file path or PostgreSQL DB name |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USERNAME` | _(empty)_ | PostgreSQL user |
| `DB_PASSWORD` | _(empty)_ | PostgreSQL password |
| `DB_SSL_MODE` | `disable` | PostgreSQL SSL mode |

---

## FileBot License

At startup, if `FILEBOT_LICENSE_PATH` is set and the file exists, the app registers the license:

```
filebot --license /config/license.psm
```

Without a license, FileBot runs in trial/open-source mode (limited rename operations).

Mount read-only:
```
-v /secure/filebot.psm:/config/license.psm:ro
```

---

## Volume Mount Requirements

filebot-webui reads the torrent's `save_path` from Deluge (e.g. `/downloads`) and passes it directly to FileBot. Both containers must share the same mount points:

```
# Deluge container         filebot-webui container
/downloads          ==     /downloads        (source)
/media              ==     /media            (= MEDIA_ROOT)
```

Mismatch → FileBot "file not found" errors.

---

## Authentication

- Plex PIN flow: user is redirected to Plex Auth App, PIN polled, access token fetched
- On success: user upserted in local DB, JWT issued as HttpOnly cookie
- JWT validates on every protected route via middleware
- Logout clears the cookie

No local password accounts. No multi-user. Single Plex identity owns the instance.

---

## Development

Prerequisites: Go 1.24+, FileBot binary (or stub), Deluge running.

```bash
cp .env.example .env
# edit .env — set JWT_SECRET, PLEX_*, DELUGE_*, MEDIA_ROOT

go build ./cmd/webui-be
./webui-be
```

Run tests:
```bash
go test ./...
go test -race ./...
go test -tags=integration ./test/integration/...
```

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.24 |
| HTTP | Gin |
| Frontend | HTMX + Alpine.js |
| Auth | Plex PIN OAuth + JWT (HS256) |
| Database | SQLite (default) / PostgreSQL |
| Logging | zerolog |
| FileBot | exec via allowlisted arg builder |

---

## Architecture

Clean layered architecture — strict import direction enforced:

```
handler → service → domain ← repository
```

- `internal/domain/` — value types + sentinel errors (no external imports)
- `internal/service/` — business logic; defines its own interfaces
- `internal/repository/` — DB access; implements service interfaces
- `internal/handler/` — HTTP (Gin); imports service only
- `cmd/webui-be/main.go` — wires all layers

See `.claude/rules/01-architecture.md` for full rules.

---

## Security

- FileBot exec: allowlisted args only — no raw user input passed to shell
- Output path validated to be under `MEDIA_ROOT` with `filepath.Clean`
- JWT secret required at startup — no insecure default
- JWT in HttpOnly cookie only — never in URL or localStorage
- Parameterized SQL queries only

---

## Releasing

```bash
git tag v2.0.0
git push origin v2.0.0
```

CI publishes image as `:2.0.0` and `:latest`.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Startup fails: `JWT_SECRET is required` | Env var missing | Set `JWT_SECRET` |
| Startup fails: `MEDIA_ROOT is required` | Env var missing | Set `MEDIA_ROOT` |
| Torrents not showing | Deluge unreachable | Check `DELUGE_HOST`, `DELUGE_PORT`, `DELUGE_PASSWORD` |
| FileBot "file not found" | Volume mount mismatch | Mirror mounts between Deluge and filebot-webui |
| FileBot "outside media root" | `--output` validation | Ensure output is under `MEDIA_ROOT` |
| Login loop | JWT cookie not set | Verify `PLEX_REDIRECT_URL` matches actual app URL |
| Plex refresh fails | Wrong server URL or token | Check Plex server accessibility from container |
