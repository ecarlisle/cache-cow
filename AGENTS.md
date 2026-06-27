# Cache Cow

Token-conserving LLM proxy. Single-binary Go reverse proxy that intercepts `/v1/chat/completions`, caches, routes, and compresses before forwarding upstream. Docs in `docs/`.

## Pipeline rationale (order matters)

The pipeline stages are ordered by impact — each stage handles what the previous one couldn't eliminate:

1. **Exact cache** (outermost, biggest win) — zero latency, zero cost on hit. Bypasses everything below.
2. **Semantic cache** — embedding similarity lookup on exact miss. Catches paraphrased queries that differ textually but are semantically identical.
3. **Tool result cache** — deterministic tool calls (same name + args + result) get cached LLM responses. Only the tool pattern needs to match, unlike the exact cache which requires the full conversation to be identical. Placed before routing because a hit skips everything below.
4. **Model routing** — if uncached but simple, fork to cheap model before investing in transforms.
5. **System prompt deduplication** — agents repeat the same system prompt in every request. Hash and cache on first sight, replace with short reference on subsequent turns. Often saves 500–2000 tokens per request.
6. **Context window** — sliding window + summarization directly shrinks what gets sent to any model.
7. **Compression** — smaller gain than context window but stacks on top.
8. **Token budgeting** (trivial overlay) — clamp `max_tokens` per route, costs nothing.

## Essential commands

```bash
go build -o bin/proxy ./cmd/proxy/   # static binary, no CGO
go test ./...                         # all tests (SQLite cache uses :memory:)
go test ./internal/router -run TestDecide -v
go vet ./...                          # must pass before commit
```

## Code conventions

- **Zero comments** in production `.go` files. Names carry intent.
- **No error returns on classification decisions**. Use `nil`/zero-value sentinels for expected misses (`cache.Get` returns `nil, nil` on miss).
- **Prepared statements** for all SQLite ops, held at package level for process lifetime.
- **No dependencies beyond stdlib + `modernc.org/sqlite`**. Sidecars (embedding) are separate processes.
- **`log.Printf` only**, prefix with stage (`CACHE HIT`, `  route:`).
- **No generics, no reflect**.

## Architecture notes

| Fact | Detail |
|------|--------|
| Entrypoint | `cmd/proxy/main.go` — parses config, wires deps, starts `http.ListenAndServe` |
| Module | `github.com/ecarlisle/cache-cow` |
| Intercepted paths | `POST /v1/chat/completions` and `/chat/completions` only. Everything else passes through untouched. |
| `stream: true` | **Not supported.** Response is fully buffered; streaming fails silently. |
| Semantic cache | Lookup on exact miss (hot path). Store fires async on upstream response. |
| Tool cache | SQLite table `tool_cache`, keyed by SHA256(tool_name + args + result). Hit returns `X-Cache: tool`. |
| `containsAny` | Custom byte-search in `router.go:91`, not `strings.Contains`. Refactor with care. |
| CI/CD | None. No GitHub Actions, no pre-commit hooks. |

## Agent reminders

- Commit after each prompt with a concise message. Include the code change, its tests, and any doc updates in the same commit.
- New env var? Add field + JSON tag to `Config`, set default in `Default()`, add `PROXY_` env var in `Load()`.
- New routing heuristic? Add toggle to `RouterConfig`, implement in `Decide()`, wire in `proxy/handler.go:38`.
- New cache layer? Implement `Get/Set` contract from `ExactCache`.
- System prompt dedup? Hash the system message, cache it, and on subsequent requests replace with a short reference token before forwarding.
- Tool result caching? Key by `(tool_name + arguments_hash)`. Requires session-level conversation identity to scope cache entries per turn.
- All code changes get tests (success + failure + edge cases) and docs updates.
- When adding or updating a doc file, ensure `README.md` is still accurate — add a row to the relevant table if a new doc was created.

See `docs/development.md` for the full feature-addition workflow.
