package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/arkiifr/afferr/internal/config"
)

var secretPattern = regexp.MustCompile(`(?i)(sk-[A-Za-z0-9_-]{8,}|AKIA[0-9A-Z]{12,}|ghp_[A-Za-z0-9]{20,}|xox[baprs]-[A-Za-z0-9-]{8,})`)

// ResolveAuth applies affer's non-interactive authentication order. Interactive
// device/OAuth flows are deliberately initiated by /connect, never by a model
// request, so a headless run cannot hang waiting for a terminal.
func ResolveAuth(providerID string, options config.ProviderOptions, def Definition) (string, error) {
	if raw := strings.TrimSpace(options.APIKey); raw != "" && !strings.HasPrefix(raw, "{env:") {
		return raw, nil
	}
	if strings.HasPrefix(options.APIKey, "{env:") && strings.HasSuffix(options.APIKey, "}") {
		name := strings.TrimSuffix(strings.TrimPrefix(options.APIKey, "{env:"), "}")
		if value := strings.TrimSpace(os.Getenv(name)); value != "" { return value, nil }
	}
	for _, name := range def.EnvKeys {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" { return value, nil }
	}
	if value := authFileValue(providerID); value != "" { return value, nil }
	// Local providers intentionally work without credentials. Cloud providers
	// may also rely on their SDK/CLI credential chain.
	if providerID == "azure-openai" && commandExists("az") { return "", nil }
	if providerID == "bedrock" && commandExists("aws") { return "", nil }
	if providerID == "vertex" && commandExists("gcloud") { return "", nil }
	return "", nil
}

func authFilePath() string {
	home, err := os.UserHomeDir(); if err != nil { return "" }
	return filepath.Join(home, ".local", "share", "affer", "auth.json")
}

func authFileValue(providerID string) string {
	path := authFilePath(); if path == "" { return "" }
	data, err := os.ReadFile(path); if err != nil { return "" }
	var values map[string]any
	if json.Unmarshal(data, &values) != nil { return "" }
	for _, key := range []string{providerID, "apiKey", "api_key", "token"} {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" { return strings.TrimSpace(value) }
	}
	return ""
}

func commandExists(name string) bool { _, err := exec.LookPath(name); return err == nil }

// Redact is safe for logs and user-visible provider diagnostics. It does not
// attempt to reveal a key's prefix or suffix.
func Redact(value string) string {
	if strings.TrimSpace(value) == "" { return "" }
	return secretPattern.ReplaceAllString(value, "[REDACTED]")
}

func ValidateProviderID(id string) error {
	if _, ok := DefinitionFor(id); !ok { return fmt.Errorf("unknown provider %q", id) }
	return nil
}
