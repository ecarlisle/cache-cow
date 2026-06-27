package router

import (
	"encoding/json"
	"testing"

	"github.com/ecarlisle/cache-cow/internal/types"
)

func newReq(model string, msgs []types.ChatMessage, tools json.RawMessage) *types.ChatRequest {
	return &types.ChatRequest{Model: model, Messages: msgs, Tools: tools}
}

func strPtr(s string) *string { return &s }

func msg(role, content string) types.ChatMessage {
	raw, _ := json.Marshal(content)
	return types.ChatMessage{Role: role, Content: raw}
}

func msgName(role, content, name string) types.ChatMessage {
	raw, _ := json.Marshal(content)
	return types.ChatMessage{Role: role, Content: raw, Name: &name}
}

func defaultCfg() RouterConfig {
	return RouterConfig{
		RouteToolCalls:        true,
		RouteLongContexts:     true,
		LongContextThreshold:  3000,
	}
}

func TestDecideNilOrEmpty(t *testing.T) {
	r := New(defaultCfg())
	if got := r.Decide(nil); got != types.RouteSimple {
		t.Errorf("nil request: got %v, want simple", got)
	}
	if got := r.Decide(&types.ChatRequest{Messages: nil}); got != types.RouteSimple {
		t.Errorf("nil messages: got %v, want simple", got)
	}
	if got := r.Decide(&types.ChatRequest{Messages: []types.ChatMessage{}}); got != types.RouteSimple {
		t.Errorf("empty messages: got %v, want simple", got)
	}
}

func TestDecideToolCalls(t *testing.T) {
	r := New(defaultCfg())

	t.Run("tools array present", func(t *testing.T) {
		req := newReq("test", []types.ChatMessage{msg("user", "hi")}, json.RawMessage(`[{"type":"function"}]`))
		if got := r.Decide(req); got != types.RouteComplex {
			t.Errorf("with tools: got %v, want complex", got)
		}
	})

	t.Run("tool role message present", func(t *testing.T) {
		req := newReq("test", []types.ChatMessage{
			msg("user", "check weather"),
			msg("tool", `{"temperature":72}`),
		}, nil)
		if got := r.Decide(req); got != types.RouteComplex {
			t.Errorf("with tool role: got %v, want complex", got)
		}
	})

	t.Run("no tools is simple", func(t *testing.T) {
		req := newReq("test", []types.ChatMessage{msg("user", "hi")}, nil)
		if got := r.Decide(req); got != types.RouteSimple {
			t.Errorf("no tools: got %v, want simple", got)
		}
	})

	t.Run("disabled route_tool_calls skips heuristic", func(t *testing.T) {
		r := New(RouterConfig{RouteToolCalls: false})
		req := newReq("test", []types.ChatMessage{msg("user", "hi")}, json.RawMessage(`[{"type":"function"}]`))
		if got := r.Decide(req); got != types.RouteSimple {
			t.Errorf("disabled tool routing: got %v, want simple", got)
		}
	})
}

func TestDecideLongMessages(t *testing.T) {
	r := New(defaultCfg())

	t.Run("message exceeds threshold", func(t *testing.T) {
		long := string(make([]byte, 3001))
		req := newReq("test", []types.ChatMessage{msg("user", long)}, nil)
		if got := r.Decide(req); got != types.RouteComplex {
			t.Errorf("long message: got %v, want complex", got)
		}
	})

	t.Run("message within threshold", func(t *testing.T) {
		short := string(make([]byte, 2000))
		for i := range short {
			short = short[:i] + "a" + short[i+1:]
		}
		req := newReq("test", []types.ChatMessage{msg("user", short)}, nil)
		if got := r.Decide(req); got != types.RouteSimple {
			t.Errorf("short message: got %v, want simple", got)
		}
	})

	t.Run("disabled long context skips heuristic", func(t *testing.T) {
		r := New(RouterConfig{RouteLongContexts: false, LongContextThreshold: 3000})
		long := string(make([]byte, 3001))
		req := newReq("test", []types.ChatMessage{msg("user", long)}, nil)
		if got := r.Decide(req); got != types.RouteSimple {
			t.Errorf("disabled long context: got %v, want simple", got)
		}
	})
}

