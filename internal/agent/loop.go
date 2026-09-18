package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/arkiifr/afferr/internal/config"
	"github.com/arkiifr/afferr/internal/hooks"
	"github.com/arkiifr/afferr/internal/provider"
	"github.com/arkiifr/afferr/internal/store"
	"github.com/arkiifr/afferr/internal/tools"
	"golang.org/x/sync/errgroup"
)

type Session struct { ID string; Project string; Model string; Variant string; Mode string; Cwd string; History []provider.Message }
type Result struct { Text string; Model string; Steps int; Usage provider.Usage; Cost float64; Compacted bool }
type PermissionPrompt func(context.Context, tools.PermissionError) bool

type Runner struct { Config config.Config; Store *store.DB; Factory provider.Factory; Tools *tools.Registry; Hooks *hooks.Bus; OnText func(string); Approve PermissionPrompt; OnTool func(string, tools.Result) }

func NewRunner(cfg config.Config, db *store.DB, cwd string) *Runner { if cwd == "" { cwd, _ = os.Getwd() }; registry := tools.NewRegistry(cwd, cfg); bus := hooks.NewBus(); r := &Runner{Config: cfg, Store: db, Factory: provider.Factory{Config: cfg}, Tools: registry, Hooks: bus}; registry.SetAudit(func(tool, decision string) { if db != nil { _ = db.Audit(context.Background(), "", tool, decision, "") } }); registry.SetTaskHandler(func(ctx context.Context, prompt, model string, maxSteps int) (string, error) { childCfg := r.Config; if maxSteps > 0 && maxSteps < childCfg.MaxSteps { childCfg.MaxSteps = maxSteps }; child := NewRunner(childCfg, db, cwd); child.Approve = r.Approve; child.OnTool = r.OnTool; result, err := child.Run(ctx, Session{Model: model, Mode: "normal", Cwd: cwd, Project: cwd}, prompt); return result.Text, err }); return r }

// Run is the package-level convenience entry point requested by the public
// agent API. Applications that need event handlers should use NewRunner.
func Run(ctx context.Context, session Session, prompt string) (Result, error) { cwd := session.Cwd; if cwd == "" { cwd, _ = os.Getwd() }; cfg, err := config.Load(cwd); if err != nil { return Result{}, err }; db, err := store.Open(""); if err != nil { return Result{}, err }; defer db.Close(); return NewRunner(cfg, db, cwd).Run(ctx, session, prompt) }

func (r *Runner) Run(ctx context.Context, session Session, prompt string) (Result, error) {
	if r == nil { return Result{}, errors.New("nil agent runner") }; if session.Cwd == "" { session.Cwd, _ = os.Getwd() }; if session.Project == "" { session.Project = session.Cwd }
	requestedModel := session.Model
	model, err := ResolveModel(ctx, r.Config, session.Model, session.Project, r.Store); if err != nil { return Result{}, err }; session.Model = model
	if session.Mode == "" { session.Mode = r.Config.Permission.Mode }; if session.Mode == "" { session.Mode = "normal" }; r.Tools.SetMode(session.Mode)
	if session.ID == "" && r.Store != nil { created, createErr := r.Store.CreateSession(ctx, session.Project, firstLine(prompt), session.Model); if createErr != nil { return Result{}, createErr }; session.ID = created.ID }
	if len(session.History) == 0 && r.Store != nil && session.ID != "" { if saved, loadErr := r.Store.Messages(ctx, session.ID); loadErr == nil { session.History = fromStored(saved) } }
	if r.Hooks != nil { _ = r.Hooks.Publish(ctx, hooks.Event{Name: hooks.SessionStart, Session: session.ID, Data: map[string]any{"model": session.Model}}) }
	system := BuildSystemPrompt(session.Cwd, r.Config); messages := append([]provider.Message{{Role: "system", Content: system}}, session.History...)
	prompt = ExpandMentions(prompt, session.Cwd); if strings.TrimSpace(prompt) == "" { return Result{}, errors.New("prompt is empty") }
	if r.Store != nil && session.ID != "" { _ = r.Store.AppendMessage(ctx, session.ID, "user", prompt, estimateTokens(prompt)) }
	messages = append(messages, provider.Message{Role: "user", Content: prompt})
	if strings.HasPrefix(strings.TrimSpace(prompt), "!") { command := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(prompt), "!")); raw, _ := json.Marshal(map[string]any{"command": command, "workdir": session.Cwd}); result, toolErr := r.executeOne(ctx, session, provider.ToolCall{ID: "inline", Name: "bash", Arguments: string(raw)}); if toolErr == nil || result.Content != "" { messages = append(messages, provider.Message{Role: "tool", ToolCallID: "inline", Content: result.Content}) } }
	explicitModel := requestedModel != ""
	result := Result{Model: session.Model}; maxSteps := r.Config.MaxSteps; if maxSteps <= 0 { maxSteps = 128 }; models := modelCandidates(session.Model, r.Config.Fallback); if explicitModel { models = []string{session.Model} }; var lastErr error
	for step := 1; step <= maxSteps; step++ {
		result.Steps = step
		if r.Config.BudgetUSD > 0 && result.Cost >= r.Config.BudgetUSD { return result, fmt.Errorf("budgetUSD %.2f exceeded", r.Config.BudgetUSD) }
		messages, compacted := r.maybeCompact(ctx, session, messages, models[0]); result.Compacted = result.Compacted || compacted
		messages = trimMessages(messages, 128000)
		var text string; var calls []provider.ToolCall; var usage provider.Usage; var finish string; usedModel := session.Model
		for index, candidate := range models {
			client, clientErr := r.Factory.Client(candidate); if clientErr != nil { lastErr = clientErr; continue }
			request := provider.Request{Model: modelPart(candidate), Messages: messages, Tools: r.Tools.Definitions(), ToolChoice: "auto", Variant: VariantPayload(r.Config, candidate, session.Variant), MaxTokens: 32768}
			callText, callCalls, callUsage, callFinish, streamErr := r.stream(ctx, client, request)
			if streamErr != nil { lastErr = streamErr; if index+1 < len(models) { continue }; return result, streamErr }
			text, calls, usage, finish, usedModel = callText, callCalls, callUsage, callFinish, candidate; lastErr = nil; if candidate != session.Model { session.Model = candidate; result.Model = candidate; if r.Store != nil && session.ID != "" { _ = r.Store.SetSessionModel(ctx, session.ID, candidate) } }; break
		}
		if lastErr != nil && text == "" && len(calls) == 0 && usage.TotalTokens == 0 { return result, lastErr }
		result.Usage.InputTokens += usage.InputTokens; result.Usage.OutputTokens += usage.OutputTokens; result.Usage.TotalTokens += usage.TotalTokens; result.Cost += usageCost(usedModel, usage); if r.Store != nil && session.ID != "" { _ = r.Store.RecordUsage(ctx, usedModel, usage.InputTokens, usage.OutputTokens, usageCost(usedModel, usage)) }
		if len(calls) == 0 && text != "" { messages = append(messages, provider.Message{Role: "assistant", Content: text}); if r.Store != nil && session.ID != "" { _ = r.Store.AppendMessage(ctx, session.ID, "assistant", text, usage.OutputTokens) } }
		if len(calls) == 0 { if r.Hooks != nil { _ = r.Hooks.Publish(ctx, hooks.Event{Name: hooks.SessionIdle, Session: session.ID}) }; if finish == "length" { return result, fmt.Errorf("model stopped at output limit") }; result.Text = text; return result, nil }
		assistant := provider.Message{Role: "assistant", Content: text, ToolCalls: calls}; messages = append(messages, assistant)
		if r.Store != nil && session.ID != "" { encoded, _ := json.Marshal(calls); _ = r.Store.AppendMessage(ctx, session.ID, "assistant", string(encoded), usage.OutputTokens) }
		results := r.executeTools(ctx, session, calls)
		for i, call := range calls { toolResult := results[i]; messages = append(messages, provider.Message{Role: "tool", ToolCallID: call.ID, Name: call.Name, Content: toolResult.Content}); if r.Store != nil && session.ID != "" { encoded, _ := json.Marshal(call); _ = r.Store.AppendToolCall(ctx, store.StoredToolCall{ID: call.ID, Session: session.ID, Tool: call.Name, Args: string(encoded), Result: toolResult.Content}) }; if r.OnTool != nil { r.OnTool(call.Name, toolResult) } }
	}
	return result, fmt.Errorf("maximum agent steps (%d) reached", maxSteps)
}

func (r *Runner) stream(parent context.Context, client provider.Client, request provider.Request) (string, []provider.ToolCall, provider.Usage, string, error) { ctx, cancel := context.WithTimeout(parent, 120*time.Second); defer cancel(); stream, err := client.Stream(ctx, request); if err != nil { return "", nil, provider.Usage{}, "", err }; var text strings.Builder; calls := make([]provider.ToolCall, 0); seen := map[string]bool{}; usage := provider.Usage{}; finish := ""; for event := range stream { switch event.Type { case "text": text.WriteString(event.Text); if r.OnText != nil { r.OnText(event.Text) }; case "tool_calls": for _, call := range event.ToolCalls { key := call.ID + "\x00" + call.Name + "\x00" + call.Arguments; if !seen[key] { seen[key] = true; calls = append(calls, call) } }; case "usage": usage.InputTokens += event.Usage.InputTokens; usage.OutputTokens += event.Usage.OutputTokens; usage.TotalTokens += event.Usage.TotalTokens; case "finish": finish = event.FinishReason; case "error": if event.Error != nil { return text.String(), calls, usage, finish, event.Error } } }; return text.String(), calls, usage, finish, nil }

func (r *Runner) executeTools(ctx context.Context, session Session, calls []provider.ToolCall) []tools.Result { results := make([]tools.Result, len(calls)); if len(calls) == 0 { return results }; hasBash := false; for _, call := range calls { if call.Name == "bash" { hasBash = true } }; if hasBash { for i, call := range calls { results[i], _ = r.executeOne(ctx, session, call) }; return results }
	group, groupCtx := errgroup.WithContext(ctx); sem := make(chan struct{}, 8); var mu sync.Mutex
	for i, call := range calls { i, call := i, call; group.Go(func() error { select { case sem <- struct{}{}: case <-groupCtx.Done(): return groupCtx.Err() }; defer func() { <-sem }(); result, err := r.executeOne(groupCtx, session, call); mu.Lock(); results[i] = result; mu.Unlock(); return err }) }
	_ = group.Wait(); return results
}
func (r *Runner) executeOne(ctx context.Context, session Session, call provider.ToolCall) (tools.Result, error) { args := json.RawMessage(call.Arguments); if r.Hooks != nil { _ = r.Hooks.Publish(ctx, hooks.Event{Name: hooks.ToolPre, Session: session.ID, Tool: call.Name, Data: map[string]any{"args": string(args)}}) }; result, err := r.Tools.Execute(ctx, call.Name, args, false); var permission *tools.PermissionError; if errors.As(err, &permission) && permission.Decision == tools.DecisionAsk { if r.Hooks != nil { _ = r.Hooks.Publish(ctx, hooks.Event{Name: hooks.PermissionAsk, Session: session.ID, Tool: call.Name, Data: map[string]any{"reason": permission.Reason}}) }; approved := false; if r.Approve != nil { approved = r.Approve(ctx, *permission) }; result, err = r.Tools.Execute(ctx, call.Name, args, approved) }; if r.Hooks != nil { name := hooks.ToolPost; _ = r.Hooks.Publish(ctx, hooks.Event{Name: name, Session: session.ID, Tool: call.Name, Data: map[string]any{"error": errorString(err)}}) }; return result, err }

func ResolveModel(ctx context.Context, cfg config.Config, requested, project string, db *store.DB) (string, error) { if requested != "" { if _, _, err := provider.SplitModel(requested); err != nil { return "", err }; return requested, nil }; if cfg.Model != "" { if _, _, err := provider.SplitModel(cfg.Model); err != nil { return "", err }; return cfg.Model, nil }; if db != nil { if model, err := db.LastUsedModel(ctx, project); err == nil && model != "" { return model, nil } }; for _, candidate := range cfg.Fallback { if _, _, err := provider.SplitModel(candidate); err == nil { return candidate, nil } }; return "", errors.New("no model configured; pass --model provider/model or set model in .affer.jsonc") }
func modelCandidates(primary string, fallback []string) []string { out := []string{primary}; for _, value := range fallback { if value == primary { continue }; if _, _, err := provider.SplitModel(value); err == nil { out = append(out, value) } }; return out }
func modelPart(value string) string { _, model, err := provider.SplitModel(value); if err != nil { return value }; return model }
func trimMessages(messages []provider.Message, contextLimit int) []provider.Message { if len(messages) < 2 { return messages }; maxChars := contextLimit * 4 * 70 / 100; total := 0; for _, m := range messages { total += len(m.Content) }; if total <= maxChars { return messages }; out := []provider.Message{messages[0]}; kept := 0; for i := len(messages) - 1; i >= 1; i-- { size := len(messages[i].Content); if kept+size > maxChars-len(messages[0].Content) { break }; out = append(out, messages[i]); kept += size }; for i, j := 1, len(out)-1; i < j; i, j = i+1, j-1 { out[i], out[j] = out[j], out[i] }; return out }
func (r *Runner) maybeCompact(ctx context.Context, session Session, messages []provider.Message, model string) ([]provider.Message, bool) { total := 0; for _, m := range messages { total += len(m.Content) }; if total < 128000*4*80/100 || len(messages) < 8 { return messages, false }; client, err := r.Factory.Client(r.Config.Router.Compact); if err != nil { return messages, false }; summary, err := Compact(ctx, client, messages, modelPart(r.Config.Router.Compact)); if err != nil { return messages, false }; compacted, _ := json.Marshal(summary); kept := messages; if len(messages) > 5 { kept = messages[len(messages)-4:] }; out := []provider.Message{messages[0], provider.Message{Role: "assistant", Content: "Session summary: " + string(compacted)}}; out = append(out, kept...); if r.Hooks != nil { _ = r.Hooks.Publish(ctx, hooks.Event{Name: hooks.SessionCompact, Session: session.ID}) }; return out, true }
func fromStored(saved []store.StoredMessage) []provider.Message { out := make([]provider.Message, 0, len(saved)); for _, message := range saved { if message.Role == "system" { continue }; out = append(out, provider.Message{Role: message.Role, Content: message.Content}) }; return out }
func firstLine(value string) string { value = strings.TrimSpace(value); if index := strings.IndexByte(value, '\n'); index >= 0 { value = value[:index] }; if len(value) > 100 { value = value[:100] }; return value }
func estimateTokens(value string) int { if value == "" { return 0 }; return (len(value) + 3) / 4 }
func usageCost(model string, usage provider.Usage) float64 { _, full, _ := provider.SplitModel(model); _ = full; return float64(usage.InputTokens)*0.000003 + float64(usage.OutputTokens)*0.000015 }
func errorString(err error) string { if err == nil { return "" }; return err.Error() }
