# Configuration

Configuration is loaded from a JSON file (optional, `--config` flag) and overlaid with environment variables. Env vars take precedence over the config file.

## Reference

| JSON key | Environment variable | Default | Description |
|----------|--------------------|---------|-------------|
| `listen_addr` | `PROXY_LISTEN_ADDR` | `:8080` | TCP address to listen on |
| `upstream_url` | `PROXY_UPSTREAM_URL` | `https://api.openai.com/v1` | Base URL of the LLM provider API |
| `api_key` | `PROXY_API_KEY` | `$OPENAI_API_KEY` | API key for the upstream provider |
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

## Example Config

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
