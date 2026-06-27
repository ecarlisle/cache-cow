package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

type UpstreamConfig struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key,omitempty"`
}

type Config struct {
	ListenAddr  string `json:"listen_addr"`
	UpstreamURL string `json:"upstream_url"`
	APIKey      string `json:"api_key"`
	Upstreams   map[string]UpstreamConfig `json:"upstreams,omitempty"`

	ExpensiveModel string `json:"expensive_model"`
	CheapModel     string `json:"cheap_model"`

	CachePath string `json:"cache_path"`
	CacheTTL  int    `json:"cache_ttl_seconds"`

	ToolCacheTTL int `json:"tool_cache_ttl_seconds"`

	EmbeddingURL         string  `json:"embedding_url"`
	SemanticThreshold    float64 `json:"semantic_threshold"`
	SemanticMaxEntries   int     `json:"semantic_max_entries"`

	SystemPromptDedup bool `json:"system_prompt_dedup"`

	TokenBudgetEnabled  bool `json:"token_budget_enabled"`
	TokenBudgetCheap    int `json:"token_budget_cheap"`
	TokenBudgetExpensive int `json:"token_budget_expensive"`

	MaxContextTurns int `json:"max_context_turns"`
	CompressTokenThreshold int `json:"compress_token_threshold"`

	RouteToolCalls bool `json:"route_tool_calls_to_expensive"`
	RouteLongContexts bool `json:"route_long_contexts_to_expensive"`
	LongContextThreshold int `json:"long_context_threshold"`
}

func Default() *Config {
	return &Config{
		ListenAddr:             ":8080",
		UpstreamURL:           "https://api.openai.com/v1",
		APIKey:                os.Getenv("OPENAI_API_KEY"),
		ExpensiveModel:        "gpt-4o",
		CheapModel:            "gpt-4o-mini",
		CachePath:             "proxy-cache.db",
		CacheTTL:              3600,
		ToolCacheTTL:          300,
		EmbeddingURL:          "",
		SemanticThreshold:     0.92,
		SemanticMaxEntries:    1000,
		SystemPromptDedup:     true,
		TokenBudgetEnabled:    true,
		TokenBudgetCheap:      1024,
		TokenBudgetExpensive:  4096,
		MaxContextTurns:       20,
		CompressTokenThreshold: 4000,
		RouteToolCalls:        true,
		RouteLongContexts:     true,
		LongContextThreshold:  3000,
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading config: %w", err)
		}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config: %w", err)
		}
	}

	if v := os.Getenv("PROXY_LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("PROXY_UPSTREAM_URL"); v != "" {
		cfg.UpstreamURL = v
	}
	if v := os.Getenv("PROXY_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("PROXY_EXPENSIVE_MODEL"); v != "" {
		cfg.ExpensiveModel = v
	}
	if v := os.Getenv("PROXY_CHEAP_MODEL"); v != "" {
		cfg.CheapModel = v
	}
	if v := os.Getenv("PROXY_CACHE_PATH"); v != "" {
		cfg.CachePath = v
	}
	if v := os.Getenv("PROXY_CACHE_TTL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.CacheTTL = n
		}
	}
	if v := os.Getenv("PROXY_TOOL_CACHE_TTL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ToolCacheTTL = n
		}
	}
	if v := os.Getenv("PROXY_EMBEDDING_URL"); v != "" {
		cfg.EmbeddingURL = v
	}
	if v := os.Getenv("PROXY_SEMANTIC_THRESHOLD"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.SemanticThreshold = n
		}
	}

	return cfg, nil
}
