# Development Guide

This guide covers development practices for working with WebUI Skeleton.

## Adding New Routes
1. Create route handlers in `internal/server/handlers.go`.
2. Register routes in `internal/server/server.go` (see `setupRoutes` function).
3. Add HTML templates in `cmd/webui-be/web/templates/` if needed.

## Adding Database Tables
1. Add migration SQL in `internal/database/database.go` (`Migrate` method).
2. Create repository interfaces and implementations in `internal/database/`.
3. Update application setup in `internal/app/app.go`.

## Authentication Middleware
- Use `AuthMiddleware()` for protected routes.
- Use `OptionalAuthMiddleware()` for routes that optionally accept authentication.
- Access user info in handlers with `auth.GetUserFromContext(c)`.

## Logging
- Uses Zerolog for structured logging.
- Configure log level and debug mode in `.env`.

## Live Reloading
- Use Air for automatic reloads during development.
- Configuration is in `.air.toml`.

## Testing
- Add unit tests in the `internal/` packages.
- Use Go's built-in testing tools: `go test ./...`

## Building for Production
1. Set production environment variables.
2. Build with: `go build -o webui-skeleton cmd/webui-be/main.go`
3. Run the binary or use Docker.

See other docs for API, authentication, and configuration details.
