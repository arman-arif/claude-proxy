package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

type ToolCallAcc struct {
	ID    string
	Name  string
	Args  strings.Builder
}

type AnthropicStreamBuilder struct {
	ID         string
	Model      string
	Created    int64
	TextBuf    strings.Builder
	ToolCalls  []ToolCallAcc
	Usage      *OpenAIUsage
	SawContent bool
	SawTool    bool
	TextOpen   bool
	ToolOpen   bool
}

func NewStreamBuilder(model string) *AnthropicStreamBuilder {
	return &AnthropicStreamBuilder{
		ID:      "msg_" + randID(24),
		Model:   model,
		Created: time.Now().UnixMilli(),
	}
}

func (sb *AnthropicStreamBuilder) HandleOpenAIEvent(line string) (events []string, err error) {
	if !strings.HasPrefix(line, "data: ") {
		return nil, nil
	}

	data := strings.TrimPrefix(line, "data: ")
	if data == "[DONE]" {
		return sb.EmitFinish(), nil
	}

	var evt OpenAIStreamEvent
	if err := json.Unmarshal([]byte(data), &evt); err != nil {
		return nil, err
	}

	if len(evt.Choices) == 0 {
		return nil, nil
	}

	choice := evt.Choices[0]

	// first chunk → message_start
	if !sb.SawContent && !sb.SawTool {
		events = append(events, sb.EmitMessageStart(&evt))
	}

	// content delta
	if choice.Delta != nil {
		if choice.Delta.Content != "" {
			sb.TextBuf.WriteString(choice.Delta.Content)
			if !sb.TextOpen {
				events = append(events, sb.EmitContentBlockStart("text", 0))
				sb.TextOpen = true
			}
			events = append(events, sb.EmitContentDelta("text", choice.Delta.Content, 0))
			sb.SawContent = true
		}

		for _, tc := range choice.Delta.ToolCalls {
			idx := 0
			if tc.ID != "" {
				// new tool call
				sb.ToolCalls = append(sb.ToolCalls, ToolCallAcc{
					ID:   tc.ID,
					Name: tc.Function.Name,
				})
				idx = len(sb.ToolCalls) - 1
				if !sb.ToolOpen {
					events = append(events, sb.EmitContentBlockStart("tool_use", idx))
					sb.ToolOpen = true
				}
				events = append(events, sb.EmitToolDelta(tc.ID, tc.Function.Name, tc.Function.Arguments, idx))
				if tc.Function.Name != "" {
					sb.ToolCalls[idx].Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					sb.ToolCalls[idx].Args.WriteString(tc.Function.Arguments)
				}
				sb.SawTool = true
			} else if len(sb.ToolCalls) > 0 {
				// accumulate args by index
				if idx < len(sb.ToolCalls) {
					sb.ToolCalls[idx].Args.WriteString(tc.Function.Arguments)
					events = append(events, sb.EmitToolDelta(sb.ToolCalls[idx].ID, sb.ToolCalls[idx].Name, tc.Function.Arguments, idx))
				}
			}
		}
	}

	if choice.FinishReason != "" {
		// close open blocks before finish
		if sb.TextOpen {
			events = append(events, sb.EmitContentBlockStop(0))
			sb.TextOpen = false
		}
		if sb.ToolOpen && len(sb.ToolCalls) > 0 {
			events = append(events, sb.EmitContentBlockStop(len(sb.ToolCalls)-1))
			sb.ToolOpen = false
		}
		events = append(events, sb.EmitMessageDelta(&evt))
	}

	return events, nil
}

func (sb *AnthropicStreamBuilder) EmitMessageStart(evt *OpenAIStreamEvent) string {
	resp := AnthropicMessageResponse{
		ID:    sb.ID,
		Type:  "message",
		Role:  "assistant",
		Model: sb.Model,
	}
	return fmt.Sprintf(`event: message_start
data: %s

`, mustJSON(map[string]any{
		"type":    "message_start",
		"message": resp,
	}))
}

func (sb *AnthropicStreamBuilder) EmitContentBlockStart(typ string, idx int) string {
	cb := map[string]any{
		"type":  typ,
		"index": idx,
	}
	if typ == "text" {
		cb["text"] = ""
	} else {
		cb["id"] = ""
		cb["name"] = ""
		cb["input"] = map[string]any{}
	}
	return fmt.Sprintf(`event: content_block_start
data: %s

`, mustJSON(map[string]any{
		"type":          "content_block_start",
		"index":         idx,
		"content_block": cb,
	}))
}

func (sb *AnthropicStreamBuilder) EmitContentDelta(typ, text string, idx int) string {
	return fmt.Sprintf(`event: content_block_delta
data: %s

`, mustJSON(map[string]any{
		"type":  "content_block_delta",
		"index": idx,
		"delta": map[string]any{
			"type": "text_delta",
			"text": text,
		},
	}))
}

func (sb *AnthropicStreamBuilder) EmitContentBlockStop(idx int) string {
	return fmt.Sprintf(`event: content_block_stop
data: %s

`, mustJSON(map[string]any{
		"type":  "content_block_stop",
		"index": idx,
	}))
}

func (sb *AnthropicStreamBuilder) EmitToolDelta(id, name, partial string, idx int) string {
	return fmt.Sprintf(`event: content_block_delta
data: %s

`, mustJSON(map[string]any{
		"type":  "content_block_delta",
		"index": idx,
		"delta": map[string]any{
			"type":         "input_json_delta",
			"partial_json": partial,
		},
	}))
}

func (sb *AnthropicStreamBuilder) EmitMessageDelta(evt *OpenAIStreamEvent) string {
	finish := "end_turn"
	if len(evt.Choices) > 0 {
		finish = mapStopReason(evt.Choices[0].FinishReason)
	}

	return fmt.Sprintf(`event: message_delta
data: %s

`, mustJSON(map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   finish,
			"stop_sequence": nil,
		},
		"usage": map[string]any{
			"input_tokens":  0,
			"output_tokens": 0,
		},
	}))
}

func (sb *AnthropicStreamBuilder) EmitFinish() []string {
	var events []string

	finish := "end_turn"

	usage := map[string]any{
		"input_tokens":  0,
		"output_tokens": 0,
	}
	if sb.Usage != nil {
		usage["input_tokens"] = sb.Usage.PromptTokens
		usage["output_tokens"] = sb.Usage.CompletionTokens
	}

	events = append(events, fmt.Sprintf(`event: message_delta
data: %s

`, mustJSON(map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   finish,
			"stop_sequence": nil,
		},
		"usage": usage,
	})))

	events = append(events, fmt.Sprintf(`event: message_stop
data: %s

`, mustJSON(map[string]any{
		"type": "message_stop",
	})))

	return events
}

func ProcessOpenAISSE(body io.Reader, model string, flush func(string)) error {
	sb := NewStreamBuilder(model)
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		evts, err := sb.HandleOpenAIEvent(line)
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}
		for _, e := range evts {
			flush(e)
		}
	}

	return scanner.Err()
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func randID(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[(time.Now().UnixNano()+int64(i))%int64(len(chars))]
	}
	return string(b)
}
