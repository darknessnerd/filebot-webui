.PHONY: run test test-race lint build docker-build

run:
	air

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
