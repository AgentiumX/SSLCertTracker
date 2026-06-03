.PHONY: help install-web build-web build-server build-agent build-all dev-web dev-server test clean

help:
	@echo "Available targets:"
	@echo "  install-web     Install frontend npm dependencies"
	@echo "  build-web       Build frontend -> server/internal/web/dist/"
	@echo "  build-server    Build server (depends on build-web)"
	@echo "  build-agent     Build agent"
	@echo "  build-all       Build everything"
	@echo "  dev-web         Run Vite dev server (proxies /api to :8080)"
	@echo "  dev-server      Run server with default config.yaml"
	@echo "  test            Run all Go tests"
	@echo "  clean           Remove binaries and build outputs"

install-web:
	cd web && npm install

build-web:
	cd web && npm run build

build-server: build-web
	cd server && go build -o server cmd/server/main.go

build-agent:
	cd agent && go build -o agent cmd/agent/main.go

build-all: build-server build-agent

dev-web:
	cd web && npm run dev

dev-server:
	cd server && go run ./cmd/server -config config.yaml

test:
	cd server && go test ./...
	cd agent && go test ./...

clean:
	rm -f server/server server/server.exe
	rm -f agent/agent agent/agent.exe
	rm -rf data/
	rm -rf web/dist server/internal/web/dist/assets
