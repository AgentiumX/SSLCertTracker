.PHONY: help build-server build-agent test clean

help:
	@echo "Available targets:"
	@echo "  build-server    Build server binary"
	@echo "  build-agent     Build agent binary"
	@echo "  test           Run all tests"
	@echo "  clean          Remove binaries and data"

build-server:
	cd server && go build -o server cmd/server/main.go

build-agent:
	cd agent && go build -o agent cmd/agent/main.go

test:
	cd server && go test ./...
	cd agent && go test ./...

clean:
	rm -f server/server server/server.exe
	rm -f agent/agent agent/agent.exe
	rm -rf data/
