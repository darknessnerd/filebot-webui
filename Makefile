.PHONY: run test test-race lint build docker-build snapshot-update e2e e2e-update e2e-down

E2E_COMPOSE = UID=$$(id -u) GID=$$(id -g) docker compose -f docker-compose.e2e.yml

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

snapshot-update:
	go test ./cmd/webui-be -run TestSnapshot -update

e2e:
	$(E2E_COMPOSE) up --build --abort-on-container-exit --exit-code-from playwright; \
		status=$$?; $(MAKE) e2e-down; exit $$status

e2e-update:
	$(E2E_COMPOSE) build app
	$(E2E_COMPOSE) run --rm playwright --update-snapshots
	$(MAKE) e2e-down

e2e-down:
	$(E2E_COMPOSE) down --remove-orphans
