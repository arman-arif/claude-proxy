package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Anthropic → OpenAI request

func TransformToOpenAI(req *AnthropicMessageRequest, cfg *Config) (*OpenAIChatRequest, error) {
	openReq := &OpenAIChatRequest{
		Model:       pickModel(req.Model, cfg.DefaultModel),
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		Stop:        req.StopSequences,
	}

	if openReq.MaxTokens <= 0 {
		openReq.MaxTokens = 4096
	}
	// defaults for OpenAI compat
	if openReq.TopP <= 0 {
		openReq.TopP = 1
	}
	if openReq.Temperature <= 0 {
		openReq.Temperature = 1
	}
	// cap for non-streaming (OpenAI compat)
	if !openReq.Stream && openReq.MaxTokens > 8192 {
		openReq.MaxTokens = 8192
	}
	if openReq.Stream && openReq.MaxTokens > 65536 {
		openReq.MaxTokens = 65536
	}

	// System: prepend as system message
	var msgs []OpenAIMessage
	switch sys := req.System.(type) {
	case string:
		if sys != "" {
			msgs = append(msgs, OpenAIMessage{Role: "system", Content: sys})
		}
	case []any:
		for _, s := range sys {
			if m, ok := s.(map[string]any); ok {
				if t, _ := m["type"].(string); t == "text" {
					if txt, _ := m["text"].(string); txt != "" {
						msgs = append(msgs, OpenAIMessage{Role: "system", Content: txt})
					}
				}
			}
		}
	}

	// Messages
	for _, m := range req.Messages {
		om := OpenAIMessage{Role: m.Role}

		switch m.Role {
		case "assistant":
			if hasToolUse(m.Content.Blocks) {
				om.ToolCalls = extractToolCalls(m.Content.Blocks)
				if txt := extractText(m.Content.Blocks); txt != "" {
					om.Content = txt
				} else {
					om.Content = nil
				}
			} else {
				om.Content = buildContent(m.Content.Blocks)
			}
		case "user":
			om.Content = buildContent(m.Content.Blocks)
		default:
			om.Content = buildContent(m.Content.Blocks)
		}

		msgs = append(msgs, om)
	}

	openReq.Messages = msgs

	// Tools
	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			schema := t.InputSchema
			if schema == nil {
				schema = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			openReq.Tools = append(openReq.Tools, OpenAITool{
				Type: "function",
				Function: &OpenAIFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  schema,
				},
			})
		}
	}

	// Tool choice
	if req.ToolChoice != nil {
		openReq.ToolChoice = req.ToolChoice
	}

	return openReq, nil
}

func hasToolUse(blocks []ContentBlock) bool {
	for _, b := range blocks {
		if b.Type == "tool_use" {
			return true
		}
	}
	return false
}

func extractToolCalls(blocks []ContentBlock) []OpenAIToolCall {
	var calls []OpenAIToolCall
	for _, b := range blocks {
		if b.Type == "tool_use" {
			args := ""
			if b.Input != nil {
				if bs, err := json.Marshal(b.Input); err == nil {
					args = string(bs)
				} else {
					args = fmt.Sprintf("%v", b.Input)
				}
			}
			calls = append(calls, OpenAIToolCall{
				ID:   b.ID,
				Type: "function",
				Function: OpenAIToolFunc{
					Name:      b.Name,
					Arguments: args,
				},
			})
		}
	}
	return calls
}

func extractText(blocks []ContentBlock) string {
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func buildContent(blocks []ContentBlock) any {
	if len(blocks) == 0 {
		return ""
	}

	// single text → string
	if len(blocks) == 1 && blocks[0].Type == "text" {
		return blocks[0].Text
	}

	// multi → array
	var parts []OpenAIContentPart
	for _, b := range blocks {
		switch b.Type {
		case "text":
			parts = append(parts, OpenAIContentPart{Type: "text", Text: b.Text})
		case "image":
			if b.Source != nil {
				parts = append(parts, OpenAIContentPart{
					Type: "image_url",
					ImageURL: &OpenAIImageURL{
						URL: "data:" + b.Source.MediaType + ";base64," + b.Source.Data,
					},
				})
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return parts
}

func pickModel(model, defaultModel string) string {
	if model != "" {
		return model
	}
	return defaultModel
}

// OpenAI → Anthropic response

func TransformFromOpenAI(openResp *OpenAIChatResponse, reqModel string) *AnthropicMessageResponse {
	resp := &AnthropicMessageResponse{
		ID:      openResp.ID,
		Type:    "message",
		Role:    "assistant",
		Model:   reqModel,
		Content: []ContentBlock{},
	}

	if openResp.Usage != nil {
		resp.Usage = &AnthropicUsage{
			InputTokens:  openResp.Usage.PromptTokens,
			OutputTokens: openResp.Usage.CompletionTokens,
		}
	}

	if len(openResp.Choices) == 0 {
		return resp
	}

	choice := openResp.Choices[0]
	finish := mapStopReason(choice.FinishReason)
	resp.StopReason = finish

	// message path
	if choice.Message != nil {
		if choice.Message.Content != nil {
			if txt, ok := choice.Message.Content.(string); ok && txt != "" {
				resp.Content = append(resp.Content, ContentBlock{Type: "text", Text: txt})
			}
		}
		if len(choice.Message.ToolCalls) > 0 {
			for _, tc := range choice.Message.ToolCalls {
				var input any
				json.Unmarshal([]byte(tc.Function.Arguments), &input)
				resp.Content = append(resp.Content, ContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
		}
	}

	// delta path (streaming)
	if choice.Delta != nil {
		if choice.Delta.Content != "" {
			resp.Content = append(resp.Content, ContentBlock{Type: "text", Text: choice.Delta.Content})
		}
		if len(choice.Delta.ToolCalls) > 0 {
			for _, tc := range choice.Delta.ToolCalls {
				var input any
				json.Unmarshal([]byte(tc.Function.Arguments), &input)
				resp.Content = append(resp.Content, ContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
		}
	}

	if len(resp.Content) == 0 {
		resp.Content = append(resp.Content, ContentBlock{Type: "text", Text: ""})
	}

	return resp
}

func mapStopReason(reason string) string {
	switch reason {
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	case "stop":
		return "end_turn"
	default:
		return "end_turn"
	}
}

func nowUnix() int64 {
	return time.Now().Unix()
}
