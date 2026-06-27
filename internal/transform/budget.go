package transform

import "github.com/ecarlisle/cache-cow/internal/types"

type TokenBudget struct {
	cheapMax    int
	expensiveMax int
	enabled     bool
}

func NewTokenBudget(cheapMax, expensiveMax int, enabled bool) *TokenBudget {
	return &TokenBudget{
		cheapMax:     cheapMax,
		expensiveMax: expensiveMax,
		enabled:      enabled,
	}
}

func (b *TokenBudget) Apply(req *types.ChatRequest, route types.RouteDecision) *types.ChatRequest {
	if req == nil || !b.enabled {
		return req
	}

	limit := b.cheapMax
	if route == types.RouteComplex {
		limit = b.expensiveMax
	}

	if req.MaxTokens == nil || *req.MaxTokens > limit {
		clamped := limit
		req.MaxTokens = &clamped
	}

	return req
}
