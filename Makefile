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

# make all builds and installs
.PHONY: all
all: clean build install

# build binary
.PHONY: build
build:
	@echo "Building ${BINARY_NAME}..."
	@mkdir -p ${BUILD_DIR}
	$(GO) build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}

# install binary in system path
.PHONY: install
install:
	@echo "Installing ${BINARY_NAME}..."
	@if [ "$$(id -u)" = "0" ]; then \
		install -m 755 ${BUILD_DIR}/${BINARY_NAME} /usr/local/bin/; \
	else \
		$(GO) install ${LDFLAGS}; \
	fi

# run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GO) test -v ./...

# run tests with race detector
.PHONY: test-race
test-race:
	@echo "Running tests with race detector..."
	$(GO) test -race ./internal/torrent -v

# run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	$(GO) tool cover -html=coverage.txt -o coverage.html

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
	@echo "  build          - Build the binary"
	@echo "  install        - Install the binary in GOPATH"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  lint           - Run golangci-lint"
	@echo "  clean          - Remove build artifacts"
	@echo "  help           - Show this help" 