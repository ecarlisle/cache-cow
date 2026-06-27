package cache

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/ecarlisle/cache-cow/internal/types"
)

func strRef(s string) *string { return &s }

func tmpDB(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "toolcache-*.db")
	if err != nil {
		t.Fatalf("creating temp db: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestNewToolCache(t *testing.T) {
	path := tmpDB(t)
	defer os.Remove(path)

	tc, err := NewToolCache(path, 60)
	if err != nil {
		t.Fatalf("NewToolCache: %v", err)
	}
	defer tc.Close()
}

func TestToolSetAndGet(t *testing.T) {
	path := tmpDB(t)
	defer os.Remove(path)

	tc, err := NewToolCache(path, 60)
	if err != nil {
		t.Fatalf("NewToolCache: %v", err)
	}
	defer tc.Close()

	entry := &ToolCacheEntry{
		Response: json.RawMessage(`{"choices":[{"message":{"role":"assistant","content":"hello"}}]}`),
		Model:    "gpt-4o",
		CachedAt: time.Now().Unix(),
	}

	if err := tc.Set("test-key", entry); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := tc.Get("test-key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil, want entry")
	}
	if string(got.Response) != string(entry.Response) {
		t.Errorf("Response: got %s, want %s", got.Response, entry.Response)
	}
	if got.Model != entry.Model {
		t.Errorf("Model: got %s, want %s", got.Model, entry.Model)
	}
}

func TestToolGetMiss(t *testing.T) {
	path := tmpDB(t)
	defer os.Remove(path)

	tc, err := NewToolCache(path, 60)
	if err != nil {
		t.Fatalf("NewToolCache: %v", err)
	}
	defer tc.Close()

	got, err := tc.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Fatal("Get returned non-nil for missing key")
	}
}

func TestToolGetExpired(t *testing.T) {
	path := tmpDB(t)
	defer os.Remove(path)

	tc, err := NewToolCache(path, 1)
	if err != nil {
		t.Fatalf("NewToolCache: %v", err)
	}
	defer tc.Close()

	entry := &ToolCacheEntry{
		Response: json.RawMessage(`{}`),
		Model:    "gpt-4o",
		CachedAt: time.Now().Unix() - 10,
	}

	tc.Set("stale", entry)

	got, _ := tc.Get("stale")
	if got != nil {
		t.Fatal("Get should return nil for expired entry")
	}
}

func TestToolRequestKey(t *testing.T) {
	msg := types.ChatMessage{
		Role: "assistant",
		ToolCalls: []types.ToolCall{
			{Function: types.ToolCallFunction{Name: "read_file", Arguments: `{"path":"config.json"}`}},
		},
	}
	toolMsg := types.ChatMessage{
		Role:       "tool",
		Content:    json.RawMessage(`{"content":"result"}`),
		ToolCallID: strRef("call_1"),
	}

	key, ok := ToolRequestKey([]types.ChatMessage{msg, toolMsg})
	if !ok {
		t.Fatal("expected key, got false")
	}
	if len(key) != 64 {
		t.Errorf("expected 64-char hash, got %d: %s", len(key), key)
	}
}

func TestToolRequestKeyDeterministic(t *testing.T) {
	msgs := []types.ChatMessage{
		{
			Role: "assistant",
			ToolCalls: []types.ToolCall{
				{Function: types.ToolCallFunction{Name: "read_file", Arguments: `{"path":"config.json"}`}},
			},
		},
		{
			Role:       "tool",
			Content:    json.RawMessage(`{"content":"result"}`),
			ToolCallID: strRef("call_1"),
		},
	}

	k1, ok1 := ToolRequestKey(msgs)
	k2, ok2 := ToolRequestKey(msgs)

	if !ok1 || !ok2 {
		t.Fatal("both calls should produce a key")
	}
	if k1 != k2 {
		t.Errorf("keys differ:\n  %s\n  %s", k1, k2)
	}
}

func TestToolRequestKeyNoTools(t *testing.T) {
	msgs := []types.ChatMessage{
		{Role: "user", Content: json.RawMessage(`"hello"`)},
		{Role: "assistant", Content: json.RawMessage(`"hi"`)},
	}

	key, ok := ToolRequestKey(msgs)
	if ok {
		t.Fatalf("expected false, got key: %s", key)
	}
	if key != "" {
		t.Errorf("expected empty key, got %s", key)
	}
}

func TestToolRequestKeyMultipleCalls(t *testing.T) {
	msgs := []types.ChatMessage{
		{
			Role: "assistant",
			ToolCalls: []types.ToolCall{
				{Function: types.ToolCallFunction{Name: "read_file", Arguments: `{"path":"a.txt"}`}},
				{Function: types.ToolCallFunction{Name: "read_file", Arguments: `{"path":"b.txt"}`}},
			},
		},
		{
			Role:       "tool",
			Content:    json.RawMessage(`"content a"`),
			ToolCallID: strRef("call_1"),
		},
		{
			Role:       "tool",
			Content:    json.RawMessage(`"content b"`),
			ToolCallID: strRef("call_2"),
		},
	}

	key, ok := ToolRequestKey(msgs)
	if !ok {
		t.Fatal("expected key for multiple tool calls")
	}
	if key == "" {
		t.Fatal("non-empty key expected")
	}
}


