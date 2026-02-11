BINARY_DIR := bin
GO := go
GOFLAGS := -trimpath
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildDate=$(BUILD_DATE)"

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

ALL_TARGETS := \
	$(BINARY_DIR)/promsketch-dropin \
	$(BINARY_DIR)/pskctl \
	$(BINARY_DIR)/psksketch \
	$(BINARY_DIR)/pskinsert \
	$(BINARY_DIR)/pskquery

.PHONY: all clean test proto

all: $(ALL_TARGETS)

$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

$(BINARY_DIR)/promsketch-dropin: $(BINARY_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $@ ./cmd/promsketch-dropin

$(BINARY_DIR)/pskctl: $(BINARY_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $@ ./cmd/pskctl

$(BINARY_DIR)/psksketch: $(BINARY_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $@ ./cmd/psksketch

$(BINARY_DIR)/pskinsert: $(BINARY_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $@ ./cmd/pskinsert

$(BINARY_DIR)/pskquery: $(BINARY_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $@ ./cmd/pskquery

test:
	$(GO) test ./...

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/psksketch/v1/psksketch.proto

clean:
	rm -rf $(BINARY_DIR)
