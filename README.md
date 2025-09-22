---

# 📂 FileBot-WebUI 🖥️

🎉 Welcome to **FileBot-WebUI** – a web interface around the FileBot CLI for organizing and renaming media libraries. Built with Go + a lightweight HTML/CSS/Alpine.js frontend, packaged in a single Docker image that also bundles FileBot itself.

---

## 🚀 What is this?
A self‑hosted web UI to:
- Browse mounted media & download directories
- Trigger FileBot operations with predefined or custom formats
- Integrate Plex authentication (optional)
- (Planned) Interact with Deluge torrents directly

---

## 🎬 Core Features
- ✅ Single container: Go backend + static UI + FileBot runtime
- ✅ Configurable via environment variables (12‑factor style)
- ✅ Plex OAuth authentication (optional / enforceable)
- ✅ Directory presets for safe path selection (no arbitrary host traversal)
- ✅ FileBot license bootstrap on startup (if provided)
- ✅ Structured logging (zerolog)
- ✅ SQLite (default) or PostgreSQL ready

---

## 📦 Quick Start (Docker)
Minimal example (adjust host paths & secrets):
```bash
docker run -d --name filebot-webui \
  -p 8888:8080 \
  -e JWT_SECRET="change-me" \
  -e DIRECTORY_PRESETS="Downloads:/downloads,Movies:/movies" \
  -e FILEBOT_LICENSE_PATH="/config/license.psm" \
  -v /path/to/license.psm:/config/license.psm:ro \
  -v /srv/downloads:/downloads \
  -v /srv/movies:/movies \
  darknessnerd/filebot-webui:latest
```
Open: http://localhost:8888

If you don't mount a license, FileBot will run in trial mode (limitations apply).

---

## 🗂️ docker-compose Example (incl. Deluge Alignment)
```yaml
services:
  deluge:
    image: lscr.io/linuxserver/deluge
    container_name: deluge
    ports:
      - "8112:8112"
    volumes:
      - /data/deluge/downloads:/downloads
      - /data/media/movies:/movies
      - /data/media/tv_shows:/tv_shows
      - /data/media/anime:/anime
    # ... other deluge config ...

  filebot-webui:
    image: darknessnerd/filebot-webui:latest
    container_name: filebot-webui
    depends_on: [deluge]
    ports:
      - "8888:8080"
    environment:
      - JWT_SECRET=change-me
      - JWT_EXPIRES_IN=24h
      - JWT_ISSUER=filebot-webui
      - ENABLE_PLEX_AUTH=true
      - AUTH_PROVIDER=plex
      - PLEX_CLIENT_ID=FilebotWebui
      - PLEX_CLIENT_SECRET=your-plex-secret
      - PLEX_REDIRECT_URL=http://your-host:8888/auth/plex/forward
      - SESSION_SECRET=change-me-session
      - DIRECTORY_PRESETS=Downloads:/downloads,Movies:/movies,TV Shows:/tv_shows,Anime:/anime
      - FILEBOT_LICENSE_PATH=/config/license.psm
      # Optional FileBot fine-tuning examples:
      # - FILEBOT_MOVIE_FORMAT={n} ({y})/{n} ({y}) - {vf}
      # - FILEBOT_SERIES_FORMAT=TV/{n}/Season {s.pad(2)}/{s00e00} - {t}
      # - FILEBOT_DATABASE=TheMovieDB
    volumes:
      - /path/to/license.psm:/config/license.psm:ro
      - /data/deluge/downloads:/downloads
      - /data/media/movies:/movies
      - /data/media/tv_shows:/tv_shows
      - /data/media/anime:/anime
    restart: unless-stopped
```

---

## 🔐 Authentication / Authorization
- Plex OAuth is enabled by setting ENABLE_PLEX_AUTH=true and AUTH_PROVIDER=plex
- JWT signing requires JWT_SECRET (required, non-empty)
- SESSION_SECRET secures server-side sessions
- If Plex is enforced and credentials (client id/secret) are missing, startup fails

---

## ⚙️ Configuration Reference
Only relevant variables are listed (defaults shown where meaningful):

Application:
- SERVER_HOST (default 0.0.0.0)
- SERVER_PORT (default 8080)
- DEBUG (default true)
- LOG_LEVEL (debug|info|warn|error; default debug)

Database:
- DB_TYPE (sqlite|postgresql; default sqlite)
- DB_DATABASE (sqlite path or DB name; default app.db)
- DB_HOST / DB_PORT / DB_USERNAME / DB_PASSWORD / DB_SSL_MODE (PostgreSQL)
- DB_MAX_OPEN_CONNS / DB_MAX_IDLE_CONNS / DB_CONN_MAX_LIFETIME

Authentication:
- JWT_SECRET (required)
- JWT_EXPIRES_IN (e.g. 24h)
- JWT_ISSUER
- ENABLE_PLEX_AUTH (true/false)
- AUTH_PROVIDER (plex|both) NOTE: code default = both; set plex to enforce Plex only
- PLEX_CLIENT_ID / PLEX_CLIENT_SECRET / PLEX_REDIRECT_URL
- SESSION_SECRET

