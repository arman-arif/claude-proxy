# anthropics-proxy

Anthropic Claude API → OpenAI compatible proxy.

Use any Anthropic-compatible client (Claude SDK, Claude Code CLI) with OpenAI models.

## Quick start

```bash
export OPENAI_API_KEY="sk-..."
go run .
```

Server starts on `0.0.0.0:8080`.

## Config

| Env var | Default | Description |
|---------|---------|-------------|
| `BIND_ADDR` | `0.0.0.0:8080` | Listen address |
| `OPENAI_API_KEY` | _(required)_ | OpenAI API key |
| `OPENAI_API_URL` | `https://api.openai.com` | OpenAI API base URL |
| `DEFAULT_MODEL` | `gpt-4o` | Default model |

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
    "model": "claude-3-5-sonnet-20241022",
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
    "model": "claude-3-5-sonnet-20241022",
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
    model="claude-3-5-sonnet-20241022",
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

## Build

```bash
go build -o anthropics-proxy .
```
