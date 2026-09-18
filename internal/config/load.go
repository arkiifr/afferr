package config

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/muhammadmuzzammil1998/jsonc"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed schema.json
var schemaFS embed.FS

// Config is the merged application configuration. The zero value is useful in
// tests, but callers should normally use Default followed by Load.
type Config struct {
	Model       string                      `json:"model,omitempty"`
	Fallback    []string                    `json:"fallback,omitempty"`
	Router      RouterConfig                `json:"router,omitempty"`
	Provider    map[string]ProviderConfig   `json:"provider,omitempty"`
	Permission  PermissionConfig            `json:"permission,omitempty"`
	Theme       string                      `json:"theme,omitempty"`
	MaxSteps    int                         `json:"maxSteps,omitempty"`
	BudgetUSD   float64                     `json:"budgetUSD,omitempty"`
	Formatters  map[string]FormatterConfig  `json:"formatters,omitempty"`
	LSP         map[string]LSPConfig        `json:"lsp,omitempty"`
	MCP         map[string]MCPServerConfig  `json:"mcp,omitempty"`
	Agent       string                      `json:"agent,omitempty"`
	AutoApprove bool                        `json:"autoApprove,omitempty"`
	Pure        bool                        `json:"pure,omitempty"`
}

type RouterConfig struct {
	Default string `json:"default,omitempty"`
	Fast    string `json:"fast,omitempty"`
	Title   string `json:"title,omitempty"`
	Compact string `json:"compact,omitempty"`
}

type ProviderConfig struct {
	Type    string                         `json:"type,omitempty"`
	Options ProviderOptions                `json:"options,omitempty"`
	Models  map[string]ModelConfigOverride `json:"models,omitempty"`
}

