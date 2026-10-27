GOBIN := $(shell go env GOPATH)/bin
export PATH := $(GOBIN):$(PATH)

.PHONY: all gen web hub agent test check cross clean

all: hub agent

# Regenerate Go + TypeScript code from proto/. Needs `bun install` in web/ first.
gen:
	buf lint
	buf generate

# Build the dashboard and copy it where the hub embeds it from.
web:
	cd web && bun run build
	rm -rf internal/webui/build/client
	cp -R web/build/client internal/webui/build/client

hub: web
	go build -o bin/hub ./cmd/hub

agent:
	go build -o bin/agent ./cmd/agent

test:
	go test -race ./...

check: test
	go vet ./...
	cd web && bun run lint && bun run typecheck && bun run test

# The agent has to run on everyone's computer, so make sure it builds for all of them.
cross:
	GOOS=windows GOARCH=amd64 go build -o bin/agent-windows-amd64.exe ./cmd/agent
	GOOS=darwin  GOARCH=arm64 go build -o bin/agent-darwin-arm64 ./cmd/agent
	GOOS=linux   GOARCH=arm64 go build -o bin/agent-linux-arm64 ./cmd/agent
	GOOS=linux   GOARCH=arm64 go build -o bin/hub-linux-arm64 ./cmd/hub

clean:
	rm -rf bin web/build internal/webui/build/client
