# binary name
BINARY_NAME=mkbrr

# go related variables
GO=go
GOBIN=$(shell $(GO) env GOPATH)/bin

# build variables
BUILD_DIR=build
VERSION=$(shell git describe --tags 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date +%FT%T%z)
LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"

# race detector settings
GORACE=log_path=./race_report.log \
       history_size=2 \
       halt_on_error=1 \
       atexit_sleep_ms=2000

# make all builds and installs
.PHONY: all
all: clean build install

# install ISA-L crypto library if not already installed
.PHONY: install-isal
install-isal:
	@if [ "$$(uname)" = "Linux" ] && [ "$$(uname -m)" = "x86_64" ]; then \
		if [ ! -f "/usr/lib/libisal_crypto.so" ] && [ ! -f "/usr/local/lib/libisal_crypto.so" ]; then \
			echo "Installing ISA-L crypto library..."; \
			chmod +x scripts/install_isal.sh; \
			./scripts/install_isal.sh; \
		else \
			echo "ISA-L crypto library already installed"; \
		fi \
	fi

# build binary
.PHONY: build
build: install-isal
	@echo "Building ${BINARY_NAME}..."
	@mkdir -p ${BUILD_DIR}
	@if [ "$$(uname)" = "Linux" ] && [ "$$(uname -m)" = "x86_64" ] && ( [ -f "/usr/lib/libisal_crypto.so" ] || [ -f "/usr/local/lib/libisal_crypto.so" ] ); then \
		echo "Building with ISA-L support..."; \
		CGO_ENABLED=1 $(GO) build ${LDFLAGS} -tags isal -o ${BUILD_DIR}/${BINARY_NAME}; \
	else \
		echo "Building without ISA-L support..."; \
		CGO_ENABLED=0 $(GO) build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}; \
	fi

# install binary in system path
.PHONY: install
install: build
	@echo "Installing ${BINARY_NAME}..."
	@if [ "$$(id -u)" = "0" ]; then \
		install -m 755 ${BUILD_DIR}/${BINARY_NAME} /usr/local/bin/; \
	else \
		install -m 755 ${BUILD_DIR}/${BINARY_NAME} ${GOBIN}/; \
	fi

# run all tests (excluding large tests)
.PHONY: test
test:
	@echo "Running tests..."
	$(GO) test -v ./...

# run quick tests with race detector (for CI and quick feedback)
.PHONY: test-race-short
test-race-short:
	@echo "Running quick tests with race detector..."
	GORACE="$(GORACE)" $(GO) test -race -short ./internal/torrent -v 
	@if [ -f "./race_report.log" ]; then \
		echo "Race conditions detected! Check race_report.log"; \
		cat "./race_report.log"; \
	fi

# run all tests with race detector (excluding large tests)
.PHONY: test-race
test-race:
	@echo "Running tests with race detector..."
	GORACE="$(GORACE)" $(GO) test -race ./internal/torrent -v
	@if [ -f "./race_report.log" ]; then \
		echo "Race conditions detected! Check race_report.log"; \
		cat "./race_report.log"; \
	fi

# run large tests (resource intensive)
.PHONY: test-large
test-large:
	@echo "Running large tests..."
	GORACE="$(GORACE)" $(GO) test -v -tags=large_tests ./internal/torrent
	@if [ -f "./race_report.log" ]; then \
		echo "Race conditions detected! Check race_report.log"; \
		cat "./race_report.log"; \
	fi

# run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	GORACE="$(GORACE)" $(GO) test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	$(GO) tool cover -html=coverage.txt -o coverage.html
	@if [ -f "./race_report.log" ]; then \
		echo "Race conditions detected! Check race_report.log"; \
		cat "./race_report.log"; \
	fi

# run golangci-lint
.PHONY: lint
lint:
	@echo "Running linter..."
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	golangci-lint run

# clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	@rm -rf ${BUILD_DIR}
	@rm -f coverage.txt coverage.html

# show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all            - Clean, build, and install the binary"
	@echo "  install-isal   - Install ISA-L crypto library (Linux x86_64 only)"
	@echo "  build          - Build the binary"
	@echo "  install        - Install the binary in GOPATH"
	@echo "  test           - Run tests (excluding large tests)"
	@echo "  test-race-short- Run quick tests with race detector"
	@echo "  test-race      - Run all tests with race detector (excluding large tests)"
	@echo "  test-large     - Run large tests (resource intensive)"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  lint           - Run golangci-lint"
	@echo "  clean          - Remove build artifacts"
	@echo "  help           - Show this help"
