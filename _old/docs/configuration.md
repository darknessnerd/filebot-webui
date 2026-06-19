# Configuration

WebUI Skeleton uses environment variables for configuration. You can set these in a `.env` file or via your shell environment.

## Main Variables

### Server
- `SERVER_HOST`: Bind address (default: 0.0.0.0)
- `SERVER_PORT`: Port (default: 8080)

### Database
- `DB_TYPE`: `sqlite` or `postgresql` (default: sqlite)
- `DB_DATABASE`: Database name/path (default: app.db)
- `DB_HOST`, `DB_PORT`, `DB_USERNAME`, `DB_PASSWORD`: PostgreSQL settings
- `DB_SSL_MODE`: PostgreSQL SSL mode (default: disable)

### Authentication
- `JWT_SECRET`: Secret for JWT tokens
- `JWT_EXPIRES_IN`: Token expiration (default: 24h)
- `JWT_ISSUER`: Token issuer (default: webui-skeleton)
- `REQUIRE_AUTH`: Require authentication for all routes (default: false)
- `PLEX_CLIENT_ID`, `PLEX_CLIENT_SECRET`, `PLEX_REDIRECT_URL`: Plex OAuth credentials
- `ENABLE_PLEX_AUTH`: Enable Plex authentication (default: true)
- `SESSION_SECRET`: Session secret key

### Logging
- `DEBUG`: Enable debug mode (default: false)
- `LOG_LEVEL`: Log level (trace/debug/info/warn/error/fatal/panic, default: info)

## Example `.env` File
```
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
DB_TYPE=sqlite
DB_DATABASE=app.db
JWT_SECRET=your-secret
JWT_EXPIRES_IN=24h
REQUIRE_AUTH=false
PLEX_CLIENT_ID=your-plex-client-id
PLEX_CLIENT_SECRET=your-plex-client-secret
PLEX_REDIRECT_URL=http://localhost:8080/auth/plex/callback
ENABLE_PLEX_AUTH=true
SESSION_SECRET=your-session-secret
DEBUG=true
LOG_LEVEL=info
```

See `internal/config/config.go` for implementation details.
