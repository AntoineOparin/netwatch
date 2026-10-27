GOBIN := $(shell go env GOPATH)/bin
export PATH := $(GOBIN):$(PATH)

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# The agent runs on everyone's computer; the hub needs unprivileged ping,
# which Windows doesn't offer, so it's built for macOS and Linux only.
AGENT_PLATFORMS := darwin/arm64 darwin/amd64 windows/amd64 windows/arm64 linux/amd64 linux/arm64
HUB_PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64

.PHONY: all gen web hub agent test check cross release clean

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
	go build -ldflags "-X main.version=$(VERSION)" -o bin/hub ./cmd/hub

agent:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/agent ./cmd/agent

test:
	go test -race ./...

check: test
	go vet ./...
	cd web && bun run lint && bun run typecheck && bun run test

# Binaries for every platform in dist/. Run `make release` to include the
# dashboard in the hub.
cross:
	@mkdir -p dist
	@for p in $(AGENT_PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		echo "agent $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o dist/netwatch-agent-$$os-$$arch$$ext ./cmd/agent || exit 1; \
	done
	@for p in $(HUB_PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		echo "hub   $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o dist/netwatch-hub-$$os-$$arch ./cmd/hub || exit 1; \
	done

release: web cross

clean:
	rm -rf bin dist web/build internal/webui/build/client
