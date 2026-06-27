package main

import (
	"encoding/json"
	"flag"
	"fmt"
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
	serveJSON := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(c.Snapshot())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if r.URL.Query().Get("format") == "json" || (accept != "" && !acceptContains(accept, "text/html")) {
			serveJSON(w)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(dashboardHTML(c.Snapshot()))
	})
}

func acceptContains(accept, mime string) bool {
	for i := 0; i <= len(accept)-len(mime); i++ {
		if accept[i] == mime[0] {
			match := true
			for j := range mime {
				if accept[i+j] != mime[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

func dashboardHTML(s metrics.CollectorSnapshot) []byte {
	hitPct := s.CacheHitRatio() * 100
	totalSaved := s.TotalSavingsBytes()

	return []byte(fmt.Sprintf(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Cache Cow</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
 *{margin:0;padding:0;box-sizing:border-box}
 body{font-family:ui-monospace,'SF Mono',monospace;background:#0d1117;color:#c9d1d9;padding:2rem}
 h1{font-size:1.25rem;color:#58a6ff;margin-bottom:1.5rem}
 .grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(14rem,1fr));gap:1rem;margin-bottom:2rem}
 .card{background:#161b22;border:1px solid #30363d;border-radius:6px;padding:1.25rem}
 .card .label{font-size:.7rem;text-transform:uppercase;color:#8b949e;letter-spacing:.05em}
 .card .value{font-size:1.5rem;font-weight:600;margin-top:.25rem}
 .card .value.green{color:#3fb950}
 .card .value.blue{color:#58a6ff}
 .card .value.yellow{color:#d29922}
 .card .value.purple{color:#bc8cff}
 .card .value.red{color:#f85149}
 table{width:100%%;border-collapse:collapse;margin-bottom:1rem}
 th,td{text-align:right;padding:.5rem .75rem;border-bottom:1px solid #21262d;font-size:.85rem}
 th{color:#8b949e;font-weight:600;text-transform:uppercase;font-size:.7rem;letter-spacing:.05em}
 td:first-child,th:first-child{text-align:left}
 .total{font-weight:700;color:#58a6ff;border-top:2px solid #30363d}
 .uptime{color:#8b949e;font-size:.8rem;margin-top:2rem;text-align:center}
</style></head>
<body>
<h1>Cache Cow</h1>
<div class="grid">
 <div class="card"><div class="label">Requests</div><div class="value blue">%d</div></div>
 <div class="card"><div class="label">Cache Hit Rate</div><div class="value green">%.1f%%</div></div>
 <div class="card"><div class="label">Total Saved</div><div class="value green">%s</div></div>
 <div class="card"><div class="label">Upstream Calls</div><div class="value yellow">%d</div></div>
</div>
<table>
 <tr><th>Stage</th><th>Count</th><th>Bytes</th></tr>
 <tr><td>Cache saved</td><td>%d hits</td><td>%s</td></tr>
 <tr><td>System dedup</td><td>—</td><td>%s</td></tr>
 <tr><td>Context trim</td><td>%d msgs</td><td>%s</td></tr>
 <tr><td>Compression</td><td>—</td><td>%s</td></tr>
 <tr><td>Budget clamp</td><td>%d tokens</td><td>—</td></tr>
 <tr><td>Upstream bytes</td><td>—</td><td>%s</td></tr>
 <tr class="total"><td>Total saved</td><td></td><td>%s</td></tr>
</table>
<div class="uptime">Uptime: %s</div>
</body></html>`,
		s.Requests, hitPct,
		humanBytes(totalSaved),
		s.UpstreamCalls,
		s.CacheHits, humanBytes(s.CacheSaveBytes),
		humanBytes(s.DedupBytes),
		s.ContextMessages, humanBytes(s.ContextBytes),
		humanBytes(s.CompressBytes),
		s.BudgetTokens,
		humanBytes(s.CacheMissBytes),
		humanBytes(totalSaved),
		s.Uptime,
	))
}

func humanBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	if n < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(n)/(1024*1024*1024))
}