type ProviderOptions struct {
	BaseURL string            `json:"baseURL,omitempty"`
	APIKey  string            `json:"apiKey,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Region  string            `json:"region,omitempty"`
	Project string            `json:"project,omitempty"`
	Zone    string            `json:"zone,omitempty"`
}

type ModelConfigOverride struct {
	Variants map[string]map[string]any `json:"variants,omitempty"`
}

type PermissionConfig struct {
	Bash      string   `json:"bash,omitempty"`
	Edit      string   `json:"edit,omitempty"`
	Write     string   `json:"write,omitempty"`
	Webfetch  string   `json:"webfetch,omitempty"`
	Read      string   `json:"read,omitempty"`
	Wildcard  string   `json:"*,omitempty"`
	Mode      string   `json:"mode,omitempty"`
	Deny      []string `json:"deny,omitempty"`
}

type FormatterConfig struct {
	Enabled bool   `json:"enabled,omitempty"`
	Command string `json:"command,omitempty"`
}

type LSPConfig struct {
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Enabled bool     `json:"enabled,omitempty"`
}

type MCPServerConfig struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
}

// Default returns conservative, useful defaults. Provider options are only
// defaults; credentials are resolved lazily by provider.AuthResolver.
func Default() Config {
	return Config{
		Fallback: []string{"anthropic/claude-sonnet-4-5", "openai/gpt-5.1-codex", "google/gemini-3-flash"},
		Router: RouterConfig{Fast: "openai/gpt-5-nano", Title: "zen/jev-latest", Compact: "google/gemini-3-flash-lite"},
		Provider: map[string]ProviderConfig{
			"openai":     {Type: "openai-compatible", Options: ProviderOptions{BaseURL: "https://api.openai.com/v1", APIKey: "{env:OPENAI_API_KEY}"}},
			"anthropic":  {Type: "anthropic", Options: ProviderOptions{BaseURL: "https://api.anthropic.com", APIKey: "{env:ANTHROPIC_API_KEY}"}},
			"google":     {Type: "gemini", Options: ProviderOptions{BaseURL: "https://generativelanguage.googleapis.com"}},
			"ollama":     {Type: "openai-compatible", Options: ProviderOptions{BaseURL: "http://localhost:11434/v1", APIKey: "ollama"}},
			"lmstudio":   {Type: "openai-compatible", Options: ProviderOptions{BaseURL: "http://localhost:1234/v1", APIKey: "lm-studio"}},
			"zen":        {Type: "openai-compatible", Options: ProviderOptions{BaseURL: "https://example.com/zen/v1", APIKey: "{env:AFFER_ZEN_KEY}"}},
			"openrouter": {Type: "openai-compatible", Options: ProviderOptions{BaseURL: "https://openrouter.ai/api/v1", APIKey: "{env:OPENROUTER_API_KEY}"}},
		},
		Permission: PermissionConfig{Bash: "ask", Edit: "ask", Write: "ask", Webfetch: "allow", Read: "allow", Mode: "normal"},
		Theme: "dark", MaxSteps: 128,
		Formatters: map[string]FormatterConfig{"gofmt": {Enabled: true, Command: "gofmt -w"}},
	}
}

// Load merges global and project JSONC configuration. Explicit config files
// are selected with AFFER_CONFIG; otherwise the conventional XDG config path
// and .affer.jsonc in projectDir are read if present.
func Load(projectDir string) (Config, error) {
	cfg := Default()
	global, project, err := configPaths(projectDir)
	if err != nil { return cfg, err }
	if explicit := os.Getenv("AFFER_CONFIG"); explicit != "" { global = expandPath(explicit) }
	for _, path := range []string{global, project} {
		if path == "" { continue }
		part, exists, err := read(path)
		if err != nil { return cfg, fmt.Errorf("load %s: %w", path, err) }
		if exists { cfg = Merge(cfg, part) }
	}
	if cfg.MaxSteps <= 0 { cfg.MaxSteps = 128 }
	if cfg.Permission.Mode == "" { cfg.Permission.Mode = "normal" }
	return cfg, nil
}

func configPaths(projectDir string) (global, project string, err error) {
	if projectDir == "" { projectDir, err = os.Getwd(); if err != nil { return "", "", err } }
	projectDir, err = filepath.Abs(projectDir); if err != nil { return "", "", err }
	configDir, err := os.UserConfigDir(); if err != nil { configDir = filepath.Join("~", ".config") }
	return filepath.Join(configDir, "affer", "config.jsonc"), filepath.Join(projectDir, ".affer.jsonc"), nil
}

func read(path string) (Config, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) { return Config{}, false, nil }
	if err != nil { return Config{}, false, err }
	clean, err := jsonc.Parse(data)
	if err != nil { return Config{}, true, fmt.Errorf("invalid JSONC: %w", err) }
	var cfg Config
	if err := json.Unmarshal(clean, &cfg); err != nil { return Config{}, true, fmt.Errorf("invalid JSON: %w", err) }
	if err := validate(cfg, clean); err != nil { return Config{}, true, err }
	return cfg, true, nil
}

func validate(cfg Config, raw []byte) error {
	b, err := schemaFS.ReadFile("schema.json"); if err != nil { return err }
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("affer://schema.json", bytes.NewReader(b)); err != nil { return fmt.Errorf("compile config schema: %w", err) }
	sch, err := compiler.Compile("affer://schema.json"); if err != nil { return fmt.Errorf("compile config schema: %w", err) }
	var value any
	if err := json.Unmarshal(raw, &value); err != nil { return err }
	if err := sch.Validate(value); err != nil { return fmt.Errorf("config schema validation: %w", err) }
	if cfg.Permission.Mode != "" && cfg.Permission.Mode != "normal" && cfg.Permission.Mode != "plan" && cfg.Permission.Mode != "auto" { return fmt.Errorf("permission.mode must be normal, plan, or auto") }
	return nil
}

// Save writes a human-readable JSON configuration atomically.
func Save(path string, cfg Config) error {
	path = expandPath(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { return err }
	data, err := json.MarshalIndent(cfg, "", "  "); if err != nil { return err }
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil { return err }
	return os.Rename(tmp, path)
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") { if home, err := os.UserHomeDir(); err == nil { return filepath.Join(home, path[2:]) } }
	return filepath.Clean(path)
}

// Platform returns a concise platform identifier for prompts and diagnostics.
func Platform() string { return runtime.GOOS + "/" + runtime.GOARCH }
