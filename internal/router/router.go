package router

import (
	"github.com/ecarlisle/cache-cow/internal/types"
)

type Router struct {
	config RouterConfig
}

type RouterConfig struct {
	RouteToolCalls     bool
	RouteLongContexts  bool
	LongContextThreshold int
}

func New(cfg RouterConfig) *Router {
	return &Router{config: cfg}
}

func (r *Router) Decide(req *types.ChatRequest) types.RouteDecision {
	if req == nil || len(req.Messages) == 0 {
		return types.RouteSimple
	}

	if r.config.RouteToolCalls && hasToolCalls(req) {
		return types.RouteComplex
	}

	if r.config.RouteLongContexts && hasLongMessages(req, r.config.LongContextThreshold) {
		return types.RouteComplex
	}

	if messageCountExceeds(req, 6) {
		return types.RouteComplex
	}

	if hasCodeGenerationIndicators(req) {
		return types.RouteComplex
	}

	return types.RouteSimple
}

func hasToolCalls(req *types.ChatRequest) bool {
	if len(req.Tools) > 0 {
		return true
	}
	for _, msg := range req.Messages {
		if msg.Role == "tool" {
			return true
		}
	}
	return false
}

func hasLongMessages(req *types.ChatRequest, threshold int) bool {
	for _, msg := range req.Messages {
		if len(msg.Content) > threshold {
			return true
		}
	}
	return false
}

func messageCountExceeds(req *types.ChatRequest, n int) bool {
	nonSystem := 0
	for _, msg := range req.Messages {
		if msg.Role != "system" {
			nonSystem++
		}
	}
	return nonSystem > n
}

func hasCodeGenerationIndicators(req *types.ChatRequest) bool {
	for _, msg := range req.Messages {
		if msg.Role != "user" {
			continue
		}
		content := string(msg.Content)
		if len(content) > 100 &&
			(containsAny(content, []string{"```", "function", "class ", "impl ", "def "}) ||
			 containsAny(content, []string{"write a", "create a", "implement", "build a", "refactor"})) {
			return true
		}
	}
	return false
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if len(sub) <= len(s) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
