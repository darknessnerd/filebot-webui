# Architecture Diagrams

## C4 — Level 2: filebot-webui Containers
Scope: deployable containers and external integrations for runtime architecture.

```mermaid
flowchart LR
  U[User Browser]

  subgraph OWNED["Owned boundary: filebot-webui"]
    API["Web UI + API Container<br/>[Go / net/http]<br/>Auth, torrent listing, execution workflow"]
    DB["App Database<br/>[SQLite or Postgres / SQL]<br/>Users, sessions, app state"]
    FS["Mounted Media/Download Paths<br/>[Filesystem]"]
  end

  PLEX["Plex API<br/>[HTTPS REST]"]
  DELUGE["Deluge Web API<br/>[HTTP JSON-RPC]"]
  TMDB["TMDB API<br/>[HTTPS REST]"]
  ANIDB["AniDB HTTP API<br/>[HTTP XML]"]

  U -->|"→ HTTPS request/response"| API
  API -->|"→ SQL queries"| DB
  API -->|"→ HTTP JSON-RPC calls"| DELUGE
  API -->|"→ HTTPS PIN auth + library refresh"| PLEX
  API -->|"→ HTTPS metadata lookup"| TMDB
  API -->|"→ HTTP XML lookup (request=anime&aid)"| ANIDB
  API -->|"→ Filesystem read/write (move/rename)"| FS
```

**Decisions recorded here:**
- Backend remains one Go container so handler/service/repository layering stays strict without distributed complexity.
- Storage is modeled as a separate container concern to keep SQLite/Postgres swappable by configuration.
- External calls stay synchronous in execution flow so cleanup can run per-torrent immediately after each successful move.
- AniDB integration uses contract-safe requests (`client/clientver/protover/request`) with local AID cache and 2s pacing guard to reduce flood risk.

## Sequence Diagram — Rename/Move Execution Flow
Scope: end-to-end request path from user action to cleanup and library refresh.

Two HTTP round-trips: GET /filebot (form) resolves torrents from Deluge and embeds source_paths into the HTML form; POST /filebot/execute receives those paths directly — no second Deluge call for path resolution.

```mermaid
sequenceDiagram
  autonumber
  actor User as User Browser
  participant API as Web UI + API (Go/net/http)
  participant Deluge as Deluge API (HTTP JSON-RPC)
  participant TMDB as TMDB API (HTTPS REST)
  participant AniDB as AniDB API (HTTP XML)
  participant FS as Filesystem
  participant Plex as Plex API (HTTPS REST)

  User->>API: GET /filebot?torrent_ids=... (HTTPS — JWT cookie validated by authMiddleware)
  API->>Deluge: ListCompleted — resolve torrent IDs to source paths (HTTP JSON-RPC)
  Deluge-->>API: torrent list
  API-->>User: HTML form with source_paths embedded as hidden fields

  User->>API: POST /filebot/execute — source_paths + job params (HTTPS — JWT cookie)
  Note over API: source_paths come from form POST body, not re-fetched from Deluge
  loop For each torrent/source_path pair
    alt DB = TheMovieDB / TheMovieDB::TV
      API->>TMDB: Resolve metadata/title mapping (HTTPS REST)
    else DB = AniDB
      alt Query is aid:<id>
        API->>AniDB: request=anime&aid=<id> (HTTP XML)
      else Query/title lookup
        API->>API: Resolve title -> AID via local ANIDB_TITLES_FILE index
        API->>AniDB: request=anime&aid=<resolved id> (HTTP XML)
      end
      API->>API: Enforce min 2s AniDB pacing + cache aid lookups
    end
    API->>FS: Rename + move files to MEDIA_ROOT (Filesystem I/O)
    alt This torrent moved successfully and action=move
      API->>Deluge: Delete this torrent + data (HTTP JSON-RPC)
    else This torrent failed
      API->>API: Keep torrent in Deluge
    end
  end
  opt At least one torrent deleted
    API->>Plex: Refresh library section (HTTPS REST) using PlexToken from JWT claims
  end
  API-->>User: 200 OK + per-torrent outcomes + per-file results (HTTPS)
```

