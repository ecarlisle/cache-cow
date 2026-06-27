package config

import (
	"os"
	"testing"
)

func TestDefaultHasUpstreamURL(t *testing.T) {
	cfg := Default()
	if cfg.UpstreamURL != "https://api.openai.com/v1" {
		t.Errorf("default UpstreamURL: got %q, want https://api.openai.com/v1", cfg.UpstreamURL)
	}
	if cfg.Upstreams != nil {
		t.Errorf("default Upstreams: got %v, want nil", cfg.Upstreams)
	}
}

func TestLoadUpstreams(t *testing.T) {
	json := `{
		"upstreams": {
			"gpt-4o": {"url": "https://api.openai.com/v1", "api_key": "sk-1"},
			"deepseek": {"url": "https://openrouter.ai/api/v1", "api_key": "sk-2"},
			"*": {"url": "https://zen.opencode.ai/v1"}
		}
	}`
	f, _ := os.CreateTemp("", "config-*.json")
	defer os.Remove(f.Name())
	f.WriteString(json)
	f.Close()

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.Upstreams) != 3 {
		t.Fatalf("Upstreams: got %d entries, want 3", len(cfg.Upstreams))
	}

	tests := []struct {
		model  string
		wantURL string
		wantKey string
	}{
		{"gpt-4o", "https://api.openai.com/v1", "sk-1"},
		{"deepseek", "https://openrouter.ai/api/v1", "sk-2"},
		{"*", "https://zen.opencode.ai/v1", ""},
	}
	for _, tt := range tests {
		uc, ok := cfg.Upstreams[tt.model]
		if !ok {
			t.Errorf("Upstreams[%q] not found", tt.model)
			continue
		}
		if uc.URL != tt.wantURL {
			t.Errorf("Upstreams[%q].URL: got %q, want %q", tt.model, uc.URL, tt.wantURL)
		}
		if uc.APIKey != tt.wantKey {
			t.Errorf("Upstreams[%q].APIKey: got %q, want %q", tt.model, uc.APIKey, tt.wantKey)
		}
	}
}

func TestLoadUpstreamsBackwardCompat(t *testing.T) {
	json := `{
		"upstream_url": "https://api.openai.com/v1",
		"api_key": "sk-global"
	}`
	f, _ := os.CreateTemp("", "config-*.json")
	defer os.Remove(f.Name())
	f.WriteString(json)
	f.Close()

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.UpstreamURL != "https://api.openai.com/v1" {
		t.Errorf("UpstreamURL: got %q", cfg.UpstreamURL)
	}
	if cfg.APIKey != "sk-global" {
		t.Errorf("APIKey: got %q", cfg.APIKey)
	}
	if len(cfg.Upstreams) != 0 {
		t.Errorf("Upstreams: got %d entries, want 0", len(cfg.Upstreams))
	}
}
