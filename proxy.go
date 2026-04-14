package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Proxy struct {
	cfg    *Config
	client *http.Client
	consoleLog *log.Logger
	fileLog    *log.Logger
}

func NewProxy(cfg *Config) *Proxy {
	p := &Proxy{
		cfg:    cfg,
		client: &http.Client{},
	}

	// console logger
	if cfg.ConsoleLogOn {
		p.consoleLog = log.New(os.Stdout, "", 0)
	}

	// file logger
	if cfg.LogEnabled && cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Printf("warning: cannot open log file %s: %v", cfg.LogFile, err)
		} else {
			p.fileLog = log.New(f, "", 0)
		}
	}
	return p
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS
	if r.Method == "OPTIONS" {
		setCORS(w)
		w.WriteHeader(200)
		return
	}

	setCORS(w)

	switch {
	case strings.HasSuffix(r.URL.Path, "/v1/messages"):
		if r.Method != "POST" {
			writeError(w, 405, "method not allowed")
			return
		}
		p.handleMessages(w, r)

	case strings.HasSuffix(r.URL.Path, "/v1/messages/count_tokens"):
		if r.Method != "POST" {
			writeError(w, 405, "method not allowed")
			return
		}
		p.handleCountTokens(w, r)

	default:
		http.NotFound(w, r)
	}
}

func (p *Proxy) handleMessages(w http.ResponseWriter, r *http.Request) {
	var req AnthropicMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid JSON: "+err.Error())
		return
	}

	openReq, err := TransformToOpenAI(&req, p.cfg)
	if err != nil {
		writeError(w, 400, "transform error: "+err.Error())
		return
	}

	if p.consoleLog != nil || p.fileLog != nil {
		tag := "HTTP"
		if req.Stream {
			tag = "SSE"
		}
		if p.consoleLog != nil {
			model := req.Model
			if model == "" {
				model = p.cfg.DefaultModel
			}
			p.consoleLog.Printf("[%s] [%s] %s %s → %s/v1/chat/completions (%s)", time.Now().Format("15:04:05"), tag, r.Method, r.URL.Path, p.cfg.OpenAIAPIURL, model)
		}
		if p.fileLog != nil {
			bodyBytes, _ := json.Marshal(openReq)
			p.fileLog.Printf("[%s] [%s] %s %s → %s\n%s", time.Now().Format(time.RFC3339), tag, r.Method, r.URL.Path, r.RemoteAddr, string(bodyBytes))
		}
	}

	bodyBytes, marshalErr := json.Marshal(openReq)
	if marshalErr != nil {
		writeError(w, 500, "marshal error: "+marshalErr.Error())
		return
	}

	openURL := p.cfg.OpenAIAPIURL + "/v1/chat/completions"

	httpReq, err := http.NewRequest("POST", openURL, bytes.NewReader(bodyBytes))
	if err != nil {
		writeError(w, 500, "request error: "+err.Error())
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.cfg.OpenAIAPIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		writeError(w, 502, "upstream error: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("openai error [%d]: %s", resp.StatusCode, string(body))
		writeError(w, resp.StatusCode, fmt.Sprintf("OpenAI API error: %s", string(body)))
		return
	}

	if req.Stream {
		p.streamResponse(w, resp.Body, req.Model)
	} else {
		p.nonStreamResponse(w, resp.Body, req.Model)
	}
}

func (p *Proxy) nonStreamResponse(w http.ResponseWriter, body io.Reader, reqModel string) {
	var openResp OpenAIChatResponse
	if err := json.NewDecoder(body).Decode(&openResp); err != nil {
		writeError(w, 500, "decode error: "+err.Error())
		return
	}

	anthResp := TransformFromOpenAI(&openResp, reqModel)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(anthResp)
}

func (p *Proxy) streamResponse(w http.ResponseWriter, body io.Reader, reqModel string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(200)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	err := ProcessOpenAISSE(body, reqModel, func(chunk string) {
		io.WriteString(w, chunk)
		flusher.Flush()
	})

	if err != nil {
		log.Printf("stream error: %v", err)
	}
}

func (p *Proxy) handleCountTokens(w http.ResponseWriter, r *http.Request) {
	// Stub: OpenAI has no count_tokens endpoint that matches this
	resp := map[string]int{"input_tokens": 0}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, x-api-key, anthropic-version")
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Request-Id", "")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"type":   "error",
		"error": map[string]any{
			"type":    "api_error",
			"message": msg,
		},
	})
}
