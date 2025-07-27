# Claude Code Instructions

## Project Overview
**Myrcast** - Automated Weather Report Generator for Radio Broadcast
- **Purpose**: Windows CLI application that generates AI-voiced weather reports for Myriad radio automation
- **Language**: Go 1.21+ with standard library focus
- **Status**: 40% complete (4/10 major tasks done, 19/47 subtasks complete)

## Task Master AI Instructions
**Import Task Master's development workflow commands and guidelines, treat as if import is in the main CLAUDE.md file.**
@./.taskmaster/CLAUDE.md

## Current Implementation Status

### ✅ Completed Components (Tasks 1-4)
- **Go Project Setup**: Module initialized with required dependencies
- **TOML Configuration**: Complete config loader with validation (`config/config.go`)
- **Structured Logging**: `log/slog` implementation with file/console output (`internal/logger/logger.go`)
- **OpenWeather API**: Full 5-day forecast integration with unit conversion (`api/weather.go`)

### 🚧 In Progress / Next Priority
- **Task 5**: Anthropic Claude API Integration (pending)
  - Template variable substitution system needed
  - Weather data formatting for Claude context
  - API request handling with retries

### 📋 Dependencies & Architecture
- **Current Dependencies**: 
  - `github.com/pelletier/go-toml/v2` (configuration)
  - `github.com/go-resty/resty/v2` (HTTP client)
  - `github.com/anthropics/anthropic-sdk-go` (Claude API)
  - `log/slog` (structured logging)
- **Project Structure**:
  - `main.go` - CLI entry point
  - `config/` - Configuration management
  - `api/` - External API integrations (weather, claude, elevenlabs)
  - `internal/` - Logger and report utilities

## Configuration Management
- When adding or changing any of the configuration file entries, always create a NEW example-full-config.toml file that reflects the current state, with ALL settings and default values.
- Current config template at `example-config.toml` covers: APIs, weather location, output paths, speech settings, prompt template

## Development Guidelines
- Remember - we need to maintain Windows and MacOS compatibility for all file path references.
- Always use publicly available and strongly supported modules and libraries before writing from scratch.
- **Production Target**: Single Windows executable for Myriad radio automation scheduling
- **Output Format**: 44.1kHz/16-bit WAV files with media_id naming convention