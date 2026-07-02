.PHONY: run test test-race lint build docker-build

run:
	@set -a; [ -f .env ] && . ./.env; set +a; \
	: "$${JWT_SECRET:?JWT_SECRET required}"; \
	: "$${MEDIA_ROOT:?MEDIA_ROOT required}"; \
	go run ./cmd/webui-be

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	go vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed — skipping"

build:
	CGO_ENABLED=1 go build -ldflags="-s -w" -o bin/webui-be ./cmd/webui-be

docker-build:
	docker build -t filebot-webui:local .
