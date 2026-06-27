package metrics

import (
	"testing"
)

func TestRequestMetricsSavings(t *testing.T) {
	rm := &RequestMetrics{
		BytesBefore:        1000,
		BytesAfterDedup:    800,
		BytesAfterContext:  600,
		BytesAfterCompress: 500,
	}

	if got := rm.DedupSavings(); got != 200 {
		t.Errorf("DedupSavings: got %d, want 200", got)
	}
	if got := rm.ContextSavings(); got != 200 {
		t.Errorf("ContextSavings: got %d, want 200", got)
	}
	if got := rm.CompressSavings(); got != 100 {
		t.Errorf("CompressSavings: got %d, want 100", got)
	}
	if got := rm.TotalTransformSavings(); got != 500 {
		t.Errorf("TotalTransformSavings: got %d, want 500", got)
	}
}

func TestRequestMetricsSavingsZero(t *testing.T) {
	rm := &RequestMetrics{}
	if got := rm.DedupSavings(); got != 0 {
		t.Errorf("DedupSavings: got %d, want 0", got)
	}
	if got := rm.TotalTransformSavings(); got != 0 {
		t.Errorf("TotalTransformSavings: got %d, want 0", got)
	}
}

func TestRequestMetricsBudgetSavings(t *testing.T) {
	tests := []struct {
		name string
		before int
		after  int
		want   int
	}{
		{"clamped", 4096, 1024, 3072},
		{"no change", 1024, 1024, 0},
		{"zero after", 4096, 0, 0},
		{"both zero", 0, 0, 0},
		{"increase ignored", 1024, 4096, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm := &RequestMetrics{MaxTokensBefore: tt.before, MaxTokensAfter: tt.after}
			if got := rm.BudgetSavings(); got != tt.want {
				t.Errorf("BudgetSavings: got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRequestMetricsMessagesRemoved(t *testing.T) {
	tests := []struct {
		name   string
		before int
		after  int
		want   int
	}{
		{"removed 5", 10, 5, 5},
		{"none removed", 5, 5, 0},
		{"negative clamped", 3, 10, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm := &RequestMetrics{MessagesBefore: tt.before, MessagesAfter: tt.after}
			if got := rm.MessagesRemoved(); got != tt.want {
				t.Errorf("MessagesRemoved: got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCollectorTrackAndSnapshot(t *testing.T) {
	c := NewCollector()

	c.Track(&RequestMetrics{
		CacheHit:   true,
		CacheType:  "exact",
		BodyBytes:  500,
	})
	c.Track(&RequestMetrics{
		RequestKey: "abc",
		BytesBefore:        1000,
		BytesAfterDedup:    800,
		BytesAfterContext:  600,
		BytesAfterCompress: 500,
		MaxTokensBefore:    4096,
		MaxTokensAfter:     1024,
		MessagesBefore:     10,
		MessagesAfter:      5,
		UpstreamTotalTokens: 150,
	})

	s := c.Snapshot()
	if s.Requests != 2 {
		t.Errorf("Requests: got %d, want 2", s.Requests)
	}
	if s.CacheHits != 1 {
		t.Errorf("CacheHits: got %d, want 1", s.CacheHits)
	}
	if s.UpstreamCalls != 1 {
		t.Errorf("UpstreamCalls: got %d, want 1", s.UpstreamCalls)
	}
	if s.UpstreamTotalTokens != 150 {
		t.Errorf("UpstreamTotalTokens: got %d, want 150", s.UpstreamTotalTokens)
	}
	if s.DedupBytes != 200 {
		t.Errorf("DedupBytes: got %d, want 200", s.DedupBytes)
	}
	if s.ContextBytes != 200 {
		t.Errorf("ContextBytes: got %d, want 200", s.ContextBytes)
	}
	if s.CompressBytes != 100 {
		t.Errorf("CompressBytes: got %d, want 100", s.CompressBytes)
	}
	if s.ContextMessages != 5 {
		t.Errorf("ContextMessages: got %d, want 5", s.ContextMessages)
	}
	if s.BudgetTokens != 3072 {
		t.Errorf("BudgetTokens: got %d, want 3072", s.BudgetTokens)
	}
	if s.CacheSaveBytes != 500 {
		t.Errorf("CacheSaveBytes: got %d, want 500", s.CacheSaveBytes)
	}
}

func TestCollectorEmpty(t *testing.T) {
	c := NewCollector()
	s := c.Snapshot()

	if s.Requests != 0 {
		t.Errorf("Requests: got %d, want 0", s.Requests)
	}
	if s.CacheHitRatio() != 0 {
		t.Errorf("CacheHitRatio: got %f, want 0", s.CacheHitRatio())
	}
	if s.TotalSavingsBytes() != 0 {
		t.Errorf("TotalSavingsBytes: got %d, want 0", s.TotalSavingsBytes())
	}
}

func TestCacheHitRatio(t *testing.T) {
	c := NewCollector()
	c.Track(&RequestMetrics{CacheHit: true})
	c.Track(&RequestMetrics{CacheHit: false})
	c.Track(&RequestMetrics{CacheHit: false})

	s := c.Snapshot()
	if got := s.CacheHitRatio(); got != 1.0/3.0 {
		t.Errorf("CacheHitRatio: got %f, want %f", got, 1.0/3.0)
	}
}

func TestTotalSavingsBytes(t *testing.T) {
	c := NewCollector()
	c.Track(&RequestMetrics{
		BytesBefore:        1000,
		BytesAfterDedup:    700,
		BytesAfterContext:  500,
		BytesAfterCompress: 400,
	})

	s := c.Snapshot()
	if got := s.TotalSavingsBytes(); got != 600 {
		t.Errorf("TotalSavingsBytes: got %d, want 600", got)
	}
}

func TestFormatReport(t *testing.T) {
	c := NewCollector()
	for i := 0; i < 5; i++ {
		c.Track(&RequestMetrics{
			BytesBefore:        1000,
			BytesAfterDedup:    800,
			BytesAfterContext:  600,
			BytesAfterCompress: 500,
			MessagesBefore:     10,
			MessagesAfter:      5,
			MaxTokensBefore:    4096,
			MaxTokensAfter:     1024,
			UpstreamTotalTokens: 200,
		})
	}
	c.Track(&RequestMetrics{CacheHit: true, CacheType: "exact", BodyBytes: 300})
	c.Track(&RequestMetrics{CacheHit: true, CacheType: "semantic", BodyBytes: 200})

	s := c.Snapshot()
	report := s.FormatReport()

	if len(report) == 0 {
		t.Fatal("empty report")
	}
	if s.CacheMissBytes != 2500 {
		t.Errorf("CacheMissBytes: got %d, want 2500", s.CacheMissBytes)
	}
}
