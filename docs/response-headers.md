# Response Headers

Every proxied response includes observability headers so agents and tooling can understand what the proxy did.

## `X-Cache`

| Value | Meaning |
|-------|---------|
| `hit` | Response was served from the exact-match SQLite cache. No upstream call was made. |
| `tool` | Response was served from the tool-result cache. No upstream call was made. |
| `semantic` | Response was served from the semantic (embedding similarity) cache. |
| `miss` | Response came from the upstream provider. May have been cached for future requests. |

**Not present** on non-chat endpoints (pass-through without pipeline).

## `X-Route`

| Value | Meaning |
|-------|---------|
| `simple` | Request was classified as simple and sent to the cheap model. |
| `complex` | Request was classified as complex and sent to the expensive model. |

Only present when `X-Cache: miss`.

## `X-Original-Model`

The model the agent originally requested. Present when `X-Cache: miss`. Useful when the router changed the model — compare `X-Original-Model` against the `model` field in the response body to see which model actually served the request.

## `X-Savings-Bytes` and per-stage headers

| Header | Meaning |
|--------|---------|
| `X-Savings-Bytes` | Total bytes removed by the pipeline (dedup + context + compression) |
| `X-Savings-Dedup` | Bytes removed by system prompt deduplication |
| `X-Savings-Context` | Bytes removed by context window trimming |
| `X-Savings-Compress` | Bytes removed by whitespace normalization and tool truncation |
| `X-Savings-Budget` | `max_tokens` reduced by token budgeting |

Present on cache misses when at least one transform stage saved bytes.

## Metrics Endpoint

The proxy serves a JSON metrics endpoint at `GET /metrics`:

```
curl http://localhost:8080/metrics
```

```json
{
  "Requests": 847,
  "CacheHits": 312,
  "SemanticHits": 18,
  "ToolHits": 4,
  "CacheSaveBytes": 468000,
  "CacheMissBytes": 234000,
  "UpstreamCalls": 529,
  "UpstreamTotalTokens": 118360,
  "UpstreamPromptTokens": 82450,
  "UpstreamCompletionTokens": 35910,
  "DedupBytes": 42000,
  "ContextBytes": 24800,
  "ContextMessages": 158,
  "CompressBytes": 6500,
  "BudgetTokens": 5400,
  "Uptime": "2h15m30s"
}
```

Numbers reset when the proxy restarts. See the [Getting Started](getting-started.md) guide for example output.

## Shutdown Summary

On `SIGINT` or `SIGTERM`, the proxy prints a formatted savings report before exiting:

```
┌─ Proxy savings report ─────────────────────────────────┐
│  Uptime:        2h15m30s                                │
│  Requests:      847                                     │
│  Cache hits:    312  (36.8%)                            │
│    exact:       294                                     │
│    semantic:     18                                     │
│    tool:          0                                     │
│  Upstream:      529 calls, 118,360 total tokens         │
│                         234,000 bytes sent              │
├─ Per-stage savings ──────────────────────────────────────┤
│  Cache saved:  468,000 bytes                            │
│  System dedup:  42,000 bytes                            │
│  Context trim:  24,800 bytes  (158 msgs)                │
│  Compression:    6,500 bytes                            │
│  Budget clamp:  5,400 max_tokens                        │
│  ────────────────────────────────────────────────────── │
│  Total saved:  541,300 bytes                            │
└──────────────────────────────────────────────────────────┘
```
