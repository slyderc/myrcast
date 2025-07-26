# Myrcast - Automated Weather Report Generator

Create a Windows CLI application called "Myrcast" written in Go that automates the generation of AI-voiced weather reports for radio broadcast automation.

## Application Overview
Myrcast retrieves weather data, generates a natural-sounding weather report script using Claude, converts it to speech using ElevenLabs, and outputs a WAV file for import into Myriad radio automation software.

## Workflow
1. Load TOML configuration file
2. Fetch weather forecast from OpenWeather API (5-day forecast endpoint)
3. Send weather data + custom prompt to Anthropic Claude API
4. Receive generated weather report script
5. Send script to ElevenLabs for text-to-speech conversion
6. Save WAV file with media_id as filename (e.g., "12345.wav")
7. Copy to Myriad import directory
8. Log all operations and exit with appropriate status code

## Technical Requirements

### Language & Libraries
- **Go 1.21+**
- Use popular, well-maintained libraries:
  - Configuration: `github.com/pelletier/go-toml/v2`
  - HTTP client: `net/http` (standard library) or `github.com/go-resty/resty/v2`
  - Logging: `log/slog` (standard library)
  - CLI args: `flag` (standard library)

### Configuration File (TOML)
The application must accept `--config=filename.toml` parameter and load:

```toml
[apis]
anthropic_api_key = "sk-ant-..."
anthropic_model = "claude-4-0-sonnet"
openweather_api_key = "your-openweather-key"
elevenlabs_api_key = "your-elevenlabs-key"

[weather]
latitude = 47.6062  # Seattle coordinates example
longitude = -122.3321
units = imperial  # metric or imperial

[output]
media_id = 12345
temp_directory = "C:\temp\myrcast"
myriad_import_directory = "C:\Myriad\Import"
log_file = "C:\Program Files\Myrcast\myrcast.log"

[speech]
voice_name = "Rachel"  # ElevenLabs voice
speech_speed = 1.0
audio_format = "wav"  # 44.1kHz/16-bit WAV

[prompt]
weather_prompt = """Generate a concise, friendly morning weather report for radio broadcast. Include today's high/low temperatures, which are {temp_high} and {temp_low}, precipitation chances, which is {rain_chance}, wind conditions, which are {wind_conditions}, and any notable weather alerts: {weather_alerts} Keep it under 45 seconds when spoken. Write in a conversational, professional radio announcer style."""
```

### API Integration Specifications

#### OpenWeather API
- Endpoint: `https://api.openweathermap.org/data/2.5/forecast`
- Documentation: `https://openweathermap.org/forecast5`
- Extract today's weather data from the 5-day forecast
- Include: temperature (high/low), conditions, precipitation probability, wind speed/direction
- Handle metric/imperial units appropriately
- Store API results for today's forecast in variables that can be used in "weather_prompt" configuration file: {temp_low}, {temp_high}, {current_temp}, {current_conditions}, {rain_chance}, {wind_conditions}, {weather_alerts}, {day}, {month}, {year}, {dow}, {time}

#### Anthropic Claude API
- Documentation: `https://github.com/anthropics/anthropic-sdk-go`
- Send weather data as structured context + custom prompt
- Request a weather report script suitable for radio broadcast
- Handle API rate limits and errors gracefully

#### ElevenLabs API
- Documentation `https://elevenlabs.io/docs/api-reference/introduction`
- Convert generated script to speech using specified voice and speed
- Download WAV file (44.1kHz/16-bit)
- Handle voice availability and API limits

### File Operations
- Create temp directory if it doesn't exist
- Write WAV file as `{media_id}.wav` in temp directory
- Copy to Myriad import directory
- Clean up temp files after successful copy

### Error Handling & Logging
- Log all operations to specified log file with timestamps
- Return exit code 0 for success, non-zero for any failure
- Log API response codes, file operations, and error details
- Handle network timeouts, API rate limits, file permission errors

### Windows CLI Executable
- Single binary with no external dependencies
- Accept `--config=path/to/config.toml` command line argument
- Validate configuration on startup
- Provide helpful error messages for missing config values

## Key Implementation Notes
- Ensure proper file path handling for Windows
- Use structured logging with appropriate log levels (INFO, WARN, ERROR)
- Implement proper HTTP timeouts and retries for API calls
- Validate all configuration values before processing
- Include version information and basic help text

## Success Criteria
The application should run reliably as a scheduled task in Myriad, produce consistent audio output, handle common failure scenarios gracefully, and provide clear logging for troubleshooting.

Build this as a production-ready tool suitable for daily use in a radio broadcast environment.
