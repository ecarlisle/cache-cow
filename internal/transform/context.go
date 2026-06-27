package transform

import (
	"encoding/json"
	"fmt"

	"github.com/ecarlisle/cache-cow/internal/types"
)

type ContextManager struct {
	MaxTurns int
}

func NewContextManager(maxTurns int) *ContextManager {
	return &ContextManager{MaxTurns: maxTurns}
}

func (cm *ContextManager) Trim(req *types.ChatRequest) *types.ChatRequest {
	if req == nil || len(req.Messages) <= cm.MaxTurns+1 {
		return req
	}

	var system []types.ChatMessage
	var others []types.ChatMessage

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			system = append(system, msg)
		} else {
			others = append(others, msg)
		}
	}

	if len(others) <= cm.MaxTurns {
		return req
	}

	keepStart := cm.MaxTurns / 2
	keepEnd := cm.MaxTurns / 2

	var trimmed []types.ChatMessage
	trimmed = append(trimmed, system...)
	trimmed = append(trimmed, others[:keepStart]...)

	summarized := summarizeMessages(others[keepStart : len(others)-keepEnd])
	if summarized != nil {
		trimmed = append(trimmed, *summarized)
	}

	trimmed = append(trimmed, others[len(others)-keepEnd:]...)

	return &types.ChatRequest{
		Model:       req.Model,
		Messages:    trimmed,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
		Tools:       req.Tools,
		ToolChoice:  req.ToolChoice,
	}
}

func summarizeMessages(msgs []types.ChatMessage) *types.ChatMessage {
	if len(msgs) == 0 {
		return nil
	}

	totalLen := 0
	for _, m := range msgs {
		totalLen += len(m.Content)
	}

	if totalLen < 200 {
		return nil
	}

	summary := fmt.Sprintf("[summary of %d previous messages omitted to conserve tokens]", len(msgs))

	content, _ := json.Marshal(summary)
	return &types.ChatMessage{
		Role:    "system",
		Content: content,
	}
}
