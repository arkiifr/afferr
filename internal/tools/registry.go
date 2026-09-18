package tools

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arkiifr/afferr/internal/config"
	"github.com/arkiifr/afferr/internal/provider"
)

const maxToolResult = 20 * 1024

type Result struct {
	Content    string `json:"content"`
	IsError    bool   `json:"isError,omitempty"`
	Untrusted  bool   `json:"untrusted,omitempty"`
	OutputFile string `json:"outputFile,omitempty"`
}

type TaskHandler func(context.Context, string, string, int) (string, error)

type ToolContext struct {
	Cwd       string
	Mode      string
	Tracker   *ReadTracker
	SpillDir  string
	SessionID string
	Task      TaskHandler
}

type ToolFunc func(context.Context, ToolContext, json.RawMessage) (Result, error)

type Entry struct {
	Definition provider.ToolDefinition
	Run        ToolFunc
}

type Registry struct {
	entries map[string]Entry
	ctx     ToolContext
	perm    *PermissionManager
	audit   func(tool, decision string)
}

func NewRegistry(cwd string, cfg config.Config) *Registry {
	if cwd == "" { cwd, _ = os.Getwd() }
	perm := NewPermissionManager(cfg.Permission)
	mode := cfg.Permission.Mode; if mode == "" { mode = "normal" }; if cfg.AutoApprove { mode = "auto" }
	home, _ := os.UserHomeDir(); spill := filepath.Join(home, ".local", "share", "affer", "tool-output")
	r := &Registry{entries: map[string]Entry{}, ctx: ToolContext{Cwd: cwd, Mode: mode, Tracker: NewReadTracker(), SpillDir: spill}, perm: perm}
	r.registerBuiltins()
	return r
}
func (r *Registry) SetAudit(fn func(tool, decision string)) { r.audit = fn }
func (r *Registry) SetTaskHandler(fn TaskHandler) { r.ctx.Task = fn }
func (r *Registry) SetMode(mode string) { r.ctx.Mode = mode }
func (r *Registry) Add(entry Entry) { r.entries[entry.Definition.Name] = entry }
func (r *Registry) Has(name string) bool { _, ok := r.entries[name]; return ok }
func (r *Registry) Definitions() []provider.ToolDefinition { out := make([]provider.ToolDefinition, 0, len(r.entries)); for _, entry := range r.entries { out = append(out, entry.Definition) }; sortDefinitions(out); return out }

func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage, approved bool) (Result, error) {
	entry, ok := r.entries[name]; if !ok { return Result{IsError: true, Content: "unknown tool: " + name}, fmt.Errorf("unknown tool %q", name) }
	if len(args) == 0 { args = json.RawMessage(`{}`) }
	var object map[string]any; if err := json.Unmarshal(args, &object); err != nil || object == nil { return Result{IsError: true, Content: "arguments must be a JSON object"}, fmt.Errorf("%s arguments must be a JSON object", name) }
	decision, reason := r.perm.Check(name, object, r.ctx.Mode, approved)
	if r.audit != nil { r.audit(name, decision) }
	if decision == DecisionDeny { return Result{IsError: true, Content: "permission denied: " + reason}, &PermissionError{Tool: name, Decision: decision, Reason: reason, Args: args} }
	if decision == DecisionAsk { return Result{IsError: true, Content: "permission required: " + reason}, &PermissionError{Tool: name, Decision: decision, Reason: reason, Args: args} }
	result, err := entry.Run(ctx, r.ctx, args)
	if len(result.Content) > maxToolResult { result = r.spill(result) }
	return result, err
}

func (r *Registry) spill(result Result) Result {
	if err := os.MkdirAll(r.ctx.SpillDir, 0o700); err != nil { result.Content = result.Content[:maxToolResult] + "\n[output truncated]"; return result }
	buf := make([]byte, 8); _, _ = rand.Read(buf); id := hex.EncodeToString(buf)
	path := filepath.Join(r.ctx.SpillDir, id+".log")
	if err := os.WriteFile(path, []byte(result.Content), 0o600); err != nil { result.Content = result.Content[:maxToolResult] + "\n[output truncated]"; return result }
	result.OutputFile = path; result.Content = result.Content[:maxToolResult] + fmt.Sprintf("\n[output truncated; full output: %s]", path); return result
}

