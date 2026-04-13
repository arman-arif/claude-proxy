package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

type AnthropicStreamBuilder struct {
	ID         string
	Model      string
	Created    int64
	TextBuf    strings.Builder
	ToolCalls  []ContentBlock
	Usage      *OpenAIUsage
	ToolIdx    int
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
	if sb.Created == time.Now().UnixMilli() && len(sb.ToolCalls) == 0 && sb.TextBuf.Len() == 0 {
		events = append(events, sb.EmitMessageStart(&evt))
	}

	// content delta
	if choice.Delta != nil {
		if choice.Delta.Content != "" {
			sb.TextBuf.WriteString(choice.Delta.Content)
			events = append(events, sb.EmitContentBlockStart("text", 0))
			events = append(events, sb.EmitContentDelta("text", choice.Delta.Content, 0))
		}

		for _, tc := range choice.Delta.ToolCalls {
			if tc.ID != "" {
				var input any
				json.Unmarshal([]byte(tc.Function.Arguments), &input)
				block := ContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				}
				sb.ToolCalls = append(sb.ToolCalls, block)
				events = append(events, sb.EmitContentBlockStart("tool_use", sb.ToolIdx))
				if tc.Function.Name != "" {
					events = append(events, sb.EmitToolDelta(tc.ID, tc.Function.Name, "", sb.ToolIdx))
				}
				if tc.Function.Arguments != "" {
					events = append(events, sb.EmitToolDelta(tc.ID, tc.Function.Name, tc.Function.Arguments, sb.ToolIdx))
				}
				sb.ToolIdx++
			} else if len(sb.ToolCalls) > 0 {
				last := &sb.ToolCalls[len(sb.ToolCalls)-1]
				var currentArgs string
				if last.Input != nil {
					if bs, _ := json.Marshal(last.Input); bs != nil {
						currentArgs = string(bs)
					}
				}
				var newInput any
				fullArgs := currentArgs + tc.Function.Arguments
				json.Unmarshal([]byte(fullArgs), &newInput)
				last.Input = newInput
				events = append(events, sb.EmitToolDelta(last.ID, last.Name, tc.Function.Arguments, sb.ToolIdx-1))
			}
		}
	}

	if choice.FinishReason != "" {
		events = append(events, sb.EmitMessageDelta(&evt))
		if len(events) > 0 {
			// close last content block
			if sb.TextBuf.Len() > 0 || len(sb.ToolCalls) > 0 {
				idx := 0
				if len(sb.ToolCalls) > 0 {
					idx = sb.ToolIdx - 1
				}
				events = append(events, sb.EmitContentBlockStop(idx))
			}
		}
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
	return fmt.Sprintf(`event: content_block_start
data: %s

`, mustJSON(map[string]any{
		"type":  "content_block_start",
		"index": idx,
		"content_block": map[string]any{
			"type": typ,
			"text": "",
			"id":   "",
			"name": "",
			"input": map[string]any{},
		},
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
			"type":        "input_json_delta",
			"partial_json": partial,
		},
	}))
}

func (sb *AnthropicStreamBuilder) EmitMessageDelta(evt *OpenAIStreamEvent) string {
	finish := mapStopReason("")
	if len(evt.Choices) > 0 {
		finish = mapStopReason(evt.Choices[0].FinishReason)
	}

	// collect content and tool_use blocks
	var content []ContentBlock
	if sb.TextBuf.Len() > 0 {
		content = append(content, ContentBlock{Type: "text", Text: sb.TextBuf.String()})
	}
	content = append(content, sb.ToolCalls...)

	return fmt.Sprintf(`event: message_delta
data: %s

`, mustJSON(map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason": finish,
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

	// usage if available
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
