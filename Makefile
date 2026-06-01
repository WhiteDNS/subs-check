# Use bash as the default shell.
SHELL=/usr/bin/env bash

# Variables.
BINARY := subs-check
COMMIT := $(shell git rev-parse --short HEAD)
COMMIT_TIMESTAMP := $(shell git log -1 --format=%ct)
VERSION := $(shell git describe --tags --abbrev=0)
GO_BIN := go

# Build flags.
CGO_ENABLED := 0
FLAGS := -trimpath
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.CurrentCommit=$(COMMIT)

# Phony targets.
.PHONY: all build run gotool clean help linux-amd64 linux-arm64 linux-arm linux-386 windows-amd64 windows-arm64 windows-386 darwin-amd64 darwin-arm64 build-all

# Default target: format/tidy code and build for the current environment.
all:  build

# Default build: current environment.
build:
	$(GO_BIN) build -o $(BINARY) $(FLAGS) -ldflags "$(LDFLAGS)"

# Clean.
clean:
	@if [ -f $(BINARY) ]; then rm -f $(BINARY); fi
	@rm -rf build/

# Linux platforms (4).
linux-amd64:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 $(GO_BIN) build -o $(BINARY)_linux_amd64 $(FLAGS) -ldflags "$(LDFLAGS)"

linux-arm64:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=arm64 $(GO_BIN) build -o $(BINARY)_linux_arm64 $(FLAGS) -ldflags "$(LDFLAGS)"

linux-arm:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=arm GOARM=7 $(GO_BIN) build -o $(BINARY)_linux_armv7 $(FLAGS) -ldflags "$(LDFLAGS)"

linux-386:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=386 $(GO_BIN) build -o $(BINARY)_linux_386 $(FLAGS) -ldflags "$(LDFLAGS)"

# Windows platforms (3).
windows-amd64:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=amd64 $(GO_BIN) build -o $(BINARY)_windows_amd64.exe $(FLAGS) -ldflags "$(LDFLAGS)"

windows-arm64:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=arm64 $(GO_BIN) build -o $(BINARY)_windows_arm64.exe $(FLAGS) -ldflags "$(LDFLAGS)"

windows-386:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=386 $(GO_BIN) build -o $(BINARY)_windows_386.exe $(FLAGS) -ldflags "$(LDFLAGS)"

# Darwin platforms (2).
darwin-amd64:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=amd64 $(GO_BIN) build -o $(BINARY)_darwin_amd64 $(FLAGS) -ldflags "$(LDFLAGS)"

darwin-arm64:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=arm64 $(GO_BIN) build -o $(BINARY)_darwin_arm64 $(FLAGS) -ldflags "$(LDFLAGS)"

# Build all selected platforms.
build-all:
	@mkdir -p build
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 $(GO_BIN) build -o build/$(BINARY)_linux_amd64 $(FLAGS) -ldflags "$(LDFLAGS)"; \
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=arm64 $(GO_BIN) build -o build/$(BINARY)_linux_arm64 $(FLAGS) -ldflags "$(LDFLAGS)"; \
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=arm GOARM=7 $(GO_BIN) build -o build/$(BINARY)_linux_armv7 $(FLAGS) -ldflags "$(LDFLAGS)"; \
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=386 $(GO_BIN) build -o build/$(BINARY)_linux_386 $(FLAGS) -ldflags "$(LDFLAGS)"; \
	CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=amd64 $(GO_BIN) build -o build/$(BINARY)_windows_amd64.exe $(FLAGS) -ldflags "$(LDFLAGS)"; \
	CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=arm64 $(GO_BIN) build -o build/$(BINARY)_windows_arm64.exe $(FLAGS) -ldflags "$(LDFLAGS)"; \
	CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=386 $(GO_BIN) build -o build/$(BINARY)_windows_386.exe $(FLAGS) -ldflags "$(LDFLAGS)"; \
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=amd64 $(GO_BIN) build -o build/$(BINARY)_darwin_amd64 $(FLAGS) -ldflags "$(LDFLAGS)"; \
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=arm64 $(GO_BIN) build -o build/$(BINARY)_darwin_arm64 $(FLAGS) -ldflags "$(LDFLAGS)"

# Help.
help:
	@echo "make              - Format/tidy Go code and build the binary for the current environment"
	@echo "make build        - Build the binary for the current environment"
	@echo "make run          - Run Go code directly"
	@echo "make gotool       - Run Go tools: 'mod tidy', 'fmt', and 'vet'"
	@echo "make clean        - Remove binaries and build directory"
	@echo "make linux-amd64  - Build Linux/amd64 binary"
	@echo "make linux-arm64  - Build Linux/arm64 binary"
	@echo "make linux-arm    - Build Linux/armv7 binary"
	@echo "make linux-386    - Build Linux/386 binary"
	@echo "make windows-amd64 - Build Windows/amd64 binary"
	@echo "make windows-arm64 - Build Windows/arm64 binary"
	@echo "make windows-386  - Build Windows/386 binary"
	@echo "make darwin-amd64 - Build macOS/amd64 binary"
	@echo "make darwin-arm64 - Build macOS/arm64 binary"
	@echo "make build-all    - Build all selected platform binaries into build/"
	@echo "make help         - Show this help message"
