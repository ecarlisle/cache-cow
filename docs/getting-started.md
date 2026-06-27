# Getting Started

## Overview

Cache Cow is a token-conserving reverse proxy for LLM chat completions. It sits between your agent (OpenCode, Pi Coder, etc.) and your LLM provider, intercepting every `/v1/chat/completions` request to:

- **Cache exact matches** — zero-latency, zero-cost on repeated queries
- **Route smartly** — simple questions go to cheap models, complex tasks to expensive ones
- **Deduplicate system prompts** — saves 500–2000 tokens per request on agent-style conversations
- **Trim context** — sliding window keeps conversations within budget
- **Compress** — normalize whitespace, truncate tool outputs
- **Budget tokens** — clamp `max_tokens` per route so agents never overshoot

## Prerequisites

- **Go 1.22+** (or download a prebuilt binary from releases)
- An **API key** from your LLM provider (OpenAI, Anthropic via proxy, etc.)

## Installation

### Build from source

```bash
git clone https://github.com/ecarlisle/cache-cow.git
cd cache-cow
go build -o bin/proxy ./cmd/proxy/
```

The binary is statically linked — no runtime dependencies, no CGO.

### Download a release

*(placeholder — once CI publishes binaries)*

```bash
curl -L -o proxy https://github.com/ecarlisle/cache-cow/releases/latest/download/proxy-darwin-arm64
chmod +x ./proxy
```

## Configuration

Create a `config.json` anywhere and pass it via `--config`:

```json
{
  "listen_addr": ":8080",
  "upstream_url": "https://api.openai.com/v1",
  "api_key": "sk-proj-...",
  "expensive_model": "gpt-4o",
  "cheap_model": "gpt-4o-mini"
}
```

All config fields are optional — see [Configuration](configuration.md) for every option.

### Environment variables (no config file needed)

The proxy runs entirely from env vars — no config file required:

```bash
export PROXY_API_KEY="sk-proj-..."
export PROXY_EXPENSIVE_MODEL="gpt-4o"
export PROXY_CHEAP_MODEL="gpt-4o-mini"
export PROXY_LISTEN_ADDR=":9090"
./bin/proxy   # no --config flag
```

Every JSON key maps to a `PROXY_` environment variable. Env vars take precedence over the config file when both are present.

If `PROXY_API_KEY` is not set, the proxy falls back to `OPENAI_API_KEY`.

### Recommended defaults for agent use

```json
{
  "expensive_model": "gpt-4o",
  "cheap_model": "gpt-4o-mini",
  "system_prompt_dedup": true,
  "token_budget_cheap": 2048,
  "token_budget_expensive": 8192,
  "max_context_turns": 30,
  "route_tool_calls_to_expensive": true
}
```

## Run

```bash
./bin/proxy --config config.json
```

Expected output:

```
starting proxy on :8080
  expensive: gpt-4o
  cheap:     gpt-4o-mini
  upstream:  https://api.openai.com/v1
  cache:     proxy-cache.db (ttl=3600s)
```

## Point an Agent at the Proxy

### OpenCode

**Option 1 — `opencode.json` in your project:**

Create `opencode.json`:

```json
{
  "provider": {
    "name": "openai",
    "url": "http://localhost:8080/v1",
    "apiKey": "sk-proj-..."
  }
}
```

The proxy forwards to the upstream using its own `api_key` config — OpenCode's `apiKey` only needs to be non-empty to satisfy validation.

**Option 2 — environment variables:**

```bash
export OPENAI_BASE_URL="http://localhost:8080/v1"
```

OpenCode reads `OPENAI_BASE_URL` automatically if no `opencode.json` provider URL is set.

**Option 3 — CLI flag:**

```bash
opencode --provider.openai.url http://localhost:8080/v1
```

### Pi Coder

**Option 1 — `~/.picoder/config.yaml`:**

```yaml
llm:
  provider: openai
  model: gpt-4o
  apiKey: sk-proj-...
  baseUrl: http://localhost:8080/v1
```

**Option 2 — environment variables:**

```bash
export PI_CODER_LLM_PROVIDER="openai"
export PI_CODER_LLM_BASE_URL="http://localhost:8080/v1"
```

### Generic OpenAI-compatible tools

Any tool that accepts a custom `base_url` or `OPENAI_BASE_URL` can use the proxy. Common patterns:

```bash
export OPENAI_BASE_URL="http://localhost:8080/v1"
export OPENAI_API_KEY="sk-proj-..."  # ignored by proxy, required by client SDK
```

## Verify It Works

Send a test request directly to the proxy:

```bash
curl -s http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Say hello in one word"}]
  }' | jq '.choices[0].message.content'
```

Response headers show which pipeline path was taken:

```bash
curl -sI -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}' 2>&1 \
  | grep -i x-cache
```

- `X-Cache: hit` — served from exact-match SQLite cache
- `X-Cache: tool` — served from tool-result cache
- `X-Cache: semantic` — served from semantic (embedding) cache
- `X-Cache: miss` — forwarded upstream

On a miss, `X-Route` tells you the routing decision: `simple` or `complex`.

## Understanding the Pipeline

```
Agent request
    │
    ▼
┌─ Exact Cache ── hit ──► response (0 latency, 0 cost)
│   miss
    ▼
┌─ Tool Cache ─── hit ──► response (deterministic tool pattern)
│   miss
    ▼
┌─ Semantic Cache ── hit ──► response (embedding similarity)
│   miss
    ▼
┌─ Router ─── simple → cheap model
│              complex → expensive model
    ▼
┌─ System Prompt Dedup ── hash + reference substitution
    ▼
┌─ Context Window ── sliding trim + middle summarization
    ▼
┌─ Compression ── whitespace + tool truncation
    ▼
┌─ Token Budget ── clamp max_tokens per route
    ▼
Forward upstream → cache response → return to agent
```

See [Pipeline](pipeline.md) for full details.

## Troubleshooting

| Symptom | Likely cause |
|---------|-------------|
| Proxy starts but `OPENAI_API_KEY` error | Set `PROXY_API_KEY` or `api_key` in config |
| Agent reports connection refused | Proxy not running, or wrong `listen_addr` / port |
| Upstream returns 401 | API key is wrong or expired |
| Cache not working | Check `cache_path` is writable; exact cache requires request hash to match exactly |
| Semantic cache not matching | Lower `semantic_threshold` (default 0.92); ensure embedding sidecar is running |
| Agent streaming breaks | The proxy does not support `stream: true` — all responses are fully buffered |

## Viewing Savings

### Per-Request Headers

Every proxied response includes:

- `X-Cache` — `hit`, `tool`, `semantic`, or `miss`
- `X-Route` — `simple` or `complex` (on miss)
- `X-Original-Model` — what the agent originally requested
- `X-Savings-Bytes` — total bytes removed by the pipeline

### Metrics Endpoint

Point your browser or `curl` at `http://localhost:8080/metrics` for a live JSON snapshot:

```bash
curl http://localhost:8080/metrics | jq
```

### Shutdown Report

Press `Ctrl+C` to stop the proxy. Before exiting, it prints a formatted savings table showing per-stage byte savings, cache hit ratios, and total upstream tokens.

## Next Steps

- [Configuration reference](configuration.md) — all knobs and toggles
- [Pipeline explanation](pipeline.md) — each stage in detail
- [Routing heuristics](routing.md) — how simple vs complex is decided
- [Cache design](cache.md) — exact-match SQLite + semantic embedding
- [Response headers](response-headers.md) — observability at a glance
- [Development guide](development.md) — building, testing, contributing
