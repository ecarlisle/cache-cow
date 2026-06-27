package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/ecarlisle/cache-cow/internal/cache"
	"github.com/ecarlisle/cache-cow/internal/config"
	"github.com/ecarlisle/cache-cow/internal/metrics"
	"github.com/ecarlisle/cache-cow/internal/router"
	"github.com/ecarlisle/cache-cow/internal/transform"
	"github.com/ecarlisle/cache-cow/internal/types"
)

type Proxy struct {
	cfg        *config.Config
	exactCache *cache.ExactCache
	semCache   *cache.SemanticCache
	toolCache  *cache.ToolCache
	router     *router.Router
	ctxMgr     *transform.ContextManager
	compressor *transform.Compressor
	deduper    *transform.SystemPromptDeduper
	budget     *transform.TokenBudget
	collector  *metrics.Collector
	upstream   *url.URL
}

func New(cfg *config.Config, exactCache *cache.ExactCache, semCache *cache.SemanticCache, toolCache *cache.ToolCache) *Proxy {
	upstream, _ := url.Parse(cfg.UpstreamURL)

	return &Proxy{
		cfg:        cfg,
		exactCache: exactCache,
		semCache:   semCache,
		toolCache:  toolCache,
		router: router.New(router.RouterConfig{
			RouteToolCalls:        cfg.RouteToolCalls,
			RouteLongContexts:     cfg.RouteLongContexts,
			LongContextThreshold:  cfg.LongContextThreshold,
		}),
		ctxMgr:     transform.NewContextManager(cfg.MaxContextTurns),
		compressor: transform.NewCompressor(cfg.CompressTokenThreshold),
		deduper:    transform.NewSystemPromptDeduper(100, cfg.SystemPromptDedup),
		budget:     transform.NewTokenBudget(cfg.TokenBudgetCheap, cfg.TokenBudgetExpensive, cfg.TokenBudgetEnabled),
		collector:  metrics.NewCollector(),
		upstream:   upstream,
	}
}

func (p *Proxy) Collector() *metrics.Collector {
	return p.collector
}

func (p *Proxy) Handler() http.Handler {
	rp := &httputil.ReverseProxy{
		Director:       p.director,
		ModifyResponse: p.modifyResponse,
		ErrorHandler:   p.errorHandler,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && (r.URL.Path == "/v1/chat/completions" || r.URL.Path == "/chat/completions") {
			p.handleChat(w, r)
			return
		}
		rp.ServeHTTP(w, r)
	})
}

