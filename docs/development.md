# Development Guide

## Building

```bash
go build -o bin/proxy ./cmd/proxy/
```

The binary is statically linked (pure Go + pure Go SQLite driver) — no CGO, no platform-specific toolchain.

## Testing

```bash
go test ./... -v          # all tests with verbose output
go test ./internal/router # single package
go test -run TestDecide ./internal/router -v  # single test
```

The SQLite cache tests use `:memory:` databases — no filesystem setup needed.

## Code Conventions

See [AGENTS.md](../AGENTS.md) for the full convention list. Key rules:

- No comments in production code. Names carry intent.
- No error returns on validation/classification decisions. Use `nil` or zero-value sentinels.
- Prepared statements for all SQLite operations.
- No external dependencies beyond stdlib + `modernc.org/sqlite`.
- stdlib `log.Printf` for logging only.
- `go vet ./...` must pass before committing.
- Unit tests for all new code covering success, failure, and edge-case paths.

## Adding Features

### Adding a new config value

1. Add the field to `internal/config/config.go` — both the `Config` struct with JSON tag and the `Default()` value.
2. Add the env var override in the `Load()` function.
3. Thread the value through to the relevant package.
4. Update `docs/configuration.md` with the new field.

### Adding a new routing heuristic

1. Add the toggle fields to `RouterConfig` in `internal/router/router.go`.
2. Implement the heuristic in `Decide()`.
3. Add the config wiring in `internal/proxy/handler.go` where the `Router` is constructed.
4. Update `docs/routing.md` with the new heuristic.
5. Add table-driven tests in `internal/router/router_test.go`.

### Adding a new cache layer

1. Implement the `Get(hash string) (*CacheEntry, error)` and `Set(hash string, entry *CacheEntry) error` contract.
2. Wire it into `proxy/handler.go` — typically calling it before or after the exact cache.
3. Update `docs/cache.md` with the new layer.
4. Add tests covering hit, miss, expiry, and edge cases.

## Deploying

The proxy is a single binary. Copy it to the target machine and run:

```bash
./bin/proxy --config /etc/proxy/config.json
```

The SQLite cache database is created at `cache_path` on first run. It can be placed on a tmpfs for faster cache access (cache is disposable — loss only causes more upstream calls).
