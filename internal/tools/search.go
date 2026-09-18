package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type GlobArgs struct { Pattern string `json:"pattern"`; Path string `json:"path"` }
func runGlob(_ context.Context, tc ToolContext, raw json.RawMessage) (Result, error) { var args GlobArgs; if err := json.Unmarshal(raw, &args); err != nil { return Result{IsError: true, Content: err.Error()}, err }; root := tc.Cwd; if args.Path != "" { root = resolvePath(tc.Cwd, args.Path) }; matches, err := Glob(root, args.Pattern); if err != nil { return Result{IsError: true, Content: err.Error()}, err }; return Result{Content: strings.Join(matches, "\n")}, nil }

func Glob(root, pattern string) ([]string, error) {
	if strings.TrimSpace(pattern) == "" { return nil, fmt.Errorf("pattern is required") }; if root == "" { root, _ = os.Getwd() }; pattern = filepath.ToSlash(pattern); var matches []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error { if err != nil { return nil }; if entry.IsDir() { name := entry.Name(); if name == ".git" || name == "node_modules" || name == ".cache" { return filepath.SkipDir }; return nil }; rel, _ := filepath.Rel(root, path); rel = filepath.ToSlash(rel); ok, _ := pathMatch(pattern, rel); if ok { matches = append(matches, rel) }; return nil })
	sort.Strings(matches); return matches, err
}
func pathMatch(pattern, path string) (bool, error) { if ok, err := filepath.Match(pattern, path); ok || err != nil { return ok, err }; if strings.HasPrefix(pattern, "**/") { if ok, err := filepath.Match(strings.TrimPrefix(pattern, "**/"), filepath.Base(path)); ok || err != nil { return ok, err } }; if strings.Contains(pattern, "**") { parts := strings.Split(pattern, "**"); return strings.HasPrefix(path, strings.TrimSuffix(parts[0], "/")) && strings.HasSuffix(path, strings.TrimPrefix(parts[len(parts)-1], "/")), nil }; return false, nil }

type GrepArgs struct { Pattern string `json:"pattern"`; Include string `json:"include"`; Path string `json:"path"` }
func runGrep(_ context.Context, tc ToolContext, raw json.RawMessage) (Result, error) { var args GrepArgs; if err := json.Unmarshal(raw, &args); err != nil { return Result{IsError: true, Content: err.Error()}, err }; root := tc.Cwd; if args.Path != "" { root = resolvePath(tc.Cwd, args.Path) }; result, err := Grep(root, args.Pattern, args.Include); if err != nil { return Result{IsError: true, Content: err.Error()}, err }; return Result{Content: result}, nil }

func Grep(root, pattern, include string) (string, error) {
	if pattern == "" { return "", fmt.Errorf("pattern is required") }; re, err := regexp.Compile(pattern); if err != nil { return "", fmt.Errorf("invalid pattern: %w", err) }; if root == "" { root, _ = os.Getwd() }; var lines []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error { if walkErr != nil { return nil }; if entry.IsDir() { if entry.Name() == ".git" || entry.Name() == "node_modules" || entry.Name() == ".cache" { return filepath.SkipDir }; return nil }; if include != "" { ok, _ := pathMatch(include, entry.Name()); if !ok && strings.Contains(include, "{") { ok = includeBraceMatch(include, entry.Name()) }; if !ok { return nil } }; data, readErr := os.ReadFile(path); if readErr != nil || len(data) > 2<<20 || strings.IndexByte(string(data[:minInt(len(data), 8192)]), 0) >= 0 { return nil }; for number, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") { if re.MatchString(line) { rel, _ := filepath.Rel(root, path); lines = append(lines, fmt.Sprintf("%s:%d:%s", filepath.ToSlash(rel), number+1, line)) } }; return nil })	sort.Strings(lines); return strings.Join(lines, "\n"), err
}
func includeBraceMatch(pattern, name string) bool { start := strings.Index(pattern, "{"); if start < 0 { return false }; tail := pattern[start+1:]; close := strings.Index(tail, "}"); if close < 0 { return false }; prefix, suffix := pattern[:start], tail[close+1:]; for _, choice := range strings.Split(tail[:close], ",") { if ok, _ := filepath.Match(prefix+choice+suffix, name); ok { return true } }; return false }
