package transform

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ecarlisle/cache-cow/internal/types"
)

type SystemPromptDeduper struct {
	mu       sync.RWMutex
	cache    map[string]string
	maxSize  int
	enabled  bool
	hitCount int
}

func NewSystemPromptDeduper(maxSize int, enabled bool) *SystemPromptDeduper {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &SystemPromptDeduper{
		cache:   make(map[string]string),
		maxSize: maxSize,
		enabled: enabled,
	}
}

func (d *SystemPromptDeduper) Dedupe(req *types.ChatRequest) (*types.ChatRequest, bool) {
	if req == nil || !d.enabled {
		return req, false
	}

	hasSystem := false
	var systemIdx int
	for i, msg := range req.Messages {
		if msg.Role == "system" {
			hasSystem = true
			systemIdx = i
			break
		}
	}
	if !hasSystem {
		return req, false
	}

	systemMsg := req.Messages[systemIdx]
	var content string
	if err := json.Unmarshal(systemMsg.Content, &content); err != nil {
		return req, false
	}

	h := sha256.New()
	h.Write([]byte(content))
	hash := fmt.Sprintf("%x", h.Sum(nil))[:16]

	d.mu.RLock()
	cached, seen := d.cache[hash]
	d.mu.RUnlock()

	if seen && cached != content {
		seen = false
	}

	if !seen {
		d.mu.Lock()
		if len(d.cache) >= d.maxSize {
			for k := range d.cache {
				delete(d.cache, k)
				break
			}
		}
		d.cache[hash] = content
		d.mu.Unlock()
		return req, false
	}

	d.mu.Lock()
	d.hitCount++
	d.mu.Unlock()

	replacement := fmt.Sprintf("[system prompt cached: %s]", hash)
	replacementContent, _ := json.Marshal(replacement)

	patched := make([]types.ChatMessage, len(req.Messages))
	copy(patched, req.Messages)
	patched[systemIdx] = types.ChatMessage{
		Role:    "system",
		Content: replacementContent,
	}

	return &types.ChatRequest{
		Model:       req.Model,
		Messages:    patched,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
		Tools:       req.Tools,
		ToolChoice:  req.ToolChoice,
	}, true
}

func (d *SystemPromptDeduper) HitCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.hitCount
}
