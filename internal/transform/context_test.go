package transform

import (
	"encoding/json"
	"testing"

	"github.com/ecarlisle/cache-cow/internal/types"
)

func TestTrimUnderLimit(t *testing.T) {
	cm := NewContextManager(10)

	msgs := []types.ChatMessage{
		msg("system", "you are helpful"),
		msg("user", "hello"),
		msg("assistant", "hi there"),
	}
	req := &types.ChatRequest{Messages: msgs}
	out := cm.Trim(req)

	if out == nil {
		t.Fatal("got nil")
	}
	if len(out.Messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(out.Messages))
	}
}

func TestTrimExactlyAtLimit(t *testing.T) {
	cm := NewContextManager(2)

	msgs := []types.ChatMessage{
		msg("system", "you are helpful"),
		msg("user", "a"),
		msg("assistant", "b"),
	}
	req := &types.ChatRequest{Messages: msgs}
	out := cm.Trim(req)

	if len(out.Messages) != 3 {
		t.Errorf("expected 3 (at limit), got %d", len(out.Messages))
	}
}

func TestTrimAboveLimit(t *testing.T) {
	cm := NewContextManager(4)

	msgs := []types.ChatMessage{
		msg("system", "you are helpful"),
		msg("user", "1"),
		msg("assistant", "2"),
		msg("user", "3"),
		msg("assistant", "4"),
		msg("user", "5"),
		msg("assistant", "6"),
	}
	req := &types.ChatRequest{Messages: msgs}
	out := cm.Trim(req)

	if len(out.Messages) > 6 {
		t.Errorf("expected ≤6 messages (4 turns + system + summary), got %d", len(out.Messages))
	}

	systemCount := 0
	summaryCount := 0
	userCount := 0
	for _, m := range out.Messages {
		switch m.Role {
		case "system":
			systemCount++
			var content string
			json.Unmarshal(m.Content, &content)
			if content != "you are helpful" {
				summaryCount++
			}
		case "user":
			userCount++
		}
	}
	if systemCount < 1 {
		t.Errorf("expected at least 1 system message")
	}
	if userCount < 1 {
		t.Errorf("expected at least 1 user message")
	}
	if summaryCount > 1 {
		t.Errorf("expected at most 1 summary, got %d", summaryCount)
	}
}

func TestTrimPreservesSystemMessages(t *testing.T) {
	cm := NewContextManager(2)

	msgs := []types.ChatMessage{
		msg("system", "rule 1"),
		msg("system", "rule 2"),
		msg("user", "a"),
		msg("assistant", "b"),
		msg("user", "c"),
		msg("assistant", "d"),
	}
	req := &types.ChatRequest{Messages: msgs}
	out := cm.Trim(req)

	systemCount := 0
	for _, m := range out.Messages {
		if m.Role == "system" {
			systemCount++
		}
	}
	if systemCount != 2 {
		t.Errorf("expected 2 system messages, got %d", systemCount)
	}
}

func TestTrimNil(t *testing.T) {
	cm := NewContextManager(10)
	if got := cm.Trim(nil); got != nil {
		t.Errorf("nil request: expected nil, got %v", got)
	}
}

func TestTrimVerySmallContentSkipped(t *testing.T) {
	cm := NewContextManager(2)

	msgs := []types.ChatMessage{
		msg("system", "you are helpful"),
		msg("user", "a"),
		msg("assistant", "b"),
		msg("user", "c"),
		msg("assistant", "d"),
		msg("user", "e"),
	}
	req := &types.ChatRequest{Messages: msgs}
	out := cm.Trim(req)

	foundSummary := false
	for _, m := range out.Messages {
		var content string
		json.Unmarshal(m.Content, &content)
		if len(content) > 10 && content[0] == '[' {
			foundSummary = true
			break
		}
	}
	if foundSummary {
		t.Errorf("expected no summary for small content")
	}
}

func TestTrimPreservesOtherFields(t *testing.T) {
	cm := NewContextManager(10)
	temp := 0.5
	toks := 200

	req := &types.ChatRequest{
		Model:       "gpt-4",
		Messages:    []types.ChatMessage{msg("user", "hi")},
		Temperature: &temp,
		MaxTokens:   &toks,
	}

	out := cm.Trim(req)
	if out.Model != "gpt-4" {
		t.Errorf("model changed")
	}
	if *out.MaxTokens != 200 {
		t.Errorf("max tokens changed")
	}
	if *out.Temperature != 0.5 {
		t.Errorf("temperature changed")
	}
}
