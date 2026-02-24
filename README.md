# healrun

Healrun automatically detects broken install steps and fixes them with your LLM until all the setup succeeds.

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

Example config (`~/.healrun/config.yaml`):

```yaml
api_keys:
  openai: ""  or use OPENAI_API_KEY env var

model:
  provider: dummy  # dummy, openai, ollama
  openai_model: gpt-4o-mini
  ollama_host: http://localhost:11434
  ollama_model: llama3.2

policies:
  allowed: []   # custom allowed commands/patterns
  blocked: []   # custom blocked commands/patterns
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

## Policies

Customize safety policies to block or allow specific commands:

```yaml
policies:
  allowed:
    - "./scripts/custom-install.sh"
  blocked:
    - "dangerous-tool --force"
```

Default blocked commands:
- `rm -rf /`
- `shutdown`, `reboot`, `poweroff`
- User deletion commands

Default allowed commands:
- `pip install`, `npm install`, `apt-get install`
- Package managers, compilers, common dev tools

## Examples

```bash
# Python package
healrun "pip install torch"

# Node.js
healrun npm install react

# System packages
healrun "apt-get install python3-pip"

# Dockerfile example
FROM python:3.11
COPY healrun /usr/local/bin/healrun
RUN healrun "pip install numpy"

# With flags
healrun --auto-approve "npm install bcrypt"
healrun --debug "pip install numpy"
healrun --dry-run "pip install pytorch"
```

## Features

- **Auto-repair**: Detects failures and applies fixes
- **Safety**: Blocks dangerous commands with custom policies
- **Docker**: Auto-approves in Docker, no prompts
- **Models**: OpenAI, Ollama, or dummy mode

## Build

```bash
go build -ldflags="-s -w" -o healrun ./cmd/healrun
sudo cp healrun /usr/local/bin/
```

## License

MIT