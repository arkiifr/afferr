package tools

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/arkiifr/afferr/internal/config"
)

type Decision string
const ( DecisionAllow Decision = "allow"; DecisionAsk Decision = "ask"; DecisionDeny Decision = "deny" )

type PermissionError struct { Tool string; Decision Decision; Reason string; Args json.RawMessage }
func (e *PermissionError) Error() string { return fmt.Sprintf("%s permission for %s: %s", e.Decision, e.Tool, e.Reason) }

type PermissionManager struct { cfg config.PermissionConfig }
func NewPermissionManager(cfg config.PermissionConfig) *PermissionManager { return &PermissionManager{cfg: cfg} }

func (p *PermissionManager) Check(tool string, args map[string]any, mode string, approved bool) (Decision, string) {
	canonical := tool + ":" + canonicalArgs(args)
	for _, denied := range p.cfg.Deny { if permissionMatch(denied, tool, canonical) { return DecisionDeny, "matched deny rule " + denied } }
	if approved { return DecisionAllow, "approved by user" }
	if mode == "plan" && (tool == "write" || tool == "edit" || tool == "bash" || tool == "apply_patch" || (tool == "git" && args["action"] == "commit")) { return DecisionDeny, "plan mode is read-only" }
	if mode == "auto" { return DecisionAllow, "auto mode" }
	action := p.action(tool)
	switch strings.ToLower(action) { case "allow": return DecisionAllow, "configuration"; case "deny": return DecisionDeny, "configuration"; default: return DecisionAsk, "configuration requires approval" }
}

func (p *PermissionManager) action(tool string) string {
	switch tool {
	case "bash": return p.cfg.Bash
	case "edit": return p.cfg.Edit
	case "write": return p.cfg.Write
	case "webfetch", "websearch": return p.cfg.Webfetch
	case "read", "glob", "grep", "lsp", "git": return p.cfg.Read
	default: if p.cfg.Wildcard != "" { return p.cfg.Wildcard }; return "ask"
	}
}

func permissionMatch(pattern, tool, canonical string) bool {
	pattern = strings.TrimSpace(pattern); if pattern == "" { return false }
	if pattern == tool || pattern == "*" { return true }
	if ok, _ := filepath.Match(pattern, tool); ok { return true }
	if ok, _ := filepath.Match(pattern, canonical); ok { return true }
	// A human-friendly deny such as bash:*:--no-verify is commonly authored
	// with multiple colon-separated segments. Treat * as a non-greedy wildcard.
parts := strings.Split(pattern, "*"); if len(parts) > 1 { if strings.HasPrefix(pattern, tool+":") { for _, part := range parts { part = strings.Trim(part, ":"); if part != "" && !strings.Contains(canonical, part) { return false } }; return true }; pos := 0; for _, part := range parts { if part == "" { continue }; next := strings.Index(canonical[pos:], part); if next < 0 { return false }; pos += next + len(part) }; return true }
	if strings.Contains(pattern, ":") { segments := strings.Split(pattern, ":"); if segments[0] == tool { for _, segment := range segments[1:] { if segment != "" && strings.Contains(canonical, segment) { return true } } } }
	return strings.HasPrefix(canonical, pattern)
}

func canonicalArgs(args map[string]any) string { data, _ := json.Marshal(args); return string(data) }
