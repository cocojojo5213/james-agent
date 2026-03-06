# James Agent

Personal AI assistant built with Go. Self-hosted, self-evolving, Telegram-first.

> Maintained by [@cocojojo5213](https://github.com/cocojojo5213)

## Features

- **Telegram Channel** - Primary interaction channel with streaming responses
- **Multi-Provider** - Anthropic, OpenAI, and 15+ OpenAI-compatible providers (DeepSeek, Groq, xAI, Mistral, SiliconFlow, etc.)
- **Streaming** - Real-time response streaming via Telegram `editMessageText`
- **Memory** - Long-term (MEMORY.md) + daily memory persistence
- **Heartbeat** - Periodic self-evolution tasks from HEARTBEAT.md
- **Skills** - Custom skill loading from workspace (OpenClaw compatible)
- **Cron Jobs** - Scheduled tasks with JSON persistence
- **OpenAI-Compatible API** - Expose as backend via `/v1/chat/completions`
- **Rate Limiting** - Per-sender rate limiting for safety
- **Graceful Shutdown** - Clean process management with errgroup
- **Structured Logging** - JSON/text logging via `log/slog`
- **Additional Channels** - Feishu, WeCom, WhatsApp, Web UI (optional)

## Quick Start

```bash
# Build
make build

# Interactive config setup
make setup

# Set your API key
export JAMES_API_KEY=your-api-key

# Set Telegram bot token
export JAMES_TELEGRAM_TOKEN=your-bot-token

# Start gateway (channels + cron + heartbeat)
make gateway
```

## Configuration

Run `make setup` for interactive config, or edit `~/.james-agent/config.json`:

```json
{
  "provider": {
    "type": "anthropic",
    "apiKey": "your-api-key",
    "baseUrl": ""
  },
  "agent": {
    "model": "claude-sonnet-4-6"
  },
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "your-bot-token",
      "allowFrom": ["123456789"]
    }
  },
  "skills": {
    "enabled": true,
    "dir": ""
  }
}
```

### Provider Types

**Official providers:**

| Type | Config | Env Vars |
|------|--------|----------|
| anthropic (default) | `"type": "anthropic"` | `JAMES_API_KEY`, `ANTHROPIC_API_KEY` |
| openai | `"type": "openai"` | `OPENAI_API_KEY` |
| openai-compatible | `"type": "openai-compatible"` | `JAMES_API_KEY` + `JAMES_OPENAI_BASE_URL` |

**Third-party providers (auto baseUrl, just set one env var):**

| Type | Env Var | Default Model |
|------|---------|---------------|
| `deepseek` | `DEEPSEEK_API_KEY` | `deepseek-chat` |
| `groq` | `GROQ_API_KEY` | `llama-3.3-70b-versatile` |
| `xai` | `XAI_API_KEY` | `grok-5` |
| `together` | `TOGETHER_API_KEY` | `meta-llama/Llama-3.3-70B-Instruct-Turbo` |
| `mistral` | `MISTRAL_API_KEY` | `mistral-large-latest` |
| `moonshot` | `MOONSHOT_API_KEY` | `moonshot-v1-8k` |
| `zhipu` | `ZHIPU_API_KEY` | `glm-4` |
| `yi` | `YI_API_KEY` | `yi-large` |
| `siliconflow` | `SILICONFLOW_API_KEY` | `deepseek-ai/DeepSeek-V3` |
| `openrouter` | `OPENROUTER_API_KEY` | - |
| `volcengine` | `VOLCENGINE_API_KEY` | - |
| `baichuan` | `BAICHUAN_API_KEY` | `Baichuan4` |
| `minimax` | `MINIMAX_API_KEY` | `abab6.5s-chat` |
| `infini` | `INFINI_API_KEY` | - |
| `ollama` | config only | `llama3` |

**Usage - just one env var:**

```bash
# DeepSeek - that's it, no baseUrl needed
export DEEPSEEK_API_KEY=sk-xxx
export JAMES_TELEGRAM_TOKEN=your-bot-token
make gateway
```

**Or in config.json:**

```json
{
  "provider": {
    "type": "deepseek",
    "apiKey": "sk-xxx"
  }
}
```

You can still override `baseUrl` and `model` if needed - the preset only fills in defaults.

### Environment Variables

| Variable | Description |
|----------|-------------|
| `JAMES_API_KEY` | API key (any provider, highest priority) |
| `JAMES_TELEGRAM_TOKEN` | Telegram bot token |
| `JAMES_MODEL` | Override model name |
| `JAMES_BASE_URL` | Custom API base URL |
| `JAMES_OPENAI_BASE_URL` | OpenAI-compatible base URL |
| `JAMES_PROVIDER` | Force provider type |

## Docker Deployment

```bash
# Copy and edit env file
cp .env.example .env

# Start
docker compose up -d

# View logs
docker compose logs -f james-agent
```

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build binary |
| `make build-release` | Build optimized release binary |
| `make run` | Run agent REPL |
| `make gateway` | Start gateway |
| `make setup` | Interactive config setup |
| `make test` | Run tests |
| `make test-race` | Run tests with race detection |
| `make lint` | Run golangci-lint |

## Architecture

```
CLI (cobra) → agent | gateway | onboard | status
                        │
                        ▼
                    Gateway
        ┌───────────┼───────────┐
    Channel      Cron      Heartbeat
    Manager
        │
    Message Bus
        │
    agentsdk-go Runtime
    (ReAct loop + tools)
        │
    Memory + Skills + Journal
```

## Project Structure

```
cmd/james-agent/     CLI entry point
internal/
  bus/               Message bus
  channel/           Channel implementations (Telegram, Feishu, WeCom, WhatsApp, WebUI)
  config/            Configuration loading
  cron/              Cron job scheduling
  gateway/           Gateway orchestration
  heartbeat/         Periodic heartbeat service
  journal/           Conversation journaling
  logging/           Structured logging
  memory/            Memory system
  provider/          AI provider factory
  shared/            Shared runtime, utilities, middleware
  skills/            Custom skill loader
workspace/
  skills/            Custom skills (SKILL.md)
  AGENTS.md          Agent system prompt
  SOUL.md            Agent personality
```

## Skills

Skills are loaded from `SKILL.md` files in the workspace:

```
<workspace>/skills/<skill-name>/SKILL.md
```

Compatible with [OpenClaw](https://github.com/claw-project/OpenClaw) skill format (YAML frontmatter).

## License

MIT
