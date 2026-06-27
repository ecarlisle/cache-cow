package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/ecarlisle/cache-cow/internal/cache"
	"github.com/ecarlisle/cache-cow/internal/config"
	"github.com/ecarlisle/cache-cow/internal/metrics"
	"github.com/ecarlisle/cache-cow/internal/proxy"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	if cfg.APIKey == "" {
		log.Fatal("API key required: set PROXY_API_KEY env, OPENAI_API_KEY env, or api_key in config")
	}

	exactCache, err := cache.NewExactCache(cfg.CachePath, cfg.CacheTTL)
	if err != nil {
		log.Fatalf("opening cache: %v", err)
	}
	defer exactCache.Close()

	semCache := cache.NewSemanticCache(cfg.EmbeddingURL, cfg.CachePath, cfg.SemanticThreshold, cfg.SemanticMaxEntries)
	if cfg.EmbeddingURL != "" {
		log.Printf("semantic cache enabled at %s (threshold=%.2f, max=%d)", cfg.EmbeddingURL, cfg.SemanticThreshold, cfg.SemanticMaxEntries)
	} else {
		log.Printf("semantic cache disabled (no embedding_url configured)")
	}
	defer semCache.Close()

	toolCache, err := cache.NewToolCache(cfg.CachePath, cfg.ToolCacheTTL)
	if err != nil {
		log.Fatalf("opening tool cache: %v", err)
	}
	defer toolCache.Close()
	log.Printf("tool result cache enabled (ttl=%ds)", cfg.ToolCacheTTL)

	p := proxy.New(cfg, exactCache, semCache, toolCache)

	mux := http.NewServeMux()
	mux.Handle("/metrics", metricsHandler(p.Collector()))
	mux.Handle("/", p.Handler())

	log.Printf("starting proxy on %s", cfg.ListenAddr)
	log.Printf("  expensive: %s", cfg.ExpensiveModel)
	log.Printf("  cheap:     %s", cfg.CheapModel)
	log.Printf("  upstream:  %s", cfg.UpstreamURL)
	log.Printf("  cache:     %s (ttl=%ds)", cfg.CachePath, cfg.CacheTTL)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Printf("\n%s", p.Collector().Snapshot().FormatReport())
		os.Exit(0)
	}()

	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func metricsHandler(c *metrics.Collector) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(c.Snapshot())
	})
}