func TestDecideManyMessages(t *testing.T) {
	r := New(defaultCfg())

	t.Run("more than 6 non-system messages", func(t *testing.T) {
		msgs := make([]types.ChatMessage, 7)
		for i := range msgs {
			msgs[i] = msg("user", "hello")
		}
		req := newReq("test", msgs, nil)
		if got := r.Decide(req); got != types.RouteComplex {
			t.Errorf("7 messages: got %v, want complex", got)
		}
	})

	t.Run("6 or fewer non-system messages is simple", func(t *testing.T) {
		msgs := make([]types.ChatMessage, 6)
		for i := range msgs {
			msgs[i] = msg("user", "hello")
		}
		req := newReq("test", msgs, nil)
		if got := r.Decide(req); got != types.RouteSimple {
			t.Errorf("6 messages: got %v, want simple", got)
		}
	})

	t.Run("system messages not counted", func(t *testing.T) {
		msgs := []types.ChatMessage{
			msg("system", "you are a helpful assistant"),
			// 7 user messages below threshold
			msg("user", "a"), msg("user", "b"), msg("user", "c"),
			msg("user", "d"), msg("user", "e"), msg("user", "f"),
			msg("user", "g"),
		}
		req := newReq("test", msgs, nil)
		if got := r.Decide(req); got != types.RouteComplex {
			t.Errorf("7 user + 1 system: got %v, want complex", got)
		}
	})
}

func TestDecideCodeGeneration(t *testing.T) {
	r := New(defaultCfg())

	tests := []struct {
		name    string
		content string
		want    types.RouteDecision
	}{
		{"short message", "hi", types.RouteSimple},
		{"code fence", "Please write a long python function for me that parses JSON:\n```\ndef parse_json(data):\n    import json\n    return json.loads(data)\n```\nMake sure it handles errors too.", types.RouteComplex},
		{"write a", "please write a python script that parses json and handles errors and also validates the input before processing it further", types.RouteComplex},
		{"create a", "create a rest api endpoint for users with authentication and database integration including all the necessary error handling", types.RouteComplex},
		{"implement", "implement binary search in go with proper error handling and edge case coverage including empty slice handling", types.RouteComplex},
		{"build a", "build a cli tool for managing todos with file persistence and colored output for a better user experience", types.RouteComplex},
		{"refactor", "refactor the user service to use clean architecture with proper dependency injection and unit tests for every method", types.RouteComplex},
		{"simple question under 100 chars", "what time is it", types.RouteSimple},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newReq("test", []types.ChatMessage{msg("user", tt.content)}, nil)
			if got := r.Decide(req); got != tt.want {
				t.Errorf("Decide(%q): got %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

func TestDecideComplexTakesPriority(t *testing.T) {
	r := New(defaultCfg())

	req := newReq("test", []types.ChatMessage{
		msg("user", "hi"),
		msg("tool", `{"result":"ok"}`),
	}, nil)
	if got := r.Decide(req); got != types.RouteComplex {
		t.Errorf("tool message should make complex early: got %v", got)
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		s       string
		substrs []string
		want    bool
	}{
		{"hello world", []string{"world"}, true},
		{"hello world", []string{"xyz"}, false},
		{"abc", []string{"abc", "def"}, true},
		{"", []string{"a"}, false},
		{"short", []string{"longer than short"}, false},
	}
	for _, tt := range tests {
		got := containsAny(tt.s, tt.substrs)
		if got != tt.want {
			t.Errorf("containsAny(%q, %v): got %v, want %v", tt.s, tt.substrs, got, tt.want)
		}
	}
}
