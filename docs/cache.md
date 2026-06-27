# Cache Design

## Exact-Match Cache

The primary cache layer. Uses SQLite for persistence.

### Key Generation

SHA256 is computed over the serialized form of:
- Model name
- Every message role + content + name

This means the same prompt sent to the same model with identical messages produces the same key.

### Storage

```sql
CREATE TABLE response_cache (
    hash TEXT PRIMARY KEY,
    model TEXT NOT NULL,
    response TEXT NOT NULL,
    cached_at INTEGER NOT NULL
);
```

### TTL

Entries expire after `cache_ttl_seconds` (default 3600s, configurable). Expired entries are lazily evicted on read and periodically cleaned by a background goroutine every 10 minutes.

### Hit Rate

Best for deterministic tool calls (read file, list directory, grep) where the same query is repeated. The cache is checked before any routing or transformation — a hit skips the entire downstream pipeline.

## Tool-Result Cache (always on)

A dedicated cache for tool call + result patterns. Lives in the same SQLite database as the exact cache, in a separate `tool_cache` table.

### How It Works

```
Request: [assistant: tool_calls=[read_file("/etc/config")]]
         [tool: "<file content>"]
         ↓
Proxy computes key: SHA256(tool_name + args + tool_call_id + result)
         ↓
   ┌─ Hit: return cached LLM response, skip upstream
   └─ Miss: forward upstream, cache response for next time
```

### Key

Computed from the tool call pattern:
- Tool name + arguments (from `assistant` messages with `tool_calls`)
- Tool call ID + result content (from subsequent `tool` messages)

This means two different conversations that make the same tool call with the same result will hit the same cache entry — even if the surrounding conversation context is different.

### TTL

Entries expire after `tool_cache_ttl_seconds` (default 300s, configurable via `PROXY_TOOL_CACHE_TTL`). Shorter than the exact cache because tool results may become stale (file changed, data updated).

### Pipeline Position

Checked after the exact cache, before the semantic cache. The tool cache is a fast SQLite lookup — no HTTP calls (unlike semantic).

### Response Header

Tool cache hits set `X-Cache: tool`.

## Semantic Cache (optional)

Uses embedding similarity to find approximate matches on exact miss. Lookup runs on the hot path; store fires asynchronously on upstream response.

### Embedding Sidecar

Configured via `embedding_url`. Expected to expose `POST /embed` with:

```json
{"input": "text to embed"}
```

Returns:

```json
{"embedding": [0.012, -0.034, ...]}
```

The `SemanticCache` client sends the messages serialized as a single text blob. The response embedding (a `[]float32`) can be used for cosine similarity lookup against previously cached entries.

### Similarity Scoring

Cosine similarity is computed between the request embedding and cached entry embeddings. Entries above the threshold (default 0.92, configurable via `semantic_threshold`) are served as cache hits.

### Status

Fully wired. Requires a running embedding sidecar (e.g., `llama.cpp --embedding`). Store is asynchronous — entries are written to the SQLite `semantic_cache` table after the upstream response arrives, not on the request path.

## Pipeline Order

```
Agent request
  │
  ▼
┌─ 1. Exact cache (full request hash) ── hit → return
  │             miss
  ▼
┌─ 2. Tool cache (tool call pattern)  ── hit → return
  │             miss
  ▼
┌─ 3. Semantic cache (embedding)      ── hit → return
  │             miss
  ▼
Forward upstream → cache response in all three → return
```
