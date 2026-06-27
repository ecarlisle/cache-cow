# Cache Cow

Single-binary Go reverse proxy for `/v1/chat/completions`. Module `github.com/ecarlisle/cache-cow`, Go 1.26, zero CGO, single dependency `modernc.org/sqlite`.

## Essential commands

```bash
go build -o bin/proxy ./cmd/proxy/   # static binary
go test ./...                         # SQLite tests use :memory:
go test ./internal/router -run TestDecide -v
go vet ./...                          # must pass before commit
```

## Pipeline execution order (handler.go:99-181)

1. **Exact cache** — SHA256 hash of model+messages, SQLite lookup. Set-and-forget on upstream response.
2. **Tool result cache** — SHA256(tool_name + args + tool_results). Only needs tool pattern, not full conversation.
3. **Semantic cache** — embedding similarity (separate sidecar process, optional via `embedding_url`).
4. **Router** — `Decide()` returns `RouteSimple` or `RouteComplex` (no error).
5. **System prompt dedup** — SHA256 hash of system message, replace with short reference on repeat.
6. **Context window** — sliding trim, middle messages replaced with placeholder summary.
7. **Compression** — whitespace normalization + tool output truncation.
8. **Token budget** — clamp `max_tokens` per route.

## Architecture

| Entrypoint | `cmd/proxy/main.go` — config load, dep wiring, signal handler |
|------------|---------------------------------------------------------------|
| Routes | `POST /v1/chat/completions` and `/chat/completions` → pipeline; everything else passthrough |
| Metrics | `GET /metrics` — HTML for browser, JSON for API (checks `Accept` header, override with `?format=json`) |
| Dashboard | Embedded HTML in `main.go:dashboardHTML()` — `humanBytes()` and `acceptContains()` are custom byte-search helpers in same file |
| `stream: true` | Not supported — response buffered, streaming fails silently |
| CI/CD | None — no GitHub Actions, no pre-commit |

## Code conventions

- **Zero comments** in production `.go` files. Names carry intent.
- **No error returns on classification decisions**. Cache miss returns `nil, nil`. Router returns `RouteDecision` (no error).
- **Prepared statements** for all SQLite, held at struct level for process lifetime.
- **No generics, no reflect.** `encoding/json` only for message types.
- **`log.Printf` only**, prefix with stage (`CACHE HIT`, `  route:`, `  tool cache hit`).
- **`containsAny`** at `router.go:91` is custom byte-search, not `strings.Contains`. The same pattern is used for `acceptContains` in `main.go:96`.
- **`%%` in HTML templates** — `dashboardHTML()` uses `fmt.Sprintf` on a raw literal, so CSS `100%` must be escaped as `100%%`.

## Extension patterns

| Task | Steps |
|------|-------|
| New config field | Add field + JSON tag to `Config` struct, set default in `Default()`, add `PROXY_` env var in `Load()`, update `docs/configuration.md` |
| New routing heuristic | Add toggle to `RouterConfig`, implement in `Decide()`, wire in `handler.go:44` (New constructor), add table-driven tests, update `docs/routing.md` |
| New cache layer | Implement `Get/Set` contract from `ExactCache`, wire into `handleChat()` before/after existing caches, add test (hit/miss/expiry), update `docs/cache.md` |

Commit after every prompt. Include code changes, tests, and doc updates in the same commit.
