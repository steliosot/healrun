# healrun

Self-healing installation agent for shell commands.

## Quick Install

```bash
curl -fsSL https://github.com/steliosot/healrun/raw/main/scripts/install.sh | sh
```

## Usage

```bash
healrun "<command>"
```

## Configuration

Copy example config:

```bash
mkdir -p ~/.healrun
cp config.example.yaml ~/.healrun/config.yaml
# Edit ~/.healrun/config.yaml with your API keys
```

Or use environment variables:

| Variable | Description |
|----------|-------------|
| `HEALRUN_MODEL_PROVIDER` | Model (openai, ollama, dummy) |
| `OPENAI_API_KEY` | OpenAI API key |
| `HEALRUN_OPENAI_MODEL` | OpenAI model name |
| `HEALRUN_OLLAMA_HOST` | Ollama server URL |
| `HEALRUN_OLLAMA_MODEL` | Ollama model name |
| `HEALRUN_AUTO_APPROVE` | Auto-approve fixes |
| `HEALRUN_DEBUG` | Enable debug logging |

## Examples

```bash
# Python package
healrun "pip install torch"

# Node.js
healrun npm install react

# System packages
healrun "apt-get install python3-pip"

# Docker
healrun docker build .

# With flags
healrun --auto-approve "npm install bcrypt"
healrun --debug "pip install numpy"
healrun --dry-run "pip install pytorch"
```

## Features

- **Auto-repair**: Detects failures and applies fixes
- **Safety**: Blocks dangerous commands
- **Docker**: Auto-approves in Docker, no prompts
- **Models**: OpenAI, Ollama, or dummy mode

## Build

```bash
go build -ldflags="-s -w" -o healrun ./cmd/healrun
sudo cp healrun /usr/local/bin/
```

## License

MIT