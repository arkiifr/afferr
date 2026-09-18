package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type OpenAIClient struct {
	provider string
	baseURL  string
	apiKey   string
	headers  map[string]string
	http     *http.Client
}

func NewOpenAIClient(providerID, baseURL, apiKey string, headers map[string]string) *OpenAIClient {
	return &OpenAIClient{provider: providerID, baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, headers: headers, http: &http.Client{}}
}
func (c *OpenAIClient) Provider() string { return c.provider }

func (c *OpenAIClient) Stream(ctx context.Context, req Request) (<-chan Event, error) {
	if c.baseURL == "" { return nil, fmt.Errorf("provider %s has no baseURL", c.provider) }
	body, err := openAIRequestBody(req); if err != nil { return nil, err }
	resp, err := c.doWithRetry(ctx, body); if err != nil { return nil, err }
	out := make(chan Event, 16)
	go func() { defer close(out); defer resp.Body.Close(); parseOpenAISSE(ctx, resp.Body, out) }()
	return out, nil
}

func (c *OpenAIClient) doWithRetry(ctx context.Context, body []byte) (*http.Response, error) {
	url := c.baseURL
	if !strings.HasSuffix(url, "/chat/completions") { url += "/chat/completions" }
	var last error
	for attempt := 0; attempt < 5; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body)); if err != nil { return nil, err }
		req.Header.Set("Content-Type", "application/json"); req.Header.Set("Accept", "text/event-stream")
		if c.apiKey != "" { req.Header.Set("Authorization", "Bearer "+c.apiKey) }
		for k, v := range c.headers { req.Header.Set(k, v) }
		resp, err := c.http.Do(req)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 { return resp, nil }
		if err != nil { last = err } else {
			data, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<10)); resp.Body.Close()
			last = &APIError{StatusCode: resp.StatusCode, Message: strings.TrimSpace(string(data))}
			if !retryableStatus(resp.StatusCode) { return nil, last }
		}
		if attempt == 4 { break }
		delay := retryDelay(attempt, resp)
		select { case <-ctx.Done(): return nil, ctx.Err(); case <-time.After(delay): }
	}
	return nil, last
}

type APIError struct { StatusCode int; Message string }
func (e *APIError) Error() string { if e.Message == "" { return fmt.Sprintf("provider returned HTTP %d", e.StatusCode) }; return fmt.Sprintf("provider returned HTTP %d: %s", e.StatusCode, Redact(e.Message)) }
func retryableStatus(status int) bool { return status == http.StatusTooManyRequests || status == 529 || status >= 500 }
func retryDelay(attempt int, resp *http.Response) time.Duration {
	if resp != nil && resp.Header.Get("Retry-After") != "" { if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds > 0 { return time.Duration(seconds) * time.Second } }
	base := 200 * time.Millisecond * time.Duration(1<<attempt); jitter := time.Duration(rand.Int63n(int64(base / 2)))
	return base/2 + jitter
}

func openAIRequestBody(req Request) ([]byte, error) {
	messages := make([]map[string]any, 0, len(req.Messages))
	for _, msg := range req.Messages {
		m := map[string]any{"role": msg.Role}
		if msg.Content != "" || len(msg.ToolCalls) == 0 { m["content"] = msg.Content }
		if msg.Name != "" { m["name"] = msg.Name }
		if msg.ToolCallID != "" { m["tool_call_id"] = msg.ToolCallID }
		if len(msg.ToolCalls) > 0 {
			calls := make([]map[string]any, 0, len(msg.ToolCalls))
			for _, call := range msg.ToolCalls { calls = append(calls, map[string]any{"id": call.ID, "type": "function", "function": map[string]any{"name": call.Name, "arguments": call.Arguments}}) }
			m["tool_calls"] = calls
		}
		messages = append(messages, m)
	}
	payload := map[string]any{"model": req.Model, "messages": messages, "stream": true}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools)); for _, t := range req.Tools { tools = append(tools, map[string]any{"type": "function", "function": map[string]any{"name": t.Name, "description": t.Description, "parameters": json.RawMessage(t.Parameters)}}) }
		payload["tools"] = tools; if req.ToolChoice != nil { payload["tool_choice"] = req.ToolChoice }
	}
	if req.Temperature != nil { payload["temperature"] = *req.Temperature }
	if req.MaxTokens > 0 { payload["max_tokens"] = req.MaxTokens }
	for key, value := range req.Variant { payload[key] = value }
	return json.Marshal(payload)
}

func parseOpenAISSE(ctx context.Context, reader io.Reader, out chan<- Event) {
	scanner := bufio.NewScanner(reader); scanner.Buffer(make([]byte, 64<<10), 2<<20)
	var toolArgs = map[int]*ToolCall{}
	for scanner.Scan() {
		select { case <-ctx.Done(): out <- Event{Type: "error", Error: ctx.Err()}; return; default: }
		line := scanner.Text(); if !strings.HasPrefix(line, "data:") { continue }
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:")); if data == "" { continue }; if data == "[DONE]" { emitOpenAITools(toolArgs, out); out <- Event{Type: "done"}; return }
		var chunk struct { Choices []struct { Delta struct { Content string `json:"content"`; ToolCalls []struct { Index int `json:"index"`; ID string `json:"id"`; Function struct { Name string `json:"name"`; Arguments string `json:"arguments"` } `json:"function"` } `json:"tool_calls"` } `json:"delta"`; Finish string `json:"finish_reason"` } `json:"choices"`; Usage *struct { Prompt int `json:"prompt_tokens"`; Completion int `json:"completion_tokens"`; Total int `json:"total_tokens"` } `json:"usage"` }
		if json.Unmarshal([]byte(data), &chunk) != nil { continue }
		if chunk.Usage != nil { out <- Event{Type: "usage", Usage: Usage{InputTokens: chunk.Usage.Prompt, OutputTokens: chunk.Usage.Completion, TotalTokens: chunk.Usage.Total}} }
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" { out <- Event{Type: "text", Text: choice.Delta.Content} }
			for _, delta := range choice.Delta.ToolCalls { call := toolArgs[delta.Index]; if call == nil { call = &ToolCall{}; toolArgs[delta.Index] = call }; if delta.ID != "" { call.ID = delta.ID }; if delta.Function.Name != "" { call.Name = delta.Function.Name }; call.Arguments += delta.Function.Arguments }
			if choice.Finish != "" { emitOpenAITools(toolArgs, out); out <- Event{Type: "finish", FinishReason: choice.Finish} }
		}
	}
	if err := scanner.Err(); err != nil { out <- Event{Type: "error", Error: err} }; emitOpenAITools(toolArgs, out); out <- Event{Type: "done"}
}
func emitOpenAITools(values map[int]*ToolCall, out chan<- Event) { if len(values) == 0 { return }; calls := make([]ToolCall, 0, len(values)); for i := 0; i < len(values); i++ { if call := values[i]; call != nil { calls = append(calls, *call) } }; if len(calls) > 0 { out <- Event{Type: "tool_calls", ToolCalls: calls} } }
