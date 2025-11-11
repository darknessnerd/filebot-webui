# Setup Guide

This guide explains how to set up and run the WebUI Skeleton project for local development.

## Prerequisites
- Go 1.20 or newer
- (Optional) Air for live reloading: `go install github.com/air-verse/air@latest`

## Steps
1. **Clone the repository**
   ```bash
   git clone git@github.com:darknessnerd/fullstack-template-go.git my-new-project
   cd my-new-project
   ```
2. **Copy environment file**
   ```bash
   cp .env.example .env
   # Edit .env to configure your environment variables
   ```
3. **Install dependencies**
   ```bash
   go mod tidy
   ```
4. **Run the application**
   - With Air (live reload):
     ```bash
     air
     ```
   - Or with Go:
     ```bash
     go run cmd/webui-be/main.go
     ```

## Access
- Web: http://localhost:8080
- Health: http://localhost:8080/health
- API: http://localhost:8080/api/v1/status

See `overview.md` for project structure and features.
