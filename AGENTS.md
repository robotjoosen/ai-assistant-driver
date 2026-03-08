# AGENTS.md - AI Assistant Driver

## Overview

This is a Go application that bridges speech-to-text tools (Whisper, Coqui) with ESPHome Voice Assistant. It handles audio processing, VAD (Voice Activity Detection), and communication with ESPHome devices.

---

## Build, Lint & Test Commands

### Building

```bash
# Build the application
go build -o bin/ai-assistant-driver ./cmd/ai-assistant-driver

# Build for different platforms
GOOS=linux GOARCH=arm64 go build -o bin/ai-assistant-driver-linux-arm64 ./cmd/ai-assistant-driver
GOOS=linux GOARCH=amd64 go build -o bin/ai-assistant-driver-linux-amd64 ./cmd/ai-assistant-driver
```

### Testing

```bash
# Run all tests
go test ./...

# Run all tests with verbose output
go test -v ./...

# Run tests for a specific package
go test -v ./internal/...

# Run a single test by name
go test -v -run TestFunctionName ./...

# Run tests with coverage
go test -v -cover ./...

# Run tests matching a pattern
go test -v -run "Test.*Audio" ./...
```

### Linting & Formatting

```bash
# Format code (run before committing)
go fmt ./...

# Run go vet
go vet ./...

# Run golint (if installed)
golangci-lint run

# Run all linters (if golangci-lint is configured)
golangci-lint run ./...

# Check for unused dependencies
go mod tidy
```

### Development

```bash
# Run the application
go run ./cmd/ai-assistant-driver

# Run with configuration file
go run ./cmd/ai-assistant-driver --config config.yaml

# Get dependencies
go mod download
```

---

## Code Style Guidelines

### General Principles

- Write clear, readable code over clever code
- Keep functions small and focused (single responsibility)
- Use meaningful names that reveal intent
- Handle errors explicitly, never ignore them

### Naming Conventions

**Variables & Functions:**
- Use `camelCase` for variable and function names
- Use `PascalCase` for exported (public) functions and types
- Use `snake_case` for file names and package names
- Avoid abbreviations unless widely understood (e.g., `ctx`, `req`, `resp`)
- Be descriptive: `audioBuffer` not `buf`, `voiceDetected` not `vd`

**Types:**
- Use descriptive type names: `AudioProcessor` not `AP`
- Add `er` suffix for interfaces: `Reader`, `Processor`, `Handler`
- Use specific types over generic ones when possible

**Constants:**
- Use `PascalCase` for exported constants
- Use `camelCase` for unexported constants
- Group related constants: `const ( AudioSampleRate = 16000 ... )`

### Imports

**Organization (in order):**
1. Standard library packages
2. Third-party packages
3. Internal/own packages

Within each group, use alphabetical ordering. Use blank lines between groups.

```go
import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/kr/text"
    "github.com/sirupsen/logrus"

    "github.com/robotjoosen/ai-assistant-driver/internal/audio"
    "github.com/robotjoosen/ai-assistant-driver/internal/config"
)
```

### Formatting

- Run `go fmt` before committing
- Use automatic formatting (no manual alignment)
- Maximum line length: 100 characters (soft limit)
- Add trailing commas in composite literals
- Use short declarations (`:=`) when possible, explicit types when clarity demands

### Error Handling

- Always handle errors explicitly: `if err != nil { return ..., err }`
- Wrap errors with context: `return nil, fmt.Errorf("failed to process audio: %w", err)`
- Use sentinel errors for known conditions: `var ErrNotFound = errors.New("not found")`
- Avoid `panic()` except for truly unrecoverable conditions
- Log errors with appropriate level before returning when meaningful

```go
// Good
if err := processAudio(data); err != nil {
    return fmt.Errorf("processing audio frame: %w", err)
}

// Bad - ignoring error
processAudio(data)

// Bad - vague error
if err := processAudio(data); err != nil {
    return err
}
```

### Types & Structs

- Use structs for data containers; prefer composition over inheritance
- Use interfaces to define contracts, not concrete types
- Use pointers (`*Type`) for large structs or when mutation is needed
- Use value types for small, immutable data
- Embed types for composition: `type MyType struct { OtherType }`

### Concurrency

- Use `context.Context` for cancellation and timeouts
- Use channels for communication between goroutines
- Use `sync.WaitGroup` for coordinating multiple goroutines
- Protect shared state with `sync.Mutex` or use mutex-free patterns
- Always close channels from the sender side
- Use `select` with `default` to avoid blocking

### Logging

- Use standard library `log/slog` for structured logging
- Include relevant context in log entries
- Use appropriate log levels: `Debug` for development, `Info` for normal operation, `Warn` for recoverable issues, `Error` for failures
- Avoid logging sensitive data (API keys, passwords, audio content)

### Configuration

- Use a configuration struct that can be loaded from files (YAML, JSON)
- Support environment variable overrides where appropriate
- Validate configuration at startup
- Use sensible defaults with clear documentation

### Testing

- Write tests for all exported functions and important internal logic
- Use table-driven tests for multiple test cases
- Test edge cases and error conditions
- Use descriptive test names: `TestAudioProcessor_RejectsInvalidSampleRate`
- Keep test files in the same package with `_test` suffix
- Use subtests for related test cases

```go
func TestAudioProcessor_RejectsInvalidSampleRate(t *testing.T) {
    tests := []struct {
        name      string
        sampleRate int
        wantErr   bool
    }{
        {"valid_rate_16000", 16000, false},
        {"valid_rate_48000", 48000, false},
        {"invalid_rate_0", 0, true},
        {"invalid_rate_negative", -1, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p, err := NewAudioProcessor(tt.sampleRate)
            if (err != nil) != tt.wantErr {
                t.Errorf("NewAudioProcessor() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Documentation

- Document all exported functions and types with doc comments
- Use complete sentences in documentation
- Include usage examples for complex functions
- Comment non-obvious code decisions

---

## Project Structure

```
.
├── cmd/
│   └── ai-assistant-driver/    # Main application entry point
├── internal/
│   ├── config/                 # Configuration handling
│   ├── esphome/               # ESPHome device client
│   ├── audio/                  # Audio processing logic (future)
│   └── vad/                    # Voice Activity Detection (future)
├── pkg/                       # Reusable packages (if any)
├── testdata/                  # Test fixtures and data
├── .env.example               # Example environment variables
├── go.mod
├── go.sum
└── Makefile                   # Common build tasks (if present)
```

---

## Common Tasks

### Adding a new package

1. Create directory under `internal/` or `pkg/`
2. Add package documentation
3. Create `_test.go` file for tests
4. Run `go mod tidy`

### Running integration tests

```bash
go test -v -tags=integration ./...
```

### Building release binaries

```bash
goreleaser build --clean --snapshot --output dist/
```
