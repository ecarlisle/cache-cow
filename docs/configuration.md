# Configuration

Configuration is loaded from a JSON file (optional, `--config` flag) and overlaid with environment variables. Env vars take precedence over the config file.

## Reference

| JSON key | Environment variable | Default | Description |
|----------|--------------------|---------|-------------|
| `listen_addr` | `PROXY_LISTEN_ADDR` | `:8080` | TCP address to listen on |
| `upstream_url` | `PROXY_UPSTREAM_URL` | `https://api.openai.com/v1` | Base URL of the LLM provider API (fallback) |
| `api_key` | `PROXY_API_KEY` | `$OPENAI_API_KEY` | API key for the upstream provider (fallback) |
| `upstreams` | — | — | Per-model upstream mappings (see below) |
| `expensive_model` | `PROXY_EXPENSIVE_MODEL` | `gpt-4o` | Model name used for complex routes |
| `cheap_model` | `PROXY_CHEAP_MODEL` | `gpt-4o-mini` | Model name used for simple routes |
| `cache_path` | `PROXY_CACHE_PATH` | `proxy-cache.db` | Filesystem path for the SQLite cache database |
| `cache_ttl_seconds` | `PROXY_CACHE_TTL` | `3600` | Time-to-live for cached responses (seconds) |
| `embedding_url` | `PROXY_EMBEDDING_URL` | — | URL of an embedding sidecar for semantic cache (optional) |
| `semantic_threshold` | — | `0.92` | Cosine similarity threshold for semantic cache hit |
| `semantic_max_entries` | — | `1000` | Maximum entries in semantic cache vector index |
| `tool_cache_ttl_seconds` | `PROXY_TOOL_CACHE_TTL` | `300` | TTL for tool-result cache entries (seconds) |

## Routing Toggles

| JSON key | Default | Description |
|----------|---------|-------------|
| `route_tool_calls_to_expensive` | `true` | Route requests with tool definitions or tool-role messages to expensive model |
| `route_long_contexts_to_expensive` | `true` | Route requests with messages over `long_context_threshold` to expensive model |
| `long_context_threshold` | `3000` | Bytes per message above which the request is classified as long context |

## Transform Toggles

| JSON key | Default | Description |
|----------|---------|-------------|
| `max_context_turns` | `20` | Maximum non-system messages before the sliding window activates |
| `compress_token_threshold` | `4000` | Tool-role messages longer than this (bytes) are truncated |
| `system_prompt_dedup` | `true` | Enable system prompt deduplication (hash-and-reference) |
| `token_budget_enabled` | `true` | Enable max_tokens clamping per route |
| `token_budget_cheap` | `1024` | Max tokens ceiling for cheap-route requests |
| `token_budget_expensive` | `4096` | Max tokens ceiling for expensive-route requests |

## Per-Model Upstreams

Use `upstreams` to route different models to different providers. The key is the model name (as set by the router in `expensive_model`/`cheap_model`), or `"*"` for a catch-all fallback.

| `upstreams` entry field | Required | Description |
|-------------------------|----------|-------------|
| `url` | yes | Base URL of the LLM provider API (OpenAI, OpenRouter, together.ai, etc.) |
| `api_key` | no | API key for this provider. Falls back to the global `api_key` |

### Resolution order

1. Exact model match in `upstreams`
2. `"*"` wildcard entry in `upstreams`
3. Global `upstream_url` / `api_key` (auto-registered as `"*"`)

### Resolver compatibility

All providers must expose an OpenAI-compatible `/v1/chat/completions` endpoint:

| Provider | Example URL |
|----------|-------------|
| OpenAI | `https://api.openai.com/v1` |
| OpenRouter | `https://openrouter.ai/api/v1` |
| OpenCode Zen | `https://zen.opencode.ai/v1` |
| Together | `https://api.together.xyz/v1` |
| Groq | `https://api.groq.com/openai/v1` |
| Anthropic (via proxy) | `https://api.anthropic.com/v1` (OpenAI-compatible wrapper) |

### Example — Multi-Provider

```json
{
  "listen_addr": ":9090",
  "upstreams": {
    "gpt-4o":        { "url": "https://api.openai.com/v1", "api_key": "sk-openai-..." },
    "claude-sonnet": { "url": "https://api.anthropic.com/v1", "api_key": "sk-ant-..." },
    "deepseek-v3":   { "url": "https://openrouter.ai/api/v1", "api_key": "sk-or-..." },
    "*":             { "url": "https://zen.opencode.ai/v1", "api_key": "sk-zen-..." }
  },
  "expensive_model": "gpt-4o",
  "cheap_model": "deepseek-v3",
  "cache_path": "/var/data/proxy-cache.db",
  "cache_ttl_seconds": 7200
}
```

### Backward-Compatible Example

```json
{
  "listen_addr": ":9090",
  "upstream_url": "https://api.openai.com/v1",
  "api_key": "sk-...",
  "expensive_model": "gpt-4o",
  "cheap_model": "gpt-4o-mini",
  "cache_path": "/var/data/proxy-cache.db",
  "cache_ttl_seconds": 7200,
  "route_tool_calls_to_expensive": true,
  "route_long_contexts_to_expensive": false,
  "max_context_turns": 30,
  "compress_token_threshold": 2000,
  "system_prompt_dedup": true,
  "token_budget_cheap": 2048,
  "token_budget_expensive": 8192
}
```
