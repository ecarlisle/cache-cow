package cache

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ecarlisle/cache-cow/internal/types"
)

func newReq(model string, msgs []types.ChatMessage) *types.ChatRequest {
	return &types.ChatRequest{Model: model, Messages: msgs}
}

func msg(role, content string) types.ChatMessage {
	raw, _ := json.Marshal(content)
	return types.ChatMessage{Role: role, Content: raw}
}

func TestNewExactCache(t *testing.T) {
	c, err := NewExactCache(":memory:", 3600)
	if err != nil {
		t.Fatalf("NewExactCache: %v", err)
	}
	defer c.Close()
}

func TestSetAndGet(t *testing.T) {
	c, err := NewExactCache(":memory:", 3600)
	if err != nil {
		t.Fatalf("NewExactCache: %v", err)
	}
	defer c.Close()

	entry := &CacheEntry{
		Response: json.RawMessage(`{"id":"123","choices":[]}`),
		Model:    "gpt-4o-mini",
		CachedAt: time.Now().Unix(),
	}

	if err := c.Set("testhash", entry); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := c.Get("testhash")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Model != "gpt-4o-mini" {
		t.Errorf("model: got %q, want %q", got.Model, "gpt-4o-mini")
	}
	if string(got.Response) != `{"id":"123","choices":[]}` {
		t.Errorf("response: got %q", string(got.Response))
	}
}

func TestGetMiss(t *testing.T) {
	c, err := NewExactCache(":memory:", 3600)
	if err != nil {
		t.Fatalf("NewExactCache: %v", err)
	}
	defer c.Close()

	got, err := c.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for miss, got %v", got)
	}
}

func TestGetExpired(t *testing.T) {
	c, err := NewExactCache(":memory:", 0)
	if err != nil {
		t.Fatalf("NewExactCache: %v", err)
	}
	defer c.Close()

	entry := &CacheEntry{
		Response: json.RawMessage(`{}`),
		Model:    "gpt-4o",
		CachedAt: time.Now().Unix() - 100,
	}

	c.Set("expired", entry)

	got, err := c.Get("expired")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for expired entry, got %v", got)
	}
}

func TestRequestKey(t *testing.T) {
	req1 := newReq("gpt-4", []types.ChatMessage{msg("user", "hello")})
	req2 := newReq("gpt-4", []types.ChatMessage{msg("user", "hello")})
	req3 := newReq("gpt-4", []types.ChatMessage{msg("user", "world")})

	k1, err := RequestKey(req1)
	if err != nil {
		t.Fatalf("RequestKey: %v", err)
	}
	k2, _ := RequestKey(req2)
	k3, _ := RequestKey(req3)

	if k1 != k2 {
		t.Errorf("identical requests gave different keys: %s vs %s", k1, k2)
	}
	if k1 == k3 {
		t.Errorf("different requests gave same key: %s", k1)
	}
}

func TestRequestKeyIncludesModel(t *testing.T) {
	msgs := []types.ChatMessage{msg("user", "hello")}
	r1 := newReq("gpt-4", msgs)
	r2 := newReq("gpt-3.5", msgs)

	k1, _ := RequestKey(r1)
	k2, _ := RequestKey(r2)

	if k1 == k2 {
		t.Errorf("different models produced same key")
	}
}

func TestRequestKeyIncludesName(t *testing.T) {
	raw, _ := json.Marshal("hello")
	r1 := &types.ChatRequest{Model: "gpt-4", Messages: []types.ChatMessage{
		{Role: "user", Content: raw, Name: strPtr("alice")},
	}}
	r2 := &types.ChatRequest{Model: "gpt-4", Messages: []types.ChatMessage{
		{Role: "user", Content: raw, Name: strPtr("bob")},
	}}

	k1, _ := RequestKey(r1)
	k2, _ := RequestKey(r2)

	if k1 == k2 {
		t.Errorf("different names produced same key")
	}
}

func TestCloseIdempotent(t *testing.T) {
	c, err := NewExactCache(":memory:", 3600)
	if err != nil {
		t.Fatalf("NewExactCache: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("first close: %v", err)
	}
}

func strPtr(s string) *string { return &s }
