package transform

import (
	"encoding/json"
	"testing"

	"github.com/ecarlisle/cache-cow/internal/types"
)

func msg(role, content string) types.ChatMessage {
	raw, _ := json.Marshal(content)
	return types.ChatMessage{Role: role, Content: raw}
}

func TestCompressNil(t *testing.T) {
	c := NewCompressor(100)
	if got := c.Compress(nil); got != nil {
		t.Errorf("nil request: got %v, want nil", got)
	}
}

func TestCompressWhitespaceNormalization(t *testing.T) {
	c := NewCompressor(100)

	req := &types.ChatRequest{
		Messages: []types.ChatMessage{msg("user", "hello\t\tworld\n\n\nhow are you?")},
	}

	out := c.Compress(req)
	if out == nil || len(out.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out.Messages))
	}

	var content string
	json.Unmarshal(out.Messages[0].Content, &content)

	if content != "hello  world\nhow are you?" {
		t.Errorf("got %q, want %q", content, "hello  world\nhow are you?")
	}
}

func TestCompressEmptyMessage(t *testing.T) {
	c := NewCompressor(100)

	req := &types.ChatRequest{
		Messages: []types.ChatMessage{
			msg("user", "hello"),
			msg("assistant", ""),
			msg("user", "world"),
		},
	}

	out := c.Compress(req)
	if len(out.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(out.Messages))
	}
}

func TestCompressWhitespaceOnly(t *testing.T) {
	c := NewCompressor(100)

	req := &types.ChatRequest{
		Messages: []types.ChatMessage{
			msg("user", "   \n  \n  "),
			msg("user", "real"),
		},
	}

	out := c.Compress(req)
	if len(out.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(out.Messages))
	}
}

func TestCompressToolTruncation(t *testing.T) {
	c := NewCompressor(20)

	longResult := "this is a very long tool result that should be truncated because it exceeds the threshold"
	req := &types.ChatRequest{
		Messages: []types.ChatMessage{
			msg("user", "what is the weather"),
			msg("tool", longResult),
		},
	}

	out := c.Compress(req)
	if len(out.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(out.Messages))
	}

	var content string
	json.Unmarshal(out.Messages[1].Content, &content)
	if len(content) < 25 || content[len(content)-11:] != "[truncated]" {
		t.Errorf("tool result not truncated: %q", content)
	}
}

func TestCompressStructuredContentSkipped(t *testing.T) {
	c := NewCompressor(100)

	structured := json.RawMessage(`{"key":"value"}`)
	req := &types.ChatRequest{
		Messages: []types.ChatMessage{
			{Role: "assistant", Content: structured},
		},
	}

	out := c.Compress(req)
	if len(out.Messages) != 1 {
		t.Errorf("expected 1 message (passthrough), got %d", len(out.Messages))
	}
}

func TestCompressNonToolNotTruncated(t *testing.T) {
	c := NewCompressor(10)

	long := "this is a long user message that exceeds the threshold but should not be truncated because it's not a tool role"
	req := &types.ChatRequest{
		Messages: []types.ChatMessage{msg("user", long)},
	}

	out := c.Compress(req)
	var content string
	json.Unmarshal(out.Messages[0].Content, &content)
	if content != "this is a long user message that exceeds the threshold but should not be truncated because it's not a tool role" {
		t.Errorf("user message was modified")
	}
}

func TestCompressPreservesFields(t *testing.T) {
	c := NewCompressor(100)
	temp := 0.7
	maxTokens := 100

	req := &types.ChatRequest{
		Model:       "gpt-4o-mini",
		Messages:    []types.ChatMessage{msg("user", "hello")},
		MaxTokens:   &maxTokens,
		Temperature: &temp,
	}

	out := c.Compress(req)
	if out.Model != "gpt-4o-mini" {
		t.Errorf("model changed: got %q", out.Model)
	}
	if *out.MaxTokens != 100 {
		t.Errorf("max tokens changed: got %d", *out.MaxTokens)
	}
	if *out.Temperature != 0.7 {
		t.Errorf("temperature changed: got %f", *out.Temperature)
	}
}
