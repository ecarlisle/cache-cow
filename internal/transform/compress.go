package transform

import (
	"encoding/json"
	"strings"
	"unicode"

	"github.com/ecarlisle/cache-cow/internal/types"
)

type Compressor struct {
	TokenThreshold int
}

func NewCompressor(threshold int) *Compressor {
	return &Compressor{TokenThreshold: threshold}
}

func (c *Compressor) Compress(req *types.ChatRequest) *types.ChatRequest {
	if req == nil {
		return nil
	}

	compressed := make([]types.ChatMessage, 0, len(req.Messages))

	for _, msg := range req.Messages {
		if len(msg.Content) == 0 {
			continue
		}

		var contentStr string
		if err := json.Unmarshal(msg.Content, &contentStr); err != nil {
			compressed = append(compressed, msg)
			continue
		}

		cleaned := c.cleanContent(contentStr)

		if cleaned == "" {
			continue
		}

		if msg.Role == "tool" && len(cleaned) > c.TokenThreshold {
			truncated := cleaned[:c.TokenThreshold] + "\n...[truncated]"
			cleaned = truncated
		}

		newContent, _ := json.Marshal(cleaned)
		compressed = append(compressed, types.ChatMessage{
			Role:    msg.Role,
			Content: newContent,
			Name:    msg.Name,
		})
	}

	return &types.ChatRequest{
		Model:       req.Model,
		Messages:    compressed,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
		Tools:       req.Tools,
		ToolChoice:  req.ToolChoice,
	}
}

func (c *Compressor) cleanContent(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) && r != '\n' {
			return ' '
		}
		return r
	}, s)

	lines := strings.Split(s, "\n")
	var cleaned []string
	for _, line := range lines {
		stripped := strings.TrimSpace(line)
		if stripped != "" {
			cleaned = append(cleaned, stripped)
		}
	}

	result := strings.Join(cleaned, "\n")
	if result == "" {
		return ""
	}
	if len(result) > len(s)/3 {
		return result
	}

	return s
}
