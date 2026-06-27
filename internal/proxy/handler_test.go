package proxy

import (
	"testing"

	"github.com/ecarlisle/cache-cow/internal/config"
)

func TestNewRegistersGlobalFallback(t *testing.T) {
	cfg := config.Default()
	cfg.APIKey = "sk-test"

	p := New(cfg, nil, nil, nil)

	u := p.upstreams["*"]
	if u == nil {
		t.Fatal("no wildcard upstream registered")
	}
	if u.apiKey != "sk-test" {
		t.Errorf("wildcard apiKey: got %q, want sk-test", u.apiKey)
	}
}

func TestNewUpstreamsOverrideWildcard(t *testing.T) {
	cfg := config.Default()
	cfg.APIKey = "sk-global"
	cfg.Upstreams = map[string]config.UpstreamConfig{
		"*": {URL: "https://custom.example.com/v1", APIKey: "sk-custom"},
	}

	p := New(cfg, nil, nil, nil)

	u := p.upstreams["*"]
	if u == nil {
		t.Fatal("no wildcard upstream registered")
	}
	if u.apiKey != "sk-custom" {
		t.Errorf("wildcard apiKey: got %q, want sk-custom", u.apiKey)
	}
}

func TestResolveUpstreamExactMatch(t *testing.T) {
	cfg := config.Default()
	cfg.Upstreams = map[string]config.UpstreamConfig{
		"gpt-4o":  {URL: "https://api.openai.com/v1", APIKey: "sk-1"},
		"claude":  {URL: "https://api.anthropic.com/v1", APIKey: "sk-2"},
		"*":       {URL: "https://fallback.example.com/v1"},
	}
	p := New(cfg, nil, nil, nil)

	u := p.resolveUpstream("gpt-4o")
	if u == nil {
		t.Fatal("resolveUpstream returned nil")
	}
	if u.url.String() != "https://api.openai.com/v1" {
		t.Errorf("url: got %q, want https://api.openai.com/v1", u.url.String())
	}
	if u.apiKey != "sk-1" {
		t.Errorf("apiKey: got %q, want sk-1", u.apiKey)
	}
}

func TestResolveUpstreamWildcardFallback(t *testing.T) {
	cfg := config.Default()
	cfg.Upstreams = map[string]config.UpstreamConfig{
		"gpt-4o": {URL: "https://api.openai.com/v1", APIKey: "sk-1"},
		"*":      {URL: "https://fallback.example.com/v1", APIKey: "sk-fallback"},
	}
	p := New(cfg, nil, nil, nil)

	u := p.resolveUpstream("unknown-model")
	if u == nil {
		t.Fatal("resolveUpstream returned nil")
	}
	if u.url.String() != "https://fallback.example.com/v1" {
		t.Errorf("url: got %q, want https://fallback.example.com/v1", u.url.String())
	}
}

func TestResolveUpstreamGlobalBackwardCompat(t *testing.T) {
	cfg := config.Default()
	cfg.UpstreamURL = "https://global.example.com/v1"
	cfg.APIKey = "sk-global"

	p := New(cfg, nil, nil, nil)

	u := p.resolveUpstream("any-model")
	if u == nil {
		t.Fatal("resolveUpstream returned nil")
	}
	if u.url.String() != "https://global.example.com/v1" {
		t.Errorf("url: got %q, want https://global.example.com/v1", u.url.String())
	}
	if u.apiKey != "sk-global" {
		t.Errorf("apiKey: got %q, want sk-global", u.apiKey)
	}
}

func TestResolveUpstreamReturnsDifferentURLPerModel(t *testing.T) {
	cfg := config.Default()
	cfg.Upstreams = map[string]config.UpstreamConfig{
		"cheap-model":    {URL: "https://cheap.example.com/v1", APIKey: "sk-cheap"},
		"expensive-model": {URL: "https://expensive.example.com/v1", APIKey: "sk-expensive"},
	}
	p := New(cfg, nil, nil, nil)

	cheap := p.resolveUpstream("cheap-model")
	expensive := p.resolveUpstream("expensive-model")

	if cheap.url.String() != "https://cheap.example.com/v1" {
		t.Errorf("cheap url: got %q", cheap.url.String())
	}
	if expensive.url.String() != "https://expensive.example.com/v1" {
		t.Errorf("expensive url: got %q", expensive.url.String())
	}
}

func TestNewSkipsInvalidUpstreamURL(t *testing.T) {
	cfg := config.Default()
	cfg.Upstreams = map[string]config.UpstreamConfig{
		"bad": {URL: "://invalid"},
		"good": {URL: "https://valid.example.com/v1"},
	}
	p := New(cfg, nil, nil, nil)

	if _, ok := p.upstreams["bad"]; ok {
		t.Error("bad upstream should have been skipped")
	}
	if _, ok := p.upstreams["good"]; !ok {
		t.Error("good upstream should have been registered")
	}
}

func TestResolvedUpstreamURLParsed(t *testing.T) {
	cfg := config.Default()
	cfg.Upstreams = map[string]config.UpstreamConfig{
		"test": {URL: "https://example.com/v1"},
	}
	p := New(cfg, nil, nil, nil)

	u := p.resolveUpstream("test")
	if u.url.Scheme != "https" {
		t.Errorf("scheme: got %q, want https", u.url.Scheme)
	}
	if u.url.Host != "example.com" {
		t.Errorf("host: got %q, want example.com", u.url.Host)
	}
	if u.url.Path != "/v1" {
		t.Errorf("path: got %q, want /v1", u.url.Path)
	}
}