**Decisions recorded here:**
- Deluge cleanup is conditional per torrent, so successful moves are cleaned even when another selected torrent fails.
- Result payload includes per-torrent moved/deleted/failed status plus per-file outcomes so partial failures are visible.
- AniDB flow prefers direct `aid:<id>` from `--q`; title-based lookup (via local index) strips episode markers before searching so bare `E01`, `- 01`, `#01`, `OVA N`, `Part N` in filenames do not corrupt the query.
- Episode detection for TV accepts `SxxEyy`, `S01.E01`, `S01 E01`, and `NxYY`; for anime it additionally handles bare-dash, hash, OVA/SP, and Part (arabic + roman) markers.
- Cross-device moves fall back to copy+delete; `copyFile` writes to a `.tmp-copy-*` temp in the target dir then renames atomically — startup cleanup removes any orphans left by a crash. After rename, `copyFile` calls `os.Chmod` to restore the source file's permission bits (default `os.CreateTemp` mask is `0600`, which blocks Plex from reading the file) and `os.Lchown` to preserve uid/gid when the process has `CAP_CHOWN`.
- `--log`, `--format`, `TheTVDB`, `AcoustID`, and `OMDb` removed from the native engine allowlist and form — unsupported databases are rejected at validation, not silently ignored.

## Sequence Diagram — AniDB Titles Scheduler
Scope: background goroutine lifecycle — startup index load, periodic freshness check, atomic disk write, live in-memory reload.

```mermaid
sequenceDiagram
    participant Main      as main.go (startup)
    participant Cleanup   as CleanupOrphanedTemps
    participant Refresh   as RefreshTitlesFile (helper)
    participant Sched     as Scheduler (goroutine)
    participant Client    as anidb.Client
    participant Disk      as Filesystem
    participant Remote    as AniDB titles dump

    Main->>Cleanup: CleanupOrphanedTemps(MEDIA_ROOT)
    Cleanup->>Disk: walk MEDIA_ROOT, remove .tmp-copy-* files
    Note over Cleanup: removes temp files left by interrupted copyFile calls

    alt ANIDB_REFRESH_TITLES_ON_START=true AND ANIDB_TITLES_URL set AND ANIDB_TITLES_FILE set
        Main->>Refresh: RefreshTitlesFile(ctx, url, path)
        Refresh->>Remote: GET ANIDB_TITLES_URL
        Remote-->>Refresh: gzip payload
        Refresh->>Disk: decompress + validate + write .tmp → os.Rename (atomic)
        Refresh-->>Main: (bytes, nil)
        Main->>Client: ReloadIndex(bytes)
        Client->>Client: swap titleIndex under RWMutex
    else ANIDB_TITLES_FILE set (startup refresh not requested or failed)
        Main->>Client: LoadIndexFromFile(path)
        Client->>Disk: read XML
        Disk-->>Client: bytes
        Client->>Client: parse → titleIndex (RWMutex.Lock swap)
    end
    Main->>Sched: go Run(sigCtx)

    loop every ANIDB_SCHEDULER_INTERVAL (default 12h)
        Sched->>Disk: stat ANIDB_TITLES_FILE (mtime)
        alt file age < ANIDB_MIN_FETCH_INTERVAL (24h)
            Sched-->>Sched: skip — AniDB policy: max 1 fetch per 24h
        else stale or missing
            Sched->>Refresh: RefreshTitlesFile(ctx, url, path)
            Refresh->>Remote: GET ANIDB_TITLES_URL
            Remote-->>Refresh: gzip payload
            Refresh->>Refresh: decompress + validate <animetitles marker
            Refresh->>Disk: write .tmp → os.Rename (atomic)
            Refresh-->>Sched: (bytes, nil)
            Sched->>Client: ReloadIndex(bytes)
            Client->>Client: swap titleIndex under RWMutex
        end
    end

    Note over Sched: SIGINT/SIGTERM → sigCtx cancelled → Run returns
```

**Decisions recorded here:**
- Startup temp cleanup runs before any other init so orphaned `.tmp-copy-*` files from crashed copy operations are removed before new moves start — prevents stale files accumulating in `MEDIA_ROOT`.
- File mtime is the freshness signal — no separate state file needed; atomic rename keeps mtime accurate.
- `ReloadIndex` swaps the in-memory index under `RWMutex` so concurrent `SearchAnime` calls are never blocked longer than a pointer swap.
- `TryLock` on the scheduler mutex prevents overlapping downloads when a tick fires while a previous slow download is still in progress.
- `short` and `kana` title types are excluded from the index to prevent abbreviations (`CotS`, `SnM`) causing spurious ambiguity hits.
