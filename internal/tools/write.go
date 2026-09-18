package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type WriteArgs struct { Path string `json:"path"`; Content string `json:"content"` }
func runWrite(ctx context.Context, tc ToolContext, raw json.RawMessage) (Result, error) { var args WriteArgs; if err := json.Unmarshal(raw, &args); err != nil { return Result{IsError: true, Content: err.Error()}, err }; return WriteFile(ctx, tc.Cwd, tc.Tracker, args) }

func WriteFile(_ context.Context, cwd string, tracker *ReadTracker, args WriteArgs) (Result, error) {
	if strings.TrimSpace(args.Path) == "" { return Result{IsError: true, Content: "path is required"}, fmt.Errorf("path is required") }
	path := resolvePath(cwd, args.Path)
	if tracker == nil || !tracker.Has(path) { err := fmt.Errorf("read-before-write required for %s", args.Path); return Result{IsError: true, Content: err.Error()}, err }
	mode := os.FileMode(0o644); if info, err := os.Stat(path); err == nil { mode = info.Mode().Perm() }
	parent := filepath.Dir(path); if _, err := os.Stat(parent); os.IsNotExist(err) { if err := os.MkdirAll(parent, 0o755); err != nil { return Result{IsError: true, Content: err.Error()}, err } }
	tmp := path + ".affer.tmp"
	if err := os.WriteFile(tmp, []byte(args.Content), mode); err != nil { return Result{IsError: true, Content: err.Error()}, err }
	if err := os.Rename(tmp, path); err != nil { _ = os.Remove(tmp); return Result{IsError: true, Content: err.Error()}, err }
	return Result{Content: fmt.Sprintf("wrote %d bytes to %s", len(args.Content), args.Path)}, nil
}

type EditArgs struct { Path string `json:"path"`; OldString string `json:"oldString"`; NewString string `json:"newString"`; ReplaceAll bool `json:"replaceAll"` }
func runEdit(ctx context.Context, tc ToolContext, raw json.RawMessage) (Result, error) { var args EditArgs; if err := json.Unmarshal(raw, &args); err != nil { return Result{IsError: true, Content: err.Error()}, err }; return EditFile(ctx, tc.Cwd, tc.Tracker, args) }

func EditFile(_ context.Context, cwd string, tracker *ReadTracker, args EditArgs) (Result, error) {
	if args.Path == "" || args.OldString == "" { return Result{IsError: true, Content: "path and oldString are required"}, fmt.Errorf("path and oldString are required") }
	path := resolvePath(cwd, args.Path); if tracker == nil || !tracker.Has(path) { err := fmt.Errorf("read-before-edit required for %s", args.Path); return Result{IsError: true, Content: err.Error()}, err }
	data, err := os.ReadFile(path); if err != nil { return Result{IsError: true, Content: err.Error()}, err }; source := string(data); count := strings.Count(source, args.OldString)
	if count == 0 { err = fmt.Errorf("oldString was not found in %s", args.Path); return Result{IsError: true, Content: err.Error()}, err }
	if count > 1 && !args.ReplaceAll { err = fmt.Errorf("oldString matched %d times in %s; pass replaceAll=true", count, args.Path); return Result{IsError: true, Content: err.Error()}, err }
	replacement := args.NewString; if args.ReplaceAll { source = strings.ReplaceAll(source, args.OldString, replacement) } else { source = strings.Replace(source, args.OldString, replacement, 1) }
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil { return Result{IsError: true, Content: err.Error()}, err }
	if !strings.Contains(source, replacement) && replacement != "" { return Result{IsError: true, Content: "edit verification failed"}, fmt.Errorf("edit verification failed") }
	return Result{Content: fmt.Sprintf("edited %s (%d replacement%s)", args.Path, count, func() string { if count == 1 { return "" }; return "s" }())}, nil
}