func (r *Registry) registerBuiltins() {
	r.Add(Entry{Definition: provider.ToolDefinition{Name: "read", Description: "Read a text file or describe a supported attachment.", Parameters: rawSchema(`{"type":"object","required":["path"],"properties":{"path":{"type":"string"},"offset":{"type":"integer","minimum":1},"limit":{"type":"integer","minimum":1,"maximum":2000}}}`)}, Run: runRead})
	r.Add(Entry{Definition: provider.ToolDefinition{Name: "write", Description: "Create or replace a file after it has been read in this turn.", Parameters: rawSchema(`{"type":"object","required":["path","content"],"properties":{"path":{"type":"string"},"content":{"type":"string"}}}`)}, Run: runWrite})
	r.Add(Entry{Definition: provider.ToolDefinition{Name: "edit", Description: "Replace an exact string in a file and verify the result.", Parameters: rawSchema(`{"type":"object","required":["path","oldString","newString"],"properties":{"path":{"type":"string"},"oldString":{"type":"string"},"newString":{"type":"string"},"replaceAll":{"type":"boolean"}}}`)}, Run: runEdit})
	r.Add(Entry{Definition: provider.ToolDefinition{Name: "bash", Description: "Run a bounded shell command in the project directory.", Parameters: rawSchema(`{"type":"object","required":["command"],"properties":{"command":{"type":"string"},"workdir":{"type":"string"},"timeout":{"type":"integer","minimum":1,"maximum":120000}}}`)}, Run: runBash})
	r.Add(Entry{Definition: provider.ToolDefinition{Name: "glob", Description: "Find files matching a glob pattern.", Parameters: rawSchema(`{"type":"object","required":["pattern"],"properties":{"pattern":{"type":"string"},"path":{"type":"string"}}}`)}, Run: runGlob})
	r.Add(Entry{Definition: provider.ToolDefinition{Name: "grep", Description: "Search project files for a regular expression.", Parameters: rawSchema(`{"type":"object","required":["pattern"],"properties":{"pattern":{"type":"string"},"include":{"type":"string"},"path":{"type":"string"}}}`)}, Run: runGrep})
	r.Add(Entry{Definition: provider.ToolDefinition{Name: "lsp", Description: "Query configured language-server diagnostics or symbols.", Parameters: rawSchema(`{"type":"object","required":["action"],"properties":{"action":{"type":"string","enum":["diagnostics","hover","definition","references","rename"]},"file":{"type":"string"},"line":{"type":"integer"},"symbol":{"type":"string"}}}`)}, Run: runLSPStub})
	r.Add(Entry{Definition: provider.ToolDefinition{Name: "webfetch", Description: "Fetch a URL as markdown.", Parameters: rawSchema(`{"type":"object","required":["url"],"properties":{"url":{"type":"string"},"format":{"type":"string"}}}`)}, Run: runWebFetch})
	r.Add(Entry{Definition: provider.ToolDefinition{Name: "websearch", Description: "Search the web through the configured gateway.", Parameters: rawSchema(`{"type":"object","required":["query"],"properties":{"query":{"type":"string"},"num":{"type":"integer","minimum":1,"maximum":20}}}`)}, Run: runWebSearchStub})
	r.Add(Entry{Definition: provider.ToolDefinition{Name: "todowrite", Description: "Update the session todo list; exactly one item may be in progress.", Parameters: rawSchema(`{"type":"object","required":["todos"],"properties":{"todos":{"type":"array"}}}`)}, Run: runTodo})
	r.Add(Entry{Definition: provider.ToolDefinition{Name: "question", Description: "Ask the user a structured question.", Parameters: rawSchema(`{"type":"object","required":["questions"],"properties":{"questions":{"type":"array"}}}`)}, Run: runQuestion})
	r.Add(Entry{Definition: provider.ToolDefinition{Name: "apply_patch", Description: "Apply a unified diff to the project.", Parameters: rawSchema(`{"type":"object","required":["patch"],"properties":{"patch":{"type":"string"}}}`)}, Run: runApplyPatch})
	r.Add(Entry{Definition: provider.ToolDefinition{Name: "git", Description: "Run a safe git status or diff query.", Parameters: rawSchema(`{"type":"object","required":["action"],"properties":{"action":{"type":"string","enum":["status","diff","branch"]},"path":{"type":"string"}}}`)}, Run: runGit})
	r.Add(Entry{Definition: provider.ToolDefinition{Name: "task", Description: "Run a bounded child coding agent and return only its summary.", Parameters: rawSchema(`{"type":"object","required":["prompt"],"properties":{"prompt":{"type":"string"},"model":{"type":"string"},"maxSteps":{"type":"integer","minimum":1,"maximum":32}}}`)}, Run: runTask})
}

func rawSchema(value string) []byte { return []byte(value) }
func sortDefinitions(values []provider.ToolDefinition) { for i := 1; i < len(values); i++ { for j := i; j > 0 && values[j].Name < values[j-1].Name; j-- { values[j], values[j-1] = values[j-1], values[j] } } }

func requiredString(object map[string]any, key string) (string, error) { value, ok := object[key].(string); if !ok || strings.TrimSpace(value) == "" { return "", fmt.Errorf("%s is required", key) }; return value, nil }
func decodeObject(args json.RawMessage) (map[string]any, error) { var object map[string]any; if err := json.Unmarshal(args, &object); err != nil || object == nil { return nil, errors.New("arguments must be a JSON object") }; return object, nil }

// Keep time imported in this file for plugin/tool implementations that attach
// an audit timestamp through the registry in future versions.
var _ = time.Now
