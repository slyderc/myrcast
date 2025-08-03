# Myrcast - AI Weather Report Generator

Myrcast automatically generates professional AI-voiced weather reports for radio broadcast automation. Simply run the application and it creates broadcast-ready WAV files with current weather conditions, forecasts, and natural-sounding voice narration.

Perfect for radio stations using Myriad automation or similar broadcast systems.

## System Requirements

- **Windows, macOS, or Linux** (64-bit)
- **Internet connection** for weather data and AI services
- **Radio automation system** (Myriad, etc.) for audio file import

## Quick Start

1. **Download** the latest Myrcast executable for your platform
2. **Generate** a configuration file: `myrcast --generate-config`
3. **Edit** `config.toml` with your API keys and location
4. **Run** `myrcast` to create your first weather report

Your generated WAV files will be saved to the configured directory, ready for broadcast automation.

## Configuration

Myrcast uses a `config.toml` file for all settings. Generate a sample file with:

```bash
myrcast --generate-config
```

### Required API Keys

You'll need three free API keys:

**OpenWeather API** (weather data)
- Visit [openweathermap.org/api](https://openweathermap.org/api)
- Sign up for free account
- Copy your API key to the config file

**Anthropic Claude API** (AI text generation)
- Visit [console.anthropic.com](https://console.anthropic.com/)
- Create account and add billing
- Generate API key and add to config

**ElevenLabs API** (text-to-speech)
- Visit [elevenlabs.io](https://elevenlabs.io/)
- Sign up for free account
- Get API key from your profile

### Essential Settings

Edit your `config.toml` file:

```toml
[apis]
openweather = "your-openweather-api-key-here"
anthropic = "your-anthropic-api-key-here" 
elevenlabs = "your-elevenlabs-api-key-here"

[weather]
# Your broadcast location coordinates
latitude = 40.7589   # Example: New York City
longitude = -73.9851
units = "imperial"   # "metric", "imperial", or "kelvin"

[output]
# Where Myriad should import audio files
import_path = "C:\\Myriad\\Import"  # Windows
# import_path = "/Users/station/Myriad/Import"  # macOS

# Audio filename (without extension)
media_id = "weather_report"
```

### Weather Report Style

Customize your weather report style in the `[prompt]` section. This is an **instruction** to the AI, not a template with variables:

```toml
[prompt]
template = "You are a professional radio weather announcer for morning drive time. Generate a 20-second weather report that's upbeat and informative. Include current conditions, today's high and low temperatures, and any weather to watch for. Use conversational language that sounds natural when spoken aloud."
```

**Example styles:**
- **Morning Show**: "You are a professional radio weather announcer for morning drive time..."
- **Casual**: "You are a friendly local weather reporter with a relaxed, conversational style..."  
- **News Format**: "You are a broadcast meteorologist delivering concise, authoritative weather updates..."

The AI automatically receives current weather data and incorporates it into the report based on your style instructions.

### Voice Settings

Choose your broadcast voice in the `[elevenlabs]` section:

```toml
[elevenlabs]
voice_id = "pNInz6obpgDQGcFmaJgB"  # Professional male voice
# voice_id = "EXAVITQu4vr4xnSDxMaL"  # Professional female voice
speed = 1.0      # Speaking speed (0.7-1.2)
stability = 0.5  # Voice consistency (0.0-1.0)
```

Browse available voices at [elevenlabs.io/voice-library](https://elevenlabs.io/voice-library)

## Running Myrcast

### Basic Usage

```bash
# Generate weather report with default config
myrcast

# Use specific configuration file  
myrcast --config /path/to/station.toml

# Test configuration without generating audio
myrcast --dry-run

# Run with detailed output for troubleshooting
myrcast --verbose
```

### Scheduling Automation

**Windows (Task Scheduler):**
1. Open Task Scheduler
2. Create Basic Task → Daily → Set time (e.g., 6:00 AM, 12:00 PM, 6:00 PM)
3. Start Program → Browse to `myrcast.exe`
4. Configure Myriad to auto-import from your `import_path`

**macOS/Linux (cron):**
```bash
# Edit crontab
crontab -e

# Run every 6 hours at :00 minutes
0 */6 * * * /path/to/myrcast

# Run weekdays at 6 AM, noon, and 6 PM
0 6,12,18 * * 1-5 /path/to/myrcast
```

## Output Files

Myrcast creates broadcast-ready WAV files:

- **Filename**: `weather_report.wav` (configurable via `media_id`)
- **Format**: 44.1 kHz, 16-bit, mono
- **Location**: Your configured `import_path` directory
- **Duration**: Typically 15-30 seconds

Configure your automation system to monitor the `import_path` directory for new files.

## Weather Data Caching

Myrcast intelligently caches weather data to reduce API costs:

- **First run of day**: Fetches complete forecast data
- **Later runs**: Uses cached forecasts + live current conditions  
- **Automatic reset**: Cache expires at midnight local time
- **Savings**: ~70-80% fewer API calls
- **Transparency**: No configuration needed, works automatically

Current conditions (temperature, alerts, precipitation) are always fetched fresh for accuracy.

## Common Issues

**"No audio file created"**
- Check that `import_path` directory exists and is writable
- Verify all three API keys are valid
- Run with `--verbose` to see detailed error messages

**"API key invalid" errors**
- Double-check API keys have no extra spaces or characters
- Ensure Claude and ElevenLabs accounts have available credits
- OpenWeather free tier allows 1000 calls/day

**"Location not found"**
- Verify latitude/longitude coordinates are correct
- Use [latlong.net](https://latlong.net) to find exact coordinates
- Check coordinates use decimal format (e.g., 40.7589, not 40°45'32"N)

**Audio quality issues**
- Try different `voice_id` values from ElevenLabs voice library
- Adjust `speed` (0.7-1.2) and `stability` (0.0-1.0) settings
- Ensure good internet connection for ElevenLabs API

**Scheduling problems**
- Test manual runs first: `myrcast --dry-run` then `myrcast`
- Check file permissions for automation user account
- Verify automation system can access the `import_path` directory

## Advanced Configuration

### Multiple Locations
Create separate config files for different broadcast areas:
```bash
myrcast --config downtown.toml
myrcast --config suburbs.toml  
myrcast --config coastal.toml
```

### Custom Logging
Monitor system health with detailed logs:
```toml
[logging]
enabled = true
directory = "C:\\Myriad\\Logs\\Weather"
level = "info"           # "debug" for troubleshooting
max_files = 30          # Keep 30 days of logs
console_output = false  # Disable for scheduled runs
```

### Cache Location
Customize weather cache storage:
```toml
[cache]
# Default: uses system temp directory
file_path = "C:\\Myriad\\Cache\\weather.toml"
```

## Support

- **Troubleshooting**: Run `myrcast --verbose` for detailed error information
- **Configuration**: Use `myrcast --generate-config` to create fresh config files
- **Testing**: Use `myrcast --dry-run` to validate setup without generating audio

For broadcast integration support, consult your automation system documentation for audio file import configuration.