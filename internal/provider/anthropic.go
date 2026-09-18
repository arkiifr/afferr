package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

type AnthropicClient struct { provider, baseURL, apiKey string; headers map[string]string; http *http.Client }
func NewAnthropicClient(providerID, baseURL, apiKey string, headers map[string]string) *AnthropicClient { return &AnthropicClient{provider: providerID, baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, headers: headers, http: &http.Client{}} }
func (c *AnthropicClient) Provider() string { return c.provider }

func (c *AnthropicClient) Stream(ctx context.Context, req Request) (<-chan Event, error) {
	body, err := anthropicRequestBody(req); if err != nil { return nil, err }
	resp, err := c.doWithRetry(ctx, body); if err != nil { return nil, err }
	out := make(chan Event, 16)
	go func() { defer close(out); defer resp.Body.Close(); parseAnthropicSSE(ctx, resp.Body, out) }()
	return out, nil
}

func (c *AnthropicClient) doWithRetry(ctx context.Context, body []byte) (*http.Response, error) {
	url := c.baseURL; if !strings.HasSuffix(url, "/messages") { url += "/v1/messages" }
	var last error
	for attempt := 0; attempt < 5; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body)); if err != nil { return nil, err }
		req.Header.Set("Content-Type", "application/json"); req.Header.Set("Accept", "text/event-stream"); req.Header.Set("anthropic-version", "2023-06-01")
		if c.apiKey != "" { req.Header.Set("x-api-key", c.apiKey) }
		for k, v := range c.headers { req.Header.Set(k, v) }
		resp, err := c.http.Do(req)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 { return resp, nil }
		if err != nil { last = err } else { data, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<10)); resp.Body.Close(); last = &APIError{StatusCode: resp.StatusCode, Message: strings.TrimSpace(string(data))}; if !retryableStatus(resp.StatusCode) { return nil, last } }
		if attempt == 4 { break }; select { case <-ctx.Done(): return nil, ctx.Err(); case <-time.After(retryDelay(attempt, resp)): }
	}
	return nil, last
}

func anthropicRequestBody(req Request) ([]byte, error) {
	messages := make([]map[string]any, 0, len(req.Messages)); system := ""
	for _, msg := range req.Messages {
		if msg.Role == "system" { system += msg.Content + "\n"; continue }
		m := map[string]any{"role": msg.Role}
		if msg.Role == "tool" { m["role"] = "user"; m["content"] = []map[string]any{{"type": "tool_result", "tool_use_id": msg.ToolCallID, "content": msg.Content}} } else if len(msg.ToolCalls) > 0 { blocks := make([]map[string]any, 0, len(msg.ToolCalls)); if msg.Content != "" { blocks = append(blocks, map[string]any{"type": "text", "text": msg.Content}) }; for _, call := range msg.ToolCalls { var input any; if json.Unmarshal([]byte(call.Arguments), &input) != nil { input = map[string]any{} }; blocks = append(blocks, map[string]any{"type": "tool_use", "id": call.ID, "name": call.Name, "input": input}) }; m["content"] = blocks } else { m["content"] = msg.Content }
		messages = append(messages, m)
	}
	payload := map[string]any{"model": req.Model, "messages": messages, "max_tokens": req.MaxTokens, "stream": true}
	if payload["max_tokens"] == 0 { payload["max_tokens"] = 32768 }; if system != "" { payload["system"] = strings.TrimSpace(system) }
	if len(req.Tools) > 0 { tools := make([]map[string]any, 0, len(req.Tools)); for _, t := range req.Tools { tools = append(tools, map[string]any{"name": t.Name, "description": t.Description, "input_schema": json.RawMessage(t.Parameters)}) }; payload["tools"] = tools }
	for key, value := range req.Variant { payload[key] = value }
	return json.Marshal(payload)
}

func parseAnthropicSSE(ctx context.Context, reader io.Reader, out chan<- Event) {
	scanner := bufio.NewScanner(reader); scanner.Buffer(make([]byte, 64<<10), 2<<20); eventName := ""; args := map[int]*ToolCall{}
	for scanner.Scan() {
		select { case <-ctx.Done(): out <- Event{Type: "error", Error: ctx.Err()}; return; default: }
		line := scanner.Text()
		if strings.HasPrefix(line, "event:") { eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:")); continue }
		if !strings.HasPrefix(line, "data:") { continue }; data := strings.TrimSpace(strings.TrimPrefix(line, "data:")); if data == "" { continue }
		var value map[string]any; if json.Unmarshal([]byte(data), &value) != nil { continue }; typ, _ := value["type"].(string); if typ == "" { typ = eventName }
		switch typ {
		case "message_start":
			if message, ok := value["message"].(map[string]any); ok { if usage, ok := message["usage"].(map[string]any); ok { out <- Event{Type: "usage", Usage: Usage{InputTokens: intNumber(usage["input_tokens"])}} } }
		case "content_block_start":
			index := intNumber(value["index"]); block, _ := value["content_block"].(map[string]any); if block != nil && block["type"] == "tool_use" { args[index] = &ToolCall{ID: stringValue(block["id"]), Name: stringValue(block["name"])} }
		case "content_block_delta":
			index := intNumber(value["index"]); delta, _ := value["delta"].(map[string]any); if delta == nil { continue }; kind := stringValue(delta["type"]); if kind == "text_delta" { out <- Event{Type: "text", Text: stringValue(delta["text"])} } else if kind == "input_json_delta" { if call := args[index]; call != nil { call.Arguments += stringValue(delta["partial_json"]) } }
		case "message_delta":
			if delta, ok := value["delta"].(map[string]any); ok { out <- Event{Type: "finish", FinishReason: stringValue(delta["stop_reason"])} }; if usage, ok := value["usage"].(map[string]any); ok { out <- Event{Type: "usage", Usage: Usage{OutputTokens: intNumber(usage["output_tokens"])}} }
		case "message_stop": emitAnthropicTools(args, out); out <- Event{Type: "done"}
		}
	}
	if err := scanner.Err(); err != nil { out <- Event{Type: "error", Error: err} }
	emitAnthropicTools(args, out); out <- Event{Type: "done"}
}
func emitAnthropicTools(values map[int]*ToolCall, out chan<- Event) { calls := make([]ToolCall, 0, len(values)); for i := 0; i < len(values); i++ { if call := values[i]; call != nil { calls = append(calls, *call) } }; if len(calls) > 0 { out <- Event{Type: "tool_calls", ToolCalls: calls} } }
func stringValue(v any) string { s, _ := v.(string); return s }
func intNumber(v any) int { switch n := v.(type) { case float64: return int(n); case int: return n; default: return 0 } }
