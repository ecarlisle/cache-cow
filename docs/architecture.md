# Architecture

## Code Layout

```
cmd/proxy/main.go           — Entry point. Parses config, wires dependencies, starts HTTP server.
internal/
├── types/types.go           — OpenAI-compatible request/response structs, RouteDecision enum
├── config/config.go         — JSON file + env var config loading with defaults
├── cache/
│   ├── exact.go             — SQLite exact-match cache (SHA256 key, prepared statements)
│   ├── semantic.go          — Embedding sidecar client (HTTP, cosine similarity)
│   └── tool.go              — Tool-result cache (SQLite, keyed by tool name + args + result)
├── router/router.go         — Request classifier: simple vs complex
├── transform/
│   ├── context.go           — Sliding window manager
│   ├── compress.go          — Whitespace compressor and tool truncation
│   ├── dedup.go             — System prompt deduplication (hash + reference)
│   └── budget.go            — Token budget clamping per route
├── metrics/metrics.go       — Per-request tracking + global atomic collector
└── proxy/handler.go         — httputil.ReverseProxy with intercept pipeline
```

## Package Responsibilities

| Package | Owns | Depends On |
|---------|------|------------|
| `types` | Request/response models, route decision enum | — |
| `config` | Loading, defaults, env var overlay | — |
| `cache/exact` | SQLite schema, CRUD, TTL, periodic cleanup | `types`, `modernc.org/sqlite` |
| `cache/semantic` | Embedding HTTP client, cosine similarity | — |
| `router` | Route classification heuristics | `types` |
| `transform/context` | Sliding window + summarization | `types` |
| `transform/compress` | Whitespace normalization, truncation | `types` |
| `transform/dedup` | System prompt hashing and reference substitution | `types` |
| `transform/budget` | Max tokens clamping per route | `types` |
| `metrics` | Per-request tracking, global atomic counters, report formatting | — |
| `proxy` | HTTP handler, pipeline orchestration, upstream forwarding | All of the above |

## Pipeline Ownership

`proxy/handler.go` owns the pipeline flow. Each arrival:

1. Reads and parses the request body, creates a `RequestMetrics` tracker
2. Calls `cache.RequestKey` + `exactCache.Get` for the fast path
3. On exact miss, tries `cache.ToolRequestKey` + `toolCache.Get` for tool pattern match
4. On tool miss, tries `semCache.Lookup` for semantic similarity
5. On semantic miss, calls `router.Decide` for model routing; `resolveUpstream` picks the provider URL + API key per the selected model
6. Runs transforms in order: `dedup`, `ctxMgr.Trim`, `compressor.Compress`, `budget.Apply`
7. Measures per-stage byte savings via JSON serialization at each step
8. Builds a new `httputil.ReverseProxy` to forward the patched request
9. Intercepts the response in `ModifyResponse` to cache it, parse usage, and set savings headers
10. Updates the global `Collector` with per-request metrics
