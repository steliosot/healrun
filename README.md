# healrun

Self-healing installation agent for shell commands.

## Overview

**healrun** wraps any shell command and automatically repairs failures using an LLM.

When a command fails, healrun:
1. Detects the failure
2. Collects system environment context
3. Requests repair commands from an LLM
4. Applies the fixes safely
5. Retries the original command
6. Stops after 3 repair attempts

## Installation

```bash
# Download or build from source
# Build from source
go build -ldflags="-s -w" -o healrun ./cmd/healrun
sudo cp healrun /usr/local/bin/
chmod +x healrun
```

## Quick Start

```bash
healrun "pip install torch"
```

## Configuration

Create `~/.healrun/config.yaml`:

```yaml
api_keys:
  openai: "sk-..."

model:
  provider: openai  # openai, ollama, dummy
  openai_model: gpt-4o-mini
  ollama_host: http://localhost:11434
  ollama_model: llama3.2

policies:
  allowed:
    - "./scripts/safe-build.sh"
  blocked:
    - "dangerous-tool --force"
```

Environment variables override the config file:

| Variable | Description |
|----------|-------------|
| `HEALRUN_MODEL_PROVIDER` | Model provider (openai, ollama, dummy) |
| `OPENAI_API_KEY` | OpenAI API key |
| `HEALRUN_OPENAI_MODEL` | OpenAI model name |
| `HEALRUN_OLLAMA_HOST` | Ollama server URL |
| `HEALRUN_OLLAMA_MODEL` | Ollama model name |
| `HEALRUN_AUTO_APPROVE` | Auto-approve fixes |
| `HEALRUN_DEBUG` | Enable debug logging |

## Usage

```bash
healrun "<command>"
healrun --auto-approve "npm install"
healrun --debug "pip install torch"
healrun --dry-run "docker build ."
HEALRUN_MODEL_PROVIDER=openai healrun "apt-get install python3-pip"
```

## Features

- **Real-time Output**: Command output streamed to terminal
- **Safety Layer**: Blocks dangerous commands (`rm -rf /`, `shutdown`, etc.)
- **Docker Mode**: Auto-approves fixes, no interactive prompts
- **Robust**: Graceful error handling, proper exit codes

## License

MIT