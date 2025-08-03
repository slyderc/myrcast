# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Myrcast is an AI-powered weather report generator designed for radio broadcast automation. It integrates with the OpenWeather API, Anthropic Claude AI, and ElevenLabs text-to-speech to create professional weather reports for radio stations using systems like Myriad.

**Architecture:** Go CLI application with modular API clients and configuration management.

## Development Commands

### Build Commands
```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Build with debug symbols
make build-debug

# Cross-platform builds
make build-windows    # Windows executable
make build-macos      # macOS (Intel and Apple Silicon)
make build-linux      # Linux (amd64 and arm64)
```

### Testing Commands
```bash
# Run all tests
make test

# Run unit tests only
make test-unit

# Run tests with coverage
make test-coverage

# Generate HTML coverage report
make test-coverage-html

# Run integration tests (requires INTEGRATION_TEST=true)
make test-integration

# Run benchmarks
make test-bench
```

### Code Quality Commands
```bash
# Format code
make fmt

# Run go vet
make vet

# Run linters (requires golangci-lint)
make lint

# Run all checks (format, vet, lint)
make check

# Run staticcheck (requires staticcheck)
make staticcheck
```

### Development Commands
```bash
# Run with development config
make run

# Run with debug logging
make run-debug

# Generate sample configuration
make generate-config

# Install development tools
make install-tools
```

### Single Test Execution
```bash
# Run specific test
go test -v ./config -run TestLoadConfig

# Run specific package tests
go test -v ./api

# Run test with specific tags
go test -tags=integration -v ./...
```

## Core Architecture

### Main Application Flow (`main.go:364-507`)
1. **Initialization**: Parse CLI flags, configure logging, load TOML configuration
2. **API Client Setup**: Initialize weather, Claude, and ElevenLabs clients with rate limiting
3. **Weather Data Retrieval**: Fetch current weather and forecast data from OpenWeather API  
4. **AI Script Generation**: Use Claude to generate natural weather report script from weather data and prompt template
5. **Speech Synthesis**: Convert generated script to audio using ElevenLabs TTS
6. **Output Management**: Save final WAV file to configured import directory for radio automation

### Key Components

**Configuration System (`config/config.go`)**
- TOML-based configuration with comprehensive validation
- Cross-platform path handling and directory creation
- API key management for OpenWeather, Anthropic, and ElevenLabs
- Logging configuration with file rotation
- Default value application and validation rules

**API Clients (`api/` directory)**
- `weather.go`: OpenWeather API client with rate limiting and retry logic
- `claude.go`: Anthropic Claude client for natural language generation  
- `elevenlabs.go`: ElevenLabs client for text-to-speech synthesis
- Each client includes exponential backoff, rate limiting, and error handling

**Internal Utilities (`internal/` directory)**
- `logger/`: Enhanced logging with file rotation and cross-platform support
- `errorutil/`: Error handling utilities for file operations, network, and validation
- `report/`: Report generation utilities

### Configuration Requirements

The application requires three API keys in `config.toml`:
- OpenWeather API key for weather data
- Anthropic API key for AI text generation  
- ElevenLabs API key for text-to-speech

Sample configuration can be generated with:
```bash
./myrcast --generate-config
```

### Key Design Patterns

**Rate Limiting**: All API clients implement custom rate limiters to respect service limits
**Retry Logic**: Exponential backoff with jitter for API failures
**Cross-Platform**: Windows/macOS/Linux support with proper path handling
**Validation**: Comprehensive configuration validation before execution
**Logging**: Structured logging with rotation for production deployment

### Testing Strategy

- Unit tests for individual components (`*_test.go` files)
- Integration tests requiring live API keys (use `INTEGRATION_TEST=true`)
- Coverage reports generated in `coverage/` directory
- Benchmarks for performance-critical operations

### Output Format

Generates broadcast-ready WAV files with:
- 44.1 kHz sample rate
- 16-bit depth  
- Mono channel (optimized for voice)
- Configurable filename via `media_id` setting

The application is designed for automated execution via cron/Task Scheduler for regular weather report generation.