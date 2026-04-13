package main

// === Anthropic API types (what clients send) ===

type AnthropicMessageRequest struct {
	Model         string               `json:"model"`
	MaxTokens     int                  `json:"max_tokens"`
	Messages      []AnthropicMessage   `json:"messages"`
	System        any                  `json:"system,omitempty"`
	Temperature   float64              `json:"temperature"`
	TopP          float64              `json:"top_p"`
	StopSequences []string             `json:"stop_sequences,omitempty"`
	Stream        bool                 `json:"stream"`
	Tools         []AnthropicTool      `json:"tools,omitempty"`
	ToolChoice    any                  `json:"tool_choice,omitempty"`
}

type AnthropicMessage struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type   string       `json:"type"`
	Text   string       `json:"text,omitempty"`
	Source *ImageSource `json:"source,omitempty"`
	ID     string       `json:"id,omitempty"`
	Name   string       `json:"name,omitempty"`
	Input  any          `json:"input,omitempty"`
	Output any          `json:"output,omitempty"`
}

type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type AnthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema"`
}

type AnthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// === Anthropic response (what clients expect) ===

type AnthropicMessageResponse struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Role         string            `json:"role"`
	Content      []ContentBlock    `json:"content"`
	Model        string            `json:"model"`
	StopReason   string            `json:"stop_reason"`
	StopSequence *string           `json:"stop_sequence"`
	Usage        *AnthropicUsage   `json:"usage"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	CacheRead    int `json:"cache_creation_input_tokens,omitempty"`
}

// === OpenAI API types (what we translate to/from) ===

type OpenAIChatRequest struct {
	Model            string          `json:"model"`
	Messages         []OpenAIMessage `json:"messages"`
	Temperature      float64         `json:"temperature"`
	MaxTokens        int             `json:"max_tokens"`
	TopP             float64         `json:"top_p"`
	Stream           bool            `json:"stream"`
	Stop             any             `json:"stop,omitempty"`
	Tools            []OpenAITool    `json:"tools,omitempty"`
	ToolChoice       any             `json:"tool_choice,omitempty"`
	ResponseFormat   *OpenAIRespFmt  `json:"response_format,omitempty"`
	FrequencyPenalty float64         `json:"frequency_penalty"`
	PresencePenalty  float64         `json:"presence_penalty"`
	Seed             *int            `json:"seed,omitempty"`
}

type OpenAIMessage struct {
	Role       string              `json:"role"`
	Content    any                 `json:"content"`
	Name       string              `json:"name,omitempty"`
	ToolCalls  []OpenAIToolCall    `json:"tool_calls,omitempty"`
	ToolCallID string              `json:"tool_call_id,omitempty"`
}

type OpenAIContentPart struct {
	Type     string            `json:"type"`
	Text     string            `json:"text,omitempty"`
	ImageURL *OpenAIImageURL   `json:"image_url,omitempty"`
}

type OpenAIImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type OpenAITool struct {
	Type     string         `json:"type"`
	Function *OpenAIFunction `json:"function,omitempty"`
}

type OpenAIFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"`
}

type OpenAIToolCall struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Function OpenAIToolFunc    `json:"function"`
}

type OpenAIToolFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type OpenAIRespFmt struct {
	Type string `json:"type"`
}

// OpenAI response

type OpenAIChatResponse struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *OpenAIUsage  `json:"usage,omitempty"`
}

type OpenAIChoice struct {
	Index        int             `json:"index"`
	Message      *OpenAIMessage  `json:"message,omitempty"`
	Delta        *OpenAIDelta    `json:"delta,omitempty"`
	FinishReason string          `json:"finish_reason"`
}

type OpenAIDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// SSE events from OpenAI

type OpenAIStreamEvent struct {
	ID      string         `json:"id,omitempty"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *OpenAIUsage   `json:"usage,omitempty"`
}
