package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/arkiifr/afferr/internal/provider"
)

func runLSPStub(_ context.Context, _ ToolContext, _ json.RawMessage) (Result, error) { err := fmt.Errorf("lsp is not configured; add lsp.<language> to .affer.jsonc"); return Result{IsError: true, Content: err.Error()}, err }

type WebFetchArgs struct { URL string `json:"url"`; Format string `json:"format"` }
func runWebFetch(ctx context.Context, _ ToolContext, raw json.RawMessage) (Result, error) { var args WebFetchArgs; if err := json.Unmarshal(raw, &args); err != nil { return Result{IsError: true, Content: err.Error()}, err }; if args.URL == "" { return Result{IsError: true, Content: "url is required"}, fmt.Errorf("url is required") }; req, err := http.NewRequestWithContext(ctx, http.MethodGet, args.URL, nil); if err != nil { return Result{IsError: true, Content: err.Error()}, err }; req.Header.Set("User-Agent", "affer/0.1 (+https://affer.dev)"); client := &http.Client{Timeout: 30 * time.Second}; resp, err := client.Do(req); if err != nil { return Result{IsError: true, Content: provider.Redact(err.Error())}, err }; defer resp.Body.Close(); body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20)); if err != nil { return Result{IsError: true, Content: err.Error()}, err }; text := string(body); if strings.Contains(resp.Header.Get("Content-Type"), "text/html") || args.Format == "markdown" { text = htmlToMarkdown(text) }; if resp.StatusCode >= 400 { err = fmt.Errorf("webfetch returned HTTP %d", resp.StatusCode); return Result{IsError: true, Content: text}, err }; return Result{Content: text}, nil }
func htmlToMarkdown(value string) string { replacements := []struct{ from, to string }{{"<br>", "\n"}, {"<br/>", "\n"}, {"</p>", "\n\n"}, {"</li>", "\n"}}; for _, item := range replacements { value = strings.ReplaceAll(value, item.from, item.to); value = strings.ReplaceAll(value, strings.ToUpper(item.from), item.to) }; for { start := strings.Index(value, "<"); if start < 0 { break }; end := strings.Index(value[start:], ">"); if end < 0 { break }; value = value[:start] + value[start+end+1:] }; return strings.TrimSpace(value) }

func runWebSearch(_ context.Context, _ ToolContext, _ json.RawMessage) (Result, error) { err := fmt.Errorf("websearch requires a configured search gateway (AFFER_WEBSEARCH_URL)"); return Result{IsError: true, Content: err.Error()}, err }
func runWebSearchStub(ctx context.Context, tc ToolContext, raw json.RawMessage) (Result, error) { return runWebSearch(ctx, tc, raw) }

type Todo struct { Content string `json:"content"`; Status string `json:"status"`; Priority string `json:"priority"` }
func runTodo(_ context.Context, _ ToolContext, raw json.RawMessage) (Result, error) { var value struct { Todos []Todo `json:"todos"` }; if err := json.Unmarshal(raw, &value); err != nil { return Result{IsError: true, Content: err.Error()}, err }; active := 0; for i := range value.Todos { if value.Todos[i].Status == "in_progress" { active++ }; if value.Todos[i].Status != "pending" && value.Todos[i].Status != "in_progress" && value.Todos[i].Status != "completed" { return Result{IsError: true, Content: "todo status must be pending, in_progress, or completed"}, fmt.Errorf("invalid todo status") } }; if active != 1 { return Result{IsError: true, Content: "exactly one todo must be in_progress"}, fmt.Errorf("expected exactly one in_progress todo") }; data, _ := json.Marshal(value.Todos); return Result{Content: string(data)}, nil }

func runQuestion(_ context.Context, _ ToolContext, raw json.RawMessage) (Result, error) { return Result{Content: "question requested: " + string(raw)}, nil }
func runTask(ctx context.Context, tc ToolContext, raw json.RawMessage) (Result, error) { var value struct { Prompt string `json:"prompt"`; Model string `json:"model"`; MaxSteps int `json:"maxSteps"` }; if err := json.Unmarshal(raw, &value); err != nil { return Result{IsError: true, Content: err.Error()}, err }; if strings.TrimSpace(value.Prompt) == "" { return Result{IsError: true, Content: "prompt is required"}, fmt.Errorf("prompt is required") }; if tc.Task == nil { err := fmt.Errorf("task runner is not configured"); return Result{IsError: true, Content: err.Error()}, err }; summary, err := tc.Task(ctx, value.Prompt, value.Model, value.MaxSteps); if err != nil { return Result{IsError: true, Content: err.Error()}, err }; return Result{Content: summary}, nil }

func runApplyPatch(ctx context.Context, tc ToolContext, raw json.RawMessage) (Result, error) { var value struct { Patch string `json:"patch"` }; if err := json.Unmarshal(raw, &value); err != nil { return Result{IsError: true, Content: err.Error()}, err }; if strings.TrimSpace(value.Patch) == "" { return Result{IsError: true, Content: "patch is required"}, fmt.Errorf("patch is required") }; // git apply reads stdin through a temporary file and keeps path handling in git.
	path := filepath.Join(tc.Cwd, ".affer-apply.patch"); if err := osWritePrivate(path, []byte(value.Patch)); err != nil { return Result{IsError: true, Content: err.Error()}, err }; defer removeFile(path); cmd := exec.CommandContext(ctx, "git", "apply", "--whitespace=nowarn", path); cmd.Dir = tc.Cwd; output, err := cmd.CombinedOutput(); text := provider.Redact(string(output)); if err != nil { return Result{IsError: true, Content: text + "\n" + err.Error()}, err }; return Result{Content: "patch applied\n" + text}, nil }

func runGit(ctx context.Context, tc ToolContext, raw json.RawMessage) (Result, error) { var value struct { Action string `json:"action"`; Path string `json:"path"`; Message string `json:"message"` }; if err := json.Unmarshal(raw, &value); err != nil { return Result{IsError: true, Content: err.Error()}, err }; var args []string; switch value.Action { case "status": args = []string{"status", "--short"}; case "diff": args = []string{"diff", "--", value.Path}; case "branch": args = []string{"branch", "--show-current"}; case "commit": if strings.TrimSpace(value.Message) == "" { return Result{IsError: true, Content: "commit message is required"}, fmt.Errorf("commit message is required") }; args = []string{"commit", "-m", value.Message}; case "pr": return Result{IsError: true, Content: "pr is intentionally explicit; use gh after reviewing the diff"}, fmt.Errorf("pr requires explicit gh invocation"); default: return Result{IsError: true, Content: "git action must be status, diff, branch, commit, or pr"}, fmt.Errorf("invalid git action") }; cmd := exec.CommandContext(ctx, "git", args...); cmd.Dir = tc.Cwd; output, err := cmd.CombinedOutput(); text := provider.Redact(string(output)); if err != nil { return Result{IsError: true, Content: text}, err }; return Result{Content: text}, nil }

// Small indirections keep misc.go's tool implementations easy to test without
// exposing temporary patch files as part of the public API.
var osWritePrivate = func(path string, data []byte) error { return writeFile0600(path, data) }
var removeFile = func(path string) { _ = remove(path) }
