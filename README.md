# Myrcast - AI Weather Report Generator

Myrcast is an automated weather report generator that creates AI-voiced weather reports for radio broadcast automation systems like Myriad. The application fetches current weather data and generates professional weather scripts using AI, then converts them to audio files ready for broadcast.

## System Requirements

- Windows 10 or later
- Internet connection for weather data and AI services
- Myriad radio automation system (or compatible audio import system)

## Quick Setup

1. **Download** the latest `myrcast.exe` from the releases page
2. **Create** a configuration file named `config.toml` in the same directory
3. **Configure** your API keys and settings (see Configuration section)
4. **Run** `myrcast.exe` to generate your first weather report

## Configuration

Create a `config.toml` file with the following structure:

```toml
[apis]
# Get your OpenWeather API key at: https://openweathermap.org/api
openweather = "your-openweather-api-key-here"

# Get your Anthropic API key at: https://console.anthropic.com/
anthropic = "your-anthropic-api-key-here"

[weather]
# Your location coordinates (find yours at latlong.net)
latitude = 40.7589  # Example: New York City
longitude = -73.9851
units = "imperial"  # "metric", "imperial", or "kelvin"

[output]
# Where temporary files are stored during processing
temp_directory = "C:\\temp\\myrcast"

# Where Myriad should import the generated audio files
import_path = "C:\\Users\\YourName\\Documents\\Myrcast"

[speech]
# Voice settings for audio generation
voice = "alloy"     # Voice ID for text-to-speech
speed = 1.0         # Speech speed (0.1 to 4.0)
format = "mp3_44100_128"  # Audio format

[prompt]
# Your custom weather report template (see Template Variables below)
template = "Good morning! Here's your weather update for {{location}} on {{dow}}, {{date}}. Currently {{current_temp}} with {{current_conditions}}. Today's high will reach {{temp_high}} with a low of {{temp_low}}. Rain chance is {{rain_chance}}. {{wind_conditions}}. Have a great day!"

[claude]
# AI model settings
model = "claude-3-5-sonnet-20241022"
max_tokens = 1000
temperature = 0.7
```

## Template Variables

Use these variables in your `[prompt]` template to automatically insert weather data:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{location}}` | Location name from configuration | "New York City" |
| `{{city}}` | City name from weather data | "New York" |
| `{{country}}` | Country code | "US" |
| `{{current_temp}}` | Current temperature with units | "72°F" |
| `{{temp_high}}` | Today's high temperature | "78°F" |
| `{{temp_low}}` | Today's low temperature | "65°F" |
| `{{current_conditions}}` | Current weather description | "partly cloudy" |
| `{{wind_conditions}}` | Wind speed and direction | "Light SW winds at 8 mph" |
| `{{rain_chance}}` | Precipitation probability | "20%" |
| `{{weather_alerts}}` | Notable weather conditions | "thunderstorms possible" |
| `{{date}}` | Today's date | "January 15" |
| `{{dow}}` | Day of the week | "Monday" |
| `{{time}}` | Current time | "9:30 AM" |
| `{{units}}` | Temperature unit system | "imperial" |

## Example Weather Prompt Templates

### 1. Professional Morning Show Format
```toml
template = "Good morning {{location}}! It's {{time}} on {{dow}}, {{date}}. Your weather forecast shows {{current_conditions}} with {{current_temp}}. We're looking at a high of {{temp_high}} and a low tonight of {{temp_low}}. There's a {{rain_chance}} chance of precipitation today. {{wind_conditions}}. That's your weather update, stay tuned for more music!"
```

### 2. Casual Drive-Time Style
```toml
template = "Hey {{location}}, happy {{dow}}! Right now it's {{current_temp}} and {{current_conditions}} out there. Today we'll hit {{temp_high}}, cooling down to {{temp_low}} tonight. Rain chances are looking at {{rain_chance}}. {{wind_conditions}}. Drive safe out there and have a great day!"
```

### 3. Detailed Weather Report
```toml
template = "This is your comprehensive weather outlook for {{location}} on {{dow}}, {{date}}. Current conditions show {{current_temp}} with {{current_conditions}}. Today's forecast calls for a high temperature of {{temp_high}} and an overnight low of {{temp_low}}. Precipitation probability stands at {{rain_chance}}. Wind conditions: {{wind_conditions}}. Weather alerts: {{weather_alerts}}. For updated forecasts, stay tuned to your weather station."
```

## Running Myrcast

### Command Line Usage
```cmd
# Generate a weather report with default settings
myrcast.exe

# Use a specific configuration file
myrcast.exe --config "C:\path\to\your\config.toml"

# Generate sample configuration file
myrcast.exe --generate-config
```

### Scheduled Automation
Set up Windows Task Scheduler to run Myrcast automatically:

1. Open **Task Scheduler**
2. Create **Basic Task**
3. Set trigger (e.g., "Daily at 6:00 AM")
4. Set action to run `myrcast.exe`
5. Configure Myriad to auto-import from your `import_path`

## Output Files

Myrcast generates audio files in your configured `import_path` with naming format:
- `weather_report_YYYYMMDD_HHMMSS.wav`
- Example: `weather_report_20240115_063000.wav`

Configure Myriad to monitor this directory for automatic import and scheduling.

## API Keys Setup

### OpenWeather API
1. Visit [openweathermap.org/api](https://openweathermap.org/api)
2. Sign up for a free account
3. Generate an API key
4. Add it to your `config.toml` file

### Anthropic Claude API
1. Visit [console.anthropic.com](https://console.anthropic.com/)
2. Create an account and add billing information
3. Generate an API key
4. Add it to your `config.toml` file

## Troubleshooting

**No audio output**: Check your `import_path` directory exists and is writable

**API errors**: Verify your API keys are valid and have sufficient credits

**Location not found**: Double-check your latitude/longitude coordinates

**Template not working**: Ensure variables use `{{variable}}` format with double braces

## Support

For technical support or feature requests, please check the project documentation or contact your system administrator.