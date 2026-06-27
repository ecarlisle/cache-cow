# Cache Cow

Token-conserving LLM proxy. Sits between agents (OpenCode, Pi, etc.) and their LLM provider. Intercepts `/v1/chat/completions`, applies a pipeline to reduce cost, then forwards upstream.

## Documentation

### For Users

| Document | Description |
|----------|-------------|
| [Getting Started](docs/getting-started.md) | Install, configure, and run; includes OpenCode & Pi Coder setup |
| [Configuration](docs/configuration.md) | All config options — JSON file and env vars |
| [Pipeline](docs/pipeline.md) | How requests flow through caching, routing, and transforms |
| [Response Headers](docs/response-headers.md) | Observability headers for monitoring |

### For Programmers

| Document | Description |
|----------|-------------|
| [Architecture](docs/architecture.md) | Code layout, package responsibilities, pipeline |
| [Development](docs/development.md) | Building, testing, code conventions, extending |
| [Cache Design](docs/cache.md) | Exact-match SQLite cache, semantic embedding sidecar |
| [Routing Heuristics](docs/routing.md) | How simple vs complex decisions work |
| [Context Management](docs/context.md) | Sliding window and summarization |
| [Compression](docs/compression.md) | Whitespace normalization, truncation rules |