func (p *Proxy) handleChat(w http.ResponseWriter, r *http.Request) {
	m := &metrics.RequestMetrics{}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "reading body", http.StatusBadRequest)
		return
	}
	r.Body.Close()

	var req types.ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "parsing request", http.StatusBadRequest)
		return
	}

	m.BodyBytes = len(body)
	m.RequestKey, _ = cache.RequestKey(&req)
	m.Model = req.Model
	if req.MaxTokens != nil {
		m.MaxTokensBefore = *req.MaxTokens
	}

	if entry, err := p.exactCache.Get(m.RequestKey); err == nil && entry != nil {
		log.Printf("CACHE HIT [%s] %s (cached at %s)", m.RequestKey[:12], req.Model, time.Unix(entry.CachedAt, 0).Format(time.RFC3339))
		m.CacheHit = true
		m.CacheType = "exact"
		p.collector.Track(m)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "hit")
		w.Write(entry.Response)
		return
	}

	log.Printf("CACHE MISS [%s] %s", m.RequestKey[:12], req.Model)

	if p.toolCache != nil {
		if toolKey, ok := cache.ToolRequestKey(req.Messages); ok {
			if entry, err := p.toolCache.Get(toolKey); err == nil && entry != nil {
				log.Printf("  tool cache hit [%s]", toolKey[:12])
				m.CacheHit = true
				m.CacheType = "tool"
				p.collector.Track(m)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "tool")
				w.Write(entry.Response)
				return
			}
		}
	}

	if p.semCache.Enabled() {
		messagesJSON, _ := json.Marshal(req.Messages)
		if len(messagesJSON) > 0 && len(messagesJSON) < 10000 {
			if entry, err := p.semCache.Lookup(string(messagesJSON)); err == nil && entry != nil {
				log.Printf("  semantic hit (cached at %s)", time.Unix(entry.CachedAt, 0).Format(time.RFC3339))
				m.CacheHit = true
				m.CacheType = "semantic"
				p.collector.Track(m)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "semantic")
				w.Write(entry.Response)
				return
			}
		}
	}

	route := p.router.Decide(&req)
	log.Printf("  route: %s", route)
	m.Route = route.String()

	targetModel := p.cfg.CheapModel
	if route == types.RouteComplex {
		targetModel = p.cfg.ExpensiveModel
	}

	m.Model = targetModel
	req.Model = targetModel

	m.MessagesBefore = len(req.Messages)

	beforeJSON, _ := json.Marshal(req)
	m.BytesBefore = len(beforeJSON)
	m.SystemPromptLength = systemPromptBytes(&req)

	if deduped, ok := p.deduper.Dedupe(&req); ok {
		log.Printf("  system prompt deduped")
		req = *deduped
	}
	afterDedupJSON, _ := json.Marshal(req)
	m.BytesAfterDedup = len(afterDedupJSON)

	req = *p.ctxMgr.Trim(&req)
	m.MessagesAfter = len(req.Messages)
	afterContextJSON, _ := json.Marshal(req)
	m.BytesAfterContext = len(afterContextJSON)

	req = *p.compressor.Compress(&req)
	afterCompressJSON, _ := json.Marshal(req)
	m.BytesAfterCompress = len(afterCompressJSON)

	req = *p.budget.Apply(&req, route)
	if req.MaxTokens != nil {
		m.MaxTokensAfter = *req.MaxTokens
	}

	patched, _ := json.Marshal(req)
	r.Body = io.NopCloser(bytes.NewReader(patched))
	r.ContentLength = int64(len(patched))

	upstreamStart := time.Now()

	rp := httputil.ReverseProxy{
		Director: p.director,
		ModifyResponse: func(res *http.Response) error {
			if err := p.cacheResponse(res, &req, m.RequestKey); err != nil {
				log.Printf("  cache write error: %v", err)
			}

			respBody, _ := io.ReadAll(res.Body)
			res.Body.Close()
			res.Body = io.NopCloser(bytes.NewReader(respBody))

			var chatResp types.ChatResponse
			if json.Unmarshal(respBody, &chatResp) == nil {
				if chatResp.Usage != nil {
					m.UpstreamPromptTokens = chatResp.Usage.PromptTokens
					m.UpstreamCompletionTokens = chatResp.Usage.CompletionTokens
					m.UpstreamTotalTokens = chatResp.Usage.TotalTokens
				}
				m.UpstreamModel = chatResp.Model
			}

			m.UpstreamLatencyMs = time.Since(upstreamStart).Milliseconds()

			p.collector.Track(m)

			res.Header.Set("X-Cache", "miss")
			res.Header.Set("X-Route", m.Route)
			res.Header.Set("X-Original-Model", m.Model)

			if s := m.TotalTransformSavings(); s > 0 {
				res.Header.Set("X-Savings-Bytes", fmt.Sprintf("%d", s))
				res.Header.Set("X-Savings-Dedup", fmt.Sprintf("%d", m.DedupSavings()))
				res.Header.Set("X-Savings-Context", fmt.Sprintf("%d", m.ContextSavings()))
				res.Header.Set("X-Savings-Compress", fmt.Sprintf("%d", m.CompressSavings()))
			}
			if b := m.BudgetSavings(); b > 0 {
				res.Header.Set("X-Savings-Budget", fmt.Sprintf("%d", b))
			}

			return nil
		},
		ErrorHandler: p.errorHandler,
	}

	rp.ServeHTTP(w, r)
}

func systemPromptBytes(req *types.ChatRequest) int {
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			return len(msg.Content)
		}
	}
	return 0
}

func (p *Proxy) director(r *http.Request) {
	r.URL.Scheme = p.upstream.Scheme
	r.URL.Host = p.upstream.Host
	r.URL.Path = p.upstream.Path + r.URL.Path
	r.Host = p.upstream.Host

	if p.cfg.APIKey != "" {
		r.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}
	r.Header.Set("Accept", "application/json")
}

func (p *Proxy) modifyResponse(res *http.Response) error {
	return nil
}

func (p *Proxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("upstream error: %v", err)
	http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
}

func (p *Proxy) cacheResponse(res *http.Response, req *types.ChatRequest, key string) error {
	if res.StatusCode != http.StatusOK {
		return nil
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	res.Body.Close()
	res.Body = io.NopCloser(bytes.NewReader(body))

	var chatResp types.ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil
	}

	entry := &cache.CacheEntry{
		Response: body,
		Model:    chatResp.Model,
		CachedAt: time.Now().Unix(),
	}

	if err := p.exactCache.Set(key, entry); err != nil {
		return fmt.Errorf("cache set: %w", err)
	}

	if p.semCache.Enabled() {
		messagesJSON, _ := json.Marshal(req.Messages)
		if len(messagesJSON) > 0 && len(messagesJSON) < 10000 {
			go p.semCache.Store(string(messagesJSON), body, chatResp.Model)
		}
	}

	if p.toolCache != nil {
		if toolKey, ok := cache.ToolRequestKey(req.Messages); ok {
			toolEntry := &cache.ToolCacheEntry{
				Response: body,
				Model:    chatResp.Model,
				CachedAt: time.Now().Unix(),
			}
			go p.toolCache.Set(toolKey, toolEntry)
		}
	}

	return nil
}
