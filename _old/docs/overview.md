# WebUI Skeleton Overview

WebUI Skeleton is a Go-based web application template designed to accelerate development of secure, modern web apps. It provides built-in authentication, RESTful APIs, web UI templates, and robust configuration management.

## Key Features
- Google OAuth2 authentication
- JWT token management
- SQLite & PostgreSQL support
- RESTful API endpoints
- Web UI templates (HTML)
- Middleware support
- Structured logging (Zerolog)
- Health check endpoints
- Live reloading with Air

## Project Structure
- `cmd/webui-be/`: Main application entry point and web templates
- `internal/`: Application logic, authentication, configuration, database, logging, server, and utilities
- `.env.example`: Example environment configuration
- `go.mod`: Go module file

See other docs for details on each feature and usage.
