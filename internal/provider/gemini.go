package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GeminiClient struct { provider, baseURL, apiKey string; headers map[string]string; http *http.Client }
func NewGeminiClient(providerID, baseURL, apiKey string, headers map[string]string) *GeminiClient { return &GeminiClient{provider: providerID, baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, headers: headers, http: &http.Client{}} }
func (c *GeminiClient) Provider() string { return c.provider }

func (c *GeminiClient) Stream(ctx context.Context, req Request) (<-chan Event, error) {
	body, err := geminiRequestBody(req); if err != nil { return nil, err }
	model := url.PathEscape(req.Model); endpoint := c.baseURL + "/v1beta/models/" + model + ":streamGenerateContent?alt=sse"; if c.apiKey != "" { endpoint += "&key=" + url.QueryEscape(c.apiKey) }
	var resp *http.Response
	for attempt := 0; attempt < 5; attempt++ {
		hreq, e := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body)); if e != nil { return nil, e }; hreq.Header.Set("Content-Type", "application/json"); hreq.Header.Set("Accept", "text/event-stream"); for k, v := range c.headers { hreq.Header.Set(k, v) }
		resp, err = c.http.Do(hreq); if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 { break }
		if err != nil { if attempt == 4 { return nil, err } } else { data, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<10)); resp.Body.Close(); apiErr := &APIError{StatusCode: resp.StatusCode, Message: strings.TrimSpace(string(data))}; if !retryableStatus(resp.StatusCode) || attempt == 4 { return nil, apiErr }; err = apiErr }
		select { case <-ctx.Done(): return nil, ctx.Err(); case <-time.After(retryDelay(attempt, resp)): }
	}
	out := make(chan Event, 16); go func() { defer close(out); defer resp.Body.Close(); parseGeminiSSE(ctx, resp.Body, out) }(); return out, nil
}

func geminiRequestBody(req Request) ([]byte, error) {
	contents := make([]map[string]any, 0, len(req.Messages)); system := ""
	for _, msg := range req.Messages {
		if msg.Role == "system" { system += msg.Content + "\n"; continue }
		role := "user"; if msg.Role == "assistant" { role = "model" }
		parts := make([]map[string]any, 0, 1); if msg.Content != "" { parts = append(parts, map[string]any{"text": msg.Content}) }
		for _, call := range msg.ToolCalls { var args any; if json.Unmarshal([]byte(call.Arguments), &args) != nil { args = map[string]any{} }; parts = append(parts, map[string]any{"functionCall": map[string]any{"name": call.Name, "args": args}}) }
		if msg.Role == "tool" { parts = []map[string]any{{"functionResponse": map[string]any{"name": msg.Name, "response": map[string]any{"content": msg.Content}}}} }
		contents = append(contents, map[string]any{"role": role, "parts": parts})
	}
	payload := map[string]any{"contents": contents}
	if system != "" { payload["systemInstruction"] = map[string]any{"parts": []map[string]any{{"text": strings.TrimSpace(system)}}} }
	if len(req.Tools) > 0 { decls := make([]map[string]any, 0, len(req.Tools)); for _, t := range req.Tools { decls = append(decls, map[string]any{"name": t.Name, "description": t.Description, "parameters": json.RawMessage(t.Parameters)}) }; payload["tools"] = []map[string]any{{"functionDeclarations": decls}} }
	generation := map[string]any{}; if req.Temperature != nil { generation["temperature"] = *req.Temperature }; if req.MaxTokens > 0 { generation["maxOutputTokens"] = req.MaxTokens }; for k, v := range req.Variant { generation[k] = v }; if len(generation) > 0 { payload["generationConfig"] = generation }
	return json.Marshal(payload)
}

func parseGeminiSSE(ctx context.Context, reader io.Reader, out chan<- Event) {
	scanner := bufio.NewScanner(reader); scanner.Buffer(make([]byte, 64<<10), 2<<20)
	for scanner.Scan() {
		select { case <-ctx.Done(): out <- Event{Type: "error", Error: ctx.Err()}; return; default: }
		line := strings.TrimSpace(scanner.Text()); if strings.HasPrefix(line, "data:") { line = strings.TrimSpace(strings.TrimPrefix(line, "data:")) }; if line == "" { continue }
		var chunk struct { Candidates []struct { Content struct { Parts []struct { Text string `json:"text"`; FunctionCall *struct { Name string `json:"name"`; Args map[string]any `json:"args"` } `json:"functionCall"` } `json:"parts"` } `json:"content"`; Finish string `json:"finishReason"` } `json:"candidates"`; Usage struct { Prompt int `json:"promptTokenCount"`; Output int `json:"candidatesTokenCount"`; Total int `json:"totalTokenCount"` } `json:"usageMetadata"` }
		if err := json.Unmarshal([]byte(line), &chunk); err != nil { continue }
		if chunk.Usage.Total > 0 { out <- Event{Type: "usage", Usage: Usage{InputTokens: chunk.Usage.Prompt, OutputTokens: chunk.Usage.Output, TotalTokens: chunk.Usage.Total}} }
		for _, candidate := range chunk.Candidates { for _, part := range candidate.Content.Parts { if part.Text != "" { out <- Event{Type: "text", Text: part.Text} }; if part.FunctionCall != nil { args, _ := json.Marshal(part.FunctionCall.Args); out <- Event{Type: "tool_calls", ToolCalls: []ToolCall{{ID: part.FunctionCall.Name, Name: part.FunctionCall.Name, Arguments: string(args)}}} } }; if candidate.Finish != "" { out <- Event{Type: "finish", FinishReason: candidate.Finish} } }
	}
	if err := scanner.Err(); err != nil { out <- Event{Type: "error", Error: fmt.Errorf("gemini stream: %w", err)} }; out <- Event{Type: "done"}
}
