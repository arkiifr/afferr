package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/arkiifr/afferr/internal/provider"
)

type BashArgs struct { Command string `json:"command"`; Workdir string `json:"workdir"`; Timeout int `json:"timeout"` }
var blockedShell = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(^|[;&|]\s*)cd(?:\s|$)`),
	regexp.MustCompile(`(?i)rm\s+-[a-z]*r[a-z]*f[a-z]*\s+(/|\\)`),
	regexp.MustCompile(`(?i)(^|[;&|\s])mkfs(?:\s|$)`),
	regexp.MustCompile(`:\(\)\s*\{\s*:\|:\s*&\s*\}`),
}
func runBash(ctx context.Context, tc ToolContext, raw json.RawMessage) (Result, error) { var args BashArgs; if err := json.Unmarshal(raw, &args); err != nil { return Result{IsError: true, Content: err.Error()}, err }; if args.Workdir == "" { args.Workdir = tc.Cwd }; return RunBash(ctx, args) }

func RunBash(parent context.Context, args BashArgs) (Result, error) {
	command := strings.TrimSpace(args.Command); if command == "" { return Result{IsError: true, Content: "command is required"}, fmt.Errorf("command is required") }
	for _, pattern := range blockedShell { if pattern.MatchString(command) { err := fmt.Errorf("blocked command: unsafe shell pattern"); return Result{IsError: true, Content: err.Error()}, err } }
	if args.Timeout <= 0 || args.Timeout > 120000 { args.Timeout = 120000 }
	ctx, cancel := context.WithTimeout(parent, time.Duration(args.Timeout)*time.Millisecond); defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" { cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", command) } else { cmd = exec.CommandContext(ctx, "bash", "-lc", command) }
	if args.Workdir != "" { cmd.Dir = args.Workdir }
	output, err := cmd.CombinedOutput(); text := provider.Redact(string(output))
	if ctx.Err() != nil { err = fmt.Errorf("command timed out after %dms", args.Timeout) }
	if err != nil { return Result{IsError: true, Content: text + "\n" + provider.Redact(err.Error())}, err }
	return Result{Content: text}, nil
}
