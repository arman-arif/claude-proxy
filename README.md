# Claude Proxy / Anthropic Claude Proxy.

Anthropic Claude API → OpenAI compatible proxy.

Use any Anthropic-compatible client (Claude SDK, Claude Code CLI) with OpenAI compatible API endpoints.

## Quick start

```bash
./serve
```

Or manually:

```bash
export OPENAI_API_KEY="sk-..."
go run .
```

Server starts on `0.0.0.0:8080` (or set `BIND_ADDR`).

## .env file

Create a `.env` file to set defaults:

```
OPENAI_API_KEY=sk-...
OPENAI_API_URL=https://api.openai.com
DEFAULT_MODEL=gpt-4o
PROXY_LOG_ENABLED=false
PROXY_CONSOLE_LOG=true
```

`./serve` loads `.env` automatically. CLI flags override.

## Manage

```bash
./serve                    # foreground (auto-builds to bin/ if missing)
./serve --daemon           # run in background (Go setsid)
./serve --stop             # stop background
./serve --status           # check if running
```

Background process logs to `log/daemon.log` and writes PID to `.daemon.pid`.

## Build

```bash
go build -o bin/claude-proxy .
```

## Config

| Env var | Default | Description |
|---------|---------|-------------|
| `BIND_ADDR` | `0.0.0.0:8080` | Listen address |
| `OPENAI_API_KEY` | _(required)_ | OpenAI API key |
| `OPENAI_API_URL` | `https://api.openai.com` | OpenAI API base URL |
| `DEFAULT_MODEL` | `gpt-4o` | Default model |
| `PROXY_LOG_ENABLED` | `false` | Enable file logging |
| `PROXY_LOG_FILE` | `proxy.log` | Log file path |
| `PROXY_CONSOLE_LOG` | `true` | Log to console |

## CLI flags

| Flag | Default | Description |
|------|---------|-------------|
| `-log-console=true` | `true` | Log requests to console |
| `-log-file=true` | `true` | Log requests to file (`PROXY_LOG_FILE`) |

## Endpoints

### POST /v1/messages

Full Anthropic Claude API support:
- Messages (user/assistant)
- System prompts (string or array)
- Streaming (SSE)
- Tool calling
- Image input (base64)
- Stop sequences
- Temperature, top_p, max_tokens

### POST /v1/messages/count_tokens

Stub endpoint (returns `{input_tokens: 0}`).

## Example usage

```bash
curl http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: anything" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gpt-4o",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### Streaming

```bash
curl -N http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: anything" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gpt-4o",
    "max_tokens": 200,
    "stream": true,
    "messages": [{"role": "user", "content": "Count to 5"}]
  }'
```

### With Anthropic SDK

```python
import anthropic

client = anthropic.Anthropic(
    base_url="http://localhost:8080",
    api_key="anything",
)

message = client.messages.create(
    model="gpt-4o",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello!"}],
)
print(message.content[0].text)
```

### With Claude Code

```bash
export ANTHROPIC_BASE_URL=http://localhost:8080
export ANTHROPIC_API_KEY=anything
claude
```
