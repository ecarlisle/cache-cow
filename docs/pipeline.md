# Pipeline

Every incoming `/v1/chat/completions` request flows through these stages:

```
Agent request
    │
    ▼
┌─ 1. Exact Cache ── hit ──► response (0 latency, 0 cost)
│   miss
    ▼
┌─ 2. Semantic Cache ── hit ──► response (embedding similarity)
│   miss
    ▼
┌─ 3. Router ─── simple → cheap model
│              complex → expensive model
    ▼
┌─ 4. System Prompt Dedup ── hash + reference substitution
    ▼
┌─ 5. Context Window ── sliding trim + middle summarization
    ▼
┌─ 6. Compression ── whitespace + tool truncation
    ▼
┌─ 7. Token Budget ── clamp max_tokens per route
    ▼
Forward upstream → cache response → set headers → return to agent
```

## Stage Details

### 1. Exact Cache (SHA256 key, SQLite)

The request is hashed (SHA256 of model + all messages). The SQLite cache is checked. On hit, the cached response is returned immediately with `X-Cache: hit`. The downstream pipeline is skipped entirely — this is the fast path.

Cache entries have a configurable TTL. Expired entries are cleaned up periodically.

### 2. Semantic Cache (embedding similarity)

If the exact cache misses and an embedding sidecar is configured, the request is embedded and compared against previously cached embeddings. On a cosine similarity match above the threshold (default 0.92), the cached response is returned with `X-Cache: semantic`.

Semantic cache hits catch paraphrased queries that differ textually but are semantically identical.

### 3. Router

The request is classified as `simple` or `complex` based on heuristics (see [Routing](routing.md)). Simple requests go to the cheap model, complex to the expensive model. The routing decision is emitted as `X-Route: simple|complex`.

### 4. System Prompt Dedup

Agents repeat the same system prompt in every request. The deduper hashes the system message content and caches it. On subsequent requests with the same system prompt, the full content is replaced with a short reference (`[system prompt cached: {hash}]`), saving 500–2000 tokens per request in agent-style conversations.

### 5. Context Window

A sliding window trims the conversation to `max_context_turns` messages. System messages are always preserved. The first half and last half of non-system messages are kept; the middle turns are replaced with a one-line summary placeholder.

### 6. Compression

Whitespace is normalized (tabs → spaces, collapsed spaces), empty messages are dropped, and tool-role messages exceeding the token threshold are truncated. Structured content (JSON objects/arrays in content) is left untouched.

### 7. Token Budget

The `max_tokens` field is clamped per route. Simple requests are capped at `token_budget_cheap` (default 1024). Complex requests at `token_budget_expensive` (default 4096). The agent's requested value is respected if it is lower than the cap.

## Metrics and Observability

The proxy tracks per-stage savings and exposes them in three ways:

| Method | Description |
|--------|-------------|
| **Response headers** | `X-Savings-Bytes`, `X-Original-Model` on every response |
| **`GET /metrics`** | Live JSON snapshot of all counters |
| **Shutdown report** | Formatted table on SIGINT/SIGTERM |

See [Response Headers](response-headers.md) for full details.
