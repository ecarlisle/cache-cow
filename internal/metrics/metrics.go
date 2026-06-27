package metrics

import (
	"fmt"
	"sync/atomic"
	"time"
)

type RequestMetrics struct {
	RequestKey string
	Model      string
	Route      string
	CacheHit   bool
	CacheType  string
	BodyBytes  int

	SystemPromptLength int
	MessagesBefore     int
	MessagesAfter      int
	BytesBefore        int
	BytesAfterDedup    int
	BytesAfterContext  int
	BytesAfterCompress int
	MaxTokensBefore    int
	MaxTokensAfter     int

	UpstreamPromptTokens     int
	UpstreamCompletionTokens int
	UpstreamTotalTokens      int
	UpstreamModel            string
	UpstreamLatencyMs        int64
}

func (rm *RequestMetrics) DedupSavings() int {
	return rm.BytesBefore - rm.BytesAfterDedup
}

func (rm *RequestMetrics) ContextSavings() int {
	return rm.BytesAfterDedup - rm.BytesAfterContext
}

func (rm *RequestMetrics) CompressSavings() int {
	return rm.BytesAfterContext - rm.BytesAfterCompress
}

func (rm *RequestMetrics) TotalTransformSavings() int {
	return rm.BytesBefore - rm.BytesAfterCompress
}

func (rm *RequestMetrics) BudgetSavings() int {
	if rm.MaxTokensAfter <= 0 {
		return 0
	}
	s := rm.MaxTokensBefore - rm.MaxTokensAfter
	if s < 0 {
		return 0
	}
	return s
}

func (rm *RequestMetrics) MessagesRemoved() int {
	r := rm.MessagesBefore - rm.MessagesAfter
	if r < 0 {
		return 0
	}
	return r
}

type Collector struct {
	requests         atomic.Int64
	cacheHits        atomic.Int64
	semanticHits     atomic.Int64
	toolHits         atomic.Int64
	cacheSaveBytes   atomic.Int64
	cacheMissBytes   atomic.Int64

	dedupBytes       atomic.Int64
	contextMessages  atomic.Int64
	contextBytes     atomic.Int64
	compressBytes    atomic.Int64
	budgetTokens     atomic.Int64

	upstreamPromptTokens     atomic.Int64
	upstreamCompletionTokens atomic.Int64
	upstreamTotalTokens      atomic.Int64
	upstreamCalls           atomic.Int64

	startTime time.Time
}

func NewCollector() *Collector {
	return &Collector{startTime: time.Now()}
}

func (c *Collector) Track(rm *RequestMetrics) {
	c.requests.Add(1)

	if rm.CacheHit {
		c.cacheHits.Add(1)
		switch rm.CacheType {
		case "semantic":
			c.semanticHits.Add(1)
		case "tool":
			c.toolHits.Add(1)
		}
		if rm.BodyBytes > 0 {
			c.cacheSaveBytes.Add(int64(rm.BodyBytes))
		}
		return
	}

	c.cacheMissBytes.Add(int64(rm.BytesAfterCompress))
	c.upstreamCalls.Add(1)
	c.upstreamPromptTokens.Add(int64(rm.UpstreamPromptTokens))
	c.upstreamCompletionTokens.Add(int64(rm.UpstreamCompletionTokens))
	c.upstreamTotalTokens.Add(int64(rm.UpstreamTotalTokens))

	c.dedupBytes.Add(int64(rm.DedupSavings()))
	c.contextMessages.Add(int64(rm.MessagesRemoved()))
	c.contextBytes.Add(int64(rm.ContextSavings()))
	c.compressBytes.Add(int64(rm.CompressSavings()))
	c.budgetTokens.Add(int64(rm.BudgetSavings()))
}

func (c *Collector) Snapshot() CollectorSnapshot {
	return CollectorSnapshot{
		Requests:                c.requests.Load(),
		CacheHits:              c.cacheHits.Load(),
		SemanticHits:           c.semanticHits.Load(),
		ToolHits:               c.toolHits.Load(),
		CacheSaveBytes:         c.cacheSaveBytes.Load(),
		CacheMissBytes:         c.cacheMissBytes.Load(),
		UpstreamCalls:          c.upstreamCalls.Load(),
		UpstreamTotalTokens:    c.upstreamTotalTokens.Load(),
		UpstreamPromptTokens:   c.upstreamPromptTokens.Load(),
		UpstreamCompletionTokens: c.upstreamCompletionTokens.Load(),
		DedupBytes:             c.dedupBytes.Load(),
		ContextBytes:           c.contextBytes.Load(),
		ContextMessages:        c.contextMessages.Load(),
		CompressBytes:          c.compressBytes.Load(),
		BudgetTokens:           c.budgetTokens.Load(),
		Uptime:                 time.Since(c.startTime).Round(time.Second).String(),
	}
}

type CollectorSnapshot struct {
	Requests                int64
	CacheHits              int64
	SemanticHits           int64
	ToolHits               int64
	CacheSaveBytes         int64
	CacheMissBytes         int64
	UpstreamCalls          int64
	UpstreamTotalTokens    int64
	UpstreamPromptTokens   int64
	UpstreamCompletionTokens int64
	DedupBytes             int64
	ContextBytes           int64
	ContextMessages        int64
	CompressBytes          int64
	BudgetTokens           int64
	Uptime                 string
}

func (s CollectorSnapshot) CacheHitRatio() float64 {
	if s.Requests == 0 {
		return 0
	}
	return float64(s.CacheHits) / float64(s.Requests)
}

func (s CollectorSnapshot) TotalSavingsBytes() int64 {
	return s.CacheSaveBytes + s.DedupBytes + s.ContextBytes + s.CompressBytes
}

func (s CollectorSnapshot) FormatReport() string {
	hitPct := s.CacheHitRatio() * 100
	totalSavings := s.TotalSavingsBytes()
	runs := s.Requests

	return fmt.Sprintf(`┌─ Proxy savings report ─────────────────────────────────┐
│  Uptime:       %14s                         │
│  Requests:     %14d                         │
│  Cache hits:   %14d  (%.1f%%)               │
│    exact:      %14d                         │
│    semantic:   %14d                         │
│    tool:       %14d                         │
│  Upstream:     %14d calls                   │
│                %14d total tokens            │
│                %14d bytes sent             │
├─ Per-stage savings ──────────────────────────────────────┤
│  Cache saved:  %14d bytes                   │
│  System dedup: %14d bytes                   │
│  Context trim: %14d bytes  (%d msgs)        │
│  Compression:  %14d bytes                   │
│  Budget clamp: %14d max_tokens              │
│  ─────────────────────────────────────────────────────── │
│  Total saved:  %14d bytes                   │
└──────────────────────────────────────────────────────────┘`,
		s.Uptime, runs,
		s.CacheHits, hitPct,
		s.CacheHits-s.SemanticHits-s.ToolHits, s.SemanticHits, s.ToolHits,
		s.UpstreamCalls, s.UpstreamTotalTokens, s.CacheMissBytes,
		s.CacheSaveBytes,
		s.DedupBytes, s.ContextBytes, s.ContextMessages, s.CompressBytes, s.BudgetTokens,
		totalSavings,
	)
}
