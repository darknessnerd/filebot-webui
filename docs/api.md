# API Documentation

WebUI Skeleton provides a RESTful API for authentication, user management, and status checks.

## Public Endpoints
- `GET /health` — Health check
- `GET /api/v1/status` — API status
- `GET /` — Home page
- `GET /auth/login` — Login page
- `GET /auth/google` — Google OAuth login
- `GET /auth/google/callback` — OAuth callback
- `POST /auth/logout` — Logout

## Protected Endpoints (require authentication)
- `GET /dashboard` — User dashboard
- `GET /auth/user` — Current user info
- `GET /api/v1/profile` — User profile

## Authentication
Protected endpoints require a valid JWT token. See `authentication.md` for details.

## Example Request
```http
GET /api/v1/profile
Authorization: Bearer <JWT_TOKEN>
```

## Error Handling
- 401 Unauthorized: Invalid or missing token
- 404 Not Found: Invalid endpoint
- 500 Internal Server Error: Unexpected error

See the source code in `internal/handlers/api.go` for implementation details.
