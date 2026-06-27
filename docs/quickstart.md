# Quick Start

## Prerequisites

- Go 1.26+ (or download a prebuilt binary)
- An API key for your LLM provider (OpenAI, Anthropic, etc.)

## Build

```bash
git clone https://github.com/ecarlisle/cache-cow.git
cd cache-cow
go build -o bin/proxy ./cmd/proxy/
```

The binary is self-contained — no runtime dependencies.

## Configure

Create a `config.json`:

```json
{
  "listen_addr": ":8080",
  "upstream_url": "https://api.openai.com/v1",
  "api_key": "sk-...",
  "expensive_model": "gpt-4o",
  "cheap_model": "gpt-4o-mini"
}
```

Or use environment variables:

```bash
export PROXY_API_KEY="sk-..."
export PROXY_EXPENSIVE_MODEL="gpt-4o"
export PROXY_CHEAP_MODEL="gpt-4o-mini"
```

## Run

```bash
./bin/proxy --config config.json
```

Expected output:

```
starting proxy on :8080
  expensive: gpt-4o
  cheap:     gpt-4o-mini
  upstream:  https://api.openai.com/v1
  cache:     proxy-cache.db (ttl=3600s)
```

## Point an Agent at the Proxy

Configure your agent to use a custom OpenAI-compatible endpoint:

**OpenCode:** Set `OPENAI_BASE_URL=http://localhost:8080/v1` in your environment or `opencode.json`.

**Pi / other agents:** Point their LLM provider URL to `http://localhost:8080/v1`.

The proxy accepts the standard OpenAI chat completions JSON format and returns standard responses. The only difference: you get `X-Cache: hit|tool|semantic|miss` and `X-Route: simple|complex` headers on every response.
