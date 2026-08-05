# Architecture Diagrams

## C4 — Level 2: filebot-webui Containers
Scope: deployable containers and external integrations for runtime architecture.

```mermaid
flowchart LR
  U[User Browser]

  subgraph OWNED["Owned boundary: filebot-webui"]
    API["Web UI + API Container<br/>[Go / Gin / HTTP]<br/>Auth, torrent listing, execution workflow"]
    DB["App Database<br/>[SQLite or Postgres / SQL]<br/>Users, sessions, app state"]
    FS["Mounted Media/Download Paths<br/>[Filesystem]"]
  end

  PLEX["Plex API<br/>[HTTPS REST]"]
  DELUGE["Deluge Web API<br/>[HTTP JSON-RPC]"]
  TMDB["TMDB API<br/>[HTTPS REST]"]

  U -->|"→ HTTPS request/response"| API
  API -->|"→ SQL queries"| DB
  API -->|"→ HTTP JSON-RPC calls"| DELUGE
  API -->|"→ HTTPS PIN auth + library refresh"| PLEX
  API -->|"→ HTTPS metadata lookup"| TMDB
  API -->|"→ Filesystem read/write (move/rename)"| FS
```

**Decisions recorded here:**
- Backend remains one Go container so handler/service/repository layering stays strict without distributed complexity.
- Storage is modeled as a separate container concern to keep SQLite/Postgres swappable by configuration.
- External calls stay synchronous in execution flow so cleanup can run per-torrent immediately after each successful move.

## Sequence Diagram — Rename/Move Execution Flow
Scope: end-to-end request path from user action to cleanup and library refresh.

```mermaid
sequenceDiagram
  autonumber
  actor User as User Browser
  participant API as Web UI + API (Go/Gin)
  participant DB as App DB (SQL)
  participant Deluge as Deluge API (HTTP JSON-RPC)
  participant TMDB as TMDB API (HTTPS REST)
  participant FS as Filesystem
  participant Plex as Plex API (HTTPS REST)

  User->>API: POST /api/filebot/execute (HTTPS)
  API->>DB: Read session/user state (SQL)
  API->>Deluge: Resolve selected torrents to source paths (HTTP JSON-RPC)
  loop For each selected torrent/path pair
    API->>TMDB: Resolve metadata/title mapping (HTTPS REST)
    API->>FS: Rename + move files to MEDIA_ROOT (Filesystem I/O)
    alt This torrent moved successfully and action=move
      API->>Deluge: Delete this torrent + data (HTTP JSON-RPC)
    else This torrent failed
      API->>API: Keep torrent in Deluge
    end
  end
  opt At least one torrent deleted
    API->>Plex: Refresh library section (HTTPS REST)
  end
  API-->>User: 200 OK + per-torrent outcomes + per-file results (HTTPS)
```

**Decisions recorded here:**
- Deluge cleanup is conditional per torrent, so successful moves are cleaned even when another selected torrent fails.
- Result payload includes per-torrent moved/deleted/failed status plus per-file outcomes so partial failures are visible.