Directory & FileBot:
- DIRECTORY_PRESETS (required) Comma list: Label:/container/path
- FILEBOT_LICENSE_PATH (path inside container to license .psm)
- FILEBOT_PATH (default /usr/bin/filebot)
- FILEBOT_ARGUMENTS (default --help) – overridden by UI when running jobs
- FILEBOT_LOG_LEVEL (info|debug)
- FILEBOT_OUTPUT_DIRECTORY (default /output if used explicitly)
- FILEBOT_MOVIE_FORMAT / FILEBOT_SERIES_FORMAT / FILEBOT_MUSIC_FORMAT
- FILEBOT_CONFLICT_ACTION (ask|override|skip)
- FILEBOT_DATABASE (TheMovieDB|AniDB|TheTVDB etc.)
- FILEBOT_LANGUAGE (default en)
- FILEBOT_RECURSIVE (true/false)
- FILEBOT_ARTWORK (true/false)
- FILEBOT_SUBTITLES (true/false)

---

## 📄 FileBot License Handling
At container start:
1. If FILEBOT_LICENSE_PATH points to an existing file, it is copied to /opt/filebot/.license/license.psm
2. The script runs: filebot --license /opt/filebot/.license/license.psm
3. If absent, continues without a license (expect feature limits)

Recommendation: mount as read-only.

Example:
```
-e FILEBOT_LICENSE_PATH=/config/license.psm \
-v /secure/licenses/filebot/license.psm:/config/license.psm:ro
```

---

## 🧭 Directory Presets
DIRECTORY_PRESETS restricts user selection to safe base paths. Format:
```
DIRECTORY_PRESETS=Label1:/path1,Label2:/path2,Movies:/movies,Shows:/tv
```
Invalid / empty value → startup error.

---

## 🔄 Torrent Path Mapping (Deluge / Other Clients)
Your torrent client and FileBot-WebUI MUST share identical mount points, otherwise FileBot cannot locate downloaded files.

Rules:
1. Mirror mounts across containers: /downloads, /movies, /tv_shows, etc.
2. Eliminate typos (downloads vs donwloads).
3. Use only container paths in API / UI operations.
4. Keep DIRECTORY_PRESETS aligned with mounted paths.

Validation:
```
# In Deluge shows: /downloads/Example.Release.2160p.mkv
docker exec -it filebot-webui ls /downloads/Example.Release.2160p.mkv
```
If remote Deluge (different host) → implement external path translation before submitting to FileBot-WebUI.

---

## 🛠️ Development (Local)
Prerequisites: Go 1.22+, FileBot license (optional), Node tooling NOT required (static assets pre-bundled).

Build & run:
```bash
go build -o bin/webui ./cmd/webui-be
JWT_SECRET=dev-secret DIRECTORY_PRESETS="Downloads:./" ./bin/webui
```
SQLite file (app.db) created in working dir unless overridden by DB_DATABASE.

Run inside project (using Docker build):
```bash
docker build -t filebot-webui:dev .
docker run --rm -p 8080:8080 \
  -e JWT_SECRET=dev \
  -e DIRECTORY_PRESETS="Root:/app" \
  filebot-webui:dev
```

---

## 🧪 Testing
(Placeholder) Add Go tests under internal/*/ with `_test.go` naming. Run:
```bash
go test ./...
```

---

## 📋 Logging
Controlled via DEBUG + LOG_LEVEL. Structured JSON (zerolog). Adjust container log driver for persistence if needed.

---

## 🔒 Security Notes
- Always change JWT_SECRET & SESSION_SECRET in production
- Restrict license file permissions (600 on host)
- Run behind HTTPS terminator (reverse proxy like Caddy / Traefik / Nginx)
- Principle of least privilege on mounted directories

---

## 🧩 FileBot Format Examples
Movie:
```
FILEBOT_MOVIE_FORMAT="Movies/{n} ({y})/{n} ({y}) - {vf}"
```
Series:
```
FILEBOT_SERIES_FORMAT="TV/{n}/Season {s.pad(2)}/{s00e00} - {t}"
```

These can also be overridden per job in the UI (future enhancement if not yet exposed).

---

## 🎨 Tech Stack
- Go backend (net/http + templates)
- Alpine.js for lightweight interactivity
- FileBot portable distribution integrated
- SQLite / PostgreSQL via database/sql
- Zerolog for logging

---

## 🤝 Contributing
1. Fork & branch (feature/xyz)
2. Write clean commits
3. Include tests where possible
4. Open PR describing rationale & configuration changes

---

## 🏷️ Releasing (Tags)
Tagging a commit (semantic versioning):
```bash
git tag v1.0.0
git push origin v1.0.0
```
CI / build automation (if configured) should publish image as :1.0.0 and :latest.

---

## ❓ Troubleshooting
- Startup fails: verify DIRECTORY_PRESETS not empty
- 401 responses: ensure JWT cookie/session present after Plex login
- License ignored: check FILEBOT_LICENSE_PATH mounted & readable
- Paths missing: confirm volume mounts + matching presets
- Database errors: for PostgreSQL ensure network + credentials + sslmode

---

## 🗺️ Roadmap (Potential)
- Direct Deluge RPC integration
- Job history & audit trail
- Custom per-user presets
- WebSocket progress streaming
- UI-driven FileBot format editor

---

Happy organizing! 💾
