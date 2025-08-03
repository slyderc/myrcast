# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Myrcast is an AI-powered weather report generator for radio broadcast automation. It fetches weather data from OpenWeather API, generates natural-sounding weather reports using Anthropic Claude, and converts them to speech using ElevenLabs TTS. The output WAV files are designed for import into Myriad radio automation systems.

## Development Commands

### Building and Testing
```bash
# Build for current platform
make build

# Run all tests with coverage
make test-coverage

# Run tests with verbose output and race detection
make test

# Format, vet, and lint code
make check

# Run with development config
make run

# Generate configuration file
make generate-config
```

### Cross-Platform Builds
```bash
# Build for all platforms
make build-all

# Build platform-specific
make build-windows
make build-macos  
make build-linux
```

### Development Tools
- Install linting tools: `make install-tools`
- Check dependencies: `make check-deps`
- Clean build artifacts: `make clean`

## Architecture

### Core Components

**Main Application (`main.go`)**
- CLI flag handling and configuration loading
- Orchestrates the complete workflow in `runWeatherReportWorkflow()`
- Comprehensive error handling with specific exit codes
- Cross-platform path validation for Windows/Unix

**API Clients (`api/` package)**
- `weather.go`: OpenWeather API client with rate limiting and caching
- `claude.go`: Anthropic Claude client with retry logic and rate limiting  
- `elevenlabs.go`: ElevenLabs TTS client with audio processing
- `weather_cache.go`: Daily weather data caching system

**Configuration (`config/` package)**
- TOML-based configuration with validation
- Support for API keys, weather location, output paths, and logging
- Cross-platform file path handling

**Utilities (`internal/` package)**
- `logger/`: Enhanced logging with rotation and structured output
- `errorutil/`: Error handling utilities for network, file, and validation errors
- `report/`: Report generation utilities

### Key Features

**Weather Data Caching**
- Caches forecast data daily to reduce API costs (~70-80% savings)
- Always fetches fresh current conditions for accuracy
- Automatic cache expiration at midnight local time

**Rate Limiting & Retries**
- All API clients implement exponential backoff retry logic
- Claude API: 50 requests/minute default limit
- ElevenLabs: Configurable rate limiting
- OpenWeather: Built-in retry with jitter

**Audio Processing**
- Outputs broadcast-ready WAV files (44.1kHz, 16-bit, mono)
- Direct output to configured import directory
- No temporary file cleanup needed

## Configuration

Uses TOML configuration files with these key sections:
- `[apis]`: API keys for OpenWeather, Anthropic, ElevenLabs
- `[weather]`: Latitude/longitude and units (metric/imperial/kelvin)
- `[output]`: Import path and media ID for automation systems
- `[prompt]`: AI instruction template for weather report style
- `[claude]`: Model configuration and retry settings
- `[elevenlabs]`: Voice settings and audio parameters
- `[logging]`: File logging with rotation
- `[cache]`: Weather cache file location

## Testing Strategy

**Unit Tests**
- All API clients have comprehensive test coverage
- Mock external APIs for reliable testing
- Configuration validation testing
- Logging system testing

**Integration Tests**
- Run with `INTEGRATION_TEST=true make test-integration`
- Test real API interactions (requires valid API keys)
- End-to-end workflow validation

**Test File Patterns**
- Unit tests: `*_test.go` files alongside source
- Integration tests use build tag `//go:build integration`

## Error Handling

Uses specific exit codes for different error types:
- `1`: General errors
- `2`: Configuration file errors  
- `3`: Configuration validation errors
- `4`: API call failures
- `5`: File system operation errors
- `6`: Network connectivity errors

## Development Notes

- Go 1.21+ required
- Uses go-resty for HTTP clients
- TOML configuration via pelletier/go-toml/v2
- Anthropic SDK for Claude API integration
- Cross-platform file path handling throughout
- Structured logging with operation tracking
- All external dependencies are production-ready libraries