# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

### Building
```bash
# Build the binary
make build

# Build with Profile-Guided Optimization (PGO)
make build-pgo

# Install to system path
make install

# Install with PGO optimization
make install-pgo

# Generate PGO profile (required before build-pgo)
make profile
```

### Testing
```bash
# Run all tests (excluding large tests)
make test

# Run tests with race detector (quick)
make test-race-short

# Run all tests with race detector
make test-race

# Run large/resource-intensive tests
make test-large

# Run tests with coverage report
make test-coverage

# Run a single test
go test -v -run TestName ./internal/torrent

# Run tests with specific build tags
go test -v -tags=large_tests ./internal/torrent
```

### Code Quality
```bash
# Run linter (golangci-lint)
make lint

# Clean build artifacts
make clean
```

## Architecture Overview

mkbrr is a high-performance torrent creation and manipulation tool written in Go. The codebase follows a clean architecture with clear separation of concerns.

### Entry Points and Command Structure

1. **Main Entry** (`main.go`) - Minimal entry point that delegates to cmd package
2. **Command Layer** (`cmd/`)
   - `root.go`: Main CLI structure with ASCII banner and global flags
   - `create.go`: Torrent creation with single/batch/preset modes
   - `check.go`: Torrent verification against local files
   - `inspect.go`: Torrent metadata inspection with tree/json output
   - `modify.go`: Metadata modification without original content
   - `update.go`: Self-update functionality
   - `version.go`: Version information display

### Core Business Logic

**Torrent Package** (`internal/torrent/`)
- `types.go`: Core data structures (`CreateTorrentOptions`, `ModifyOptions`, etc.)
- `create.go`: Torrent creation with automatic piece length calculation
- `hasher.go`: High-performance parallel hashing with adaptive worker pools
- `verify.go`: Torrent integrity verification
- `batch.go`: Parallel batch processing for multiple torrents
- `seasonfinder.go`: TV season pack completeness detection
- `modify.go`: Metadata modification without needing source files
- `display.go`: User interface and progress display logic
- `progress.go`: Progress tracking for operations

**Tracker Integration** (`internal/trackers/`)
- `trackers.go`: Centralized tracker-specific constraints
- Enforces piece length limits, torrent size limits, and custom rules
- Automatically selects optimal piece size based on tracker requirements

**Preset System** (`internal/preset/`)
- `preset.go`: Configuration preset management
- YAML-based configuration with JSON schema validation
- Supports default settings with preset-specific overrides

### Key Design Patterns and Conventions

1. **Options Pattern**: All operations use structured options for clean APIs
2. **Worker Pools**: Adaptive parallel processing based on workload characteristics
3. **Tracker Awareness**: Automatic enforcement of tracker-specific requirements
4. **Error Handling**: User-friendly error messages with actionable feedback
5. **Progress Display**: Real-time feedback with multiple display modes

### Important Implementation Details

1. **Parallel Hashing**: 
   - Adaptive worker count based on file size/count
   - Memory-efficient buffer pooling
   - Optimized for both small and large file workloads

2. **Season Pack Detection**:
   - Regex-based pattern matching in `seasonfinder.go`
   - Detects missing episodes in TV season packs
   - Supports multiple naming conventions

3. **Configuration Files**:
   - Presets: `presets.yaml` for reusable settings
   - Batch: `batch.yaml` for multiple torrent operations
   - JSON Schema validation for both formats

4. **File Filtering**:
   - Include/exclude patterns with precedence rules
   - Regex support for complex filtering
   - Default exclusions for common non-media files

### Performance Optimization

- Profile-Guided Optimization (PGO) support via Makefile
- Parallel processing with controlled concurrency
- Memory pooling to reduce allocations
- Optimized piece size selection based on content size

### Testing Strategy

- Unit tests alongside implementation files
- Large tests with `large_tests` build tag for expensive operations
- Race condition detection with custom GORACE settings
- Benchmark tests comparing against competitors

### Common Development Tasks

```bash
# Add a new tracker with specific requirements
# Edit internal/trackers/trackers.go and add to trackerRules map

# Modify season detection patterns
# Edit internal/torrent/seasonfinder.go regex patterns

# Add new command
# Create new file in cmd/ following existing patterns

# Test specific functionality
go test -v -run TestCreateTorrent ./internal/torrent
```

### Directory Structure
```
mkbrr/
├── cmd/                    # CLI commands
├── internal/              # Core business logic
│   ├── torrent/          # Torrent operations
│   ├── trackers/         # Tracker-specific logic
│   └── preset/           # Configuration presets
├── schemas/              # JSON schemas for validation
└── test/                 # Test fixtures and data
```

### Git Commit Guidelines

- Use Conventional Commit format: `type(scope): description`
  - Types: `fix`, `feat`, `chore`, `docs`, `test`, `refactor`, `perf`, `style`
  - Example: `fix(torrent): correct piece length calculation for small files`
  - Example: `feat(tracker): add support for new tracker requirements`
- Keep commits atomic and focused on a single change
- Write clear, descriptive commit messages
- **IMPORTANT**: Never mention Claude or Claude Code in commit messages