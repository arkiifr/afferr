package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/arkiifr/afferr/internal/config"
)

const catalogTTL = 24 * time.Hour

type ModelCapabilities struct {
	ToolCall         bool `json:"toolcall"`
	Vision           bool `json:"vision"`
	Reasoning        bool `json:"reasoning"`
	Temperature      bool `json:"temperature"`
	StructuredOutput bool `json:"structuredOutput"`
	PDF              bool `json:"pdf"`
	Audio            bool `json:"audio"`
}

type ModelRecord struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Provider      string            `json:"provider,omitempty"`
	Family        string            `json:"family"`
	Context       int               `json:"context"`
	MaxOutput    int               `json:"maxOutput"`
	InputPer1M   float64           `json:"inputPer1M"`
	OutputPer1M  float64           `json:"outputPer1M"`
	CacheReadPer1M float64         `json:"cacheReadPer1M"`
	Supports     ModelCapabilities  `json:"supports"`
	Deprecated   bool              `json:"deprecated"`
	ReleaseDate  string            `json:"releaseDate,omitempty"`
	Variants     []string          `json:"variants,omitempty"`
}

func (m ModelRecord) FullID() string { if m.Provider == "" { return m.ID }; return m.Provider + "/" + m.ID }
func (m ModelRecord) Free() bool { return m.InputPer1M == 0 && m.OutputPer1M == 0 }

type Catalog struct {
	FetchedAt time.Time    `json:"fetchedAt"`
	Models    []ModelRecord `json:"models"`
}

func CatalogPath() string {
	cache, err := os.UserCacheDir(); if err != nil { if home, e := os.UserHomeDir(); e == nil { cache = filepath.Join(home, ".cache") } }
	return filepath.Join(cache, "affer", "models.json")
}

func LoadCatalog(ctx context.Context, cfg config.Config, refresh bool) (Catalog, error) {
	path := CatalogPath()
	if !refresh {
		if data, err := os.ReadFile(path); err == nil {
			var c Catalog
			if json.Unmarshal(data, &c) == nil && !c.FetchedAt.IsZero() && time.Since(c.FetchedAt) < catalogTTL && len(c.Models) > 0 { return c, nil }
		}
	}
	c := Catalog{FetchedAt: time.Now().UTC(), Models: BuiltinModels()}
	if refresh && !cfg.Pure { c.Models = refreshConfigured(ctx, cfg, c.Models) }
	if err := writeCatalog(path, c); err != nil {
		// Read-only sandboxes should still be able to list the built-in catalog.
		return c, nil
	}
	return c, nil
}

func writeCatalog(path string, c Catalog) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { return err }
	data, err := json.MarshalIndent(c, "", "  "); if err != nil { return err }
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil { return err }
	return os.Rename(tmp, path)
}

func refreshConfigured(ctx context.Context, cfg config.Config, models []ModelRecord) []ModelRecord {
	seen := make(map[string]bool, len(models)); for _, m := range models { seen[m.FullID()] = true }
	client := &http.Client{Timeout: 6 * time.Second}
	for id, pc := range cfg.Provider {
		def, ok := DefinitionFor(id); if !ok || pc.Options.BaseURL == "" { continue }
		key, _ := ResolveAuth(id, pc.Options, def)
		if key == "" && id != "ollama" && id != "lmstudio" { continue }
		url := strings.TrimRight(pc.Options.BaseURL, "/") + "/models"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil); if err != nil { continue }
		req.Header.Set("Accept", "application/json")
		if key != "" { req.Header.Set("Authorization", "Bearer "+key) }
		for k, v := range pc.Options.Headers { req.Header.Set(k, v) }
		resp, err := client.Do(req); if err != nil { continue }
		var payload struct { Data []struct { ID string `json:"id"`; Name string `json:"name"` } `json:"data"` }
		_ = json.NewDecoder(resp.Body).Decode(&payload); resp.Body.Close()
		for _, item := range payload.Data {
			if item.ID == "" { continue }; full := id + "/" + item.ID; if seen[full] { continue }
			name := item.Name; if name == "" { name = item.ID }
			models = append(models, ModelRecord{ID: item.ID, Name: name, Provider: id, Family: id, Context: 128000, MaxOutput: 16384, Supports: ModelCapabilities{ToolCall: true, Temperature: true}}); seen[full] = true
		}
	}
	return models
}

func BuiltinModels() []ModelRecord {
	capTools := ModelCapabilities{ToolCall: true, Vision: true, Reasoning: true, Temperature: true, StructuredOutput: true}
	free := ModelCapabilities{ToolCall: true, Temperature: true}
	return []ModelRecord{
		{Provider: "anthropic", ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", Family: "claude", Context: 200000, MaxOutput: 16384, InputPer1M: 3, OutputPer1M: 15, CacheReadPer1M: .3, Supports: capTools, ReleaseDate: "2025-09-29", Variants: []string{"none", "high", "max"}},
		{Provider: "anthropic", ID: "claude-opus-4-1", Name: "Claude Opus 4.1", Family: "claude", Context: 200000, MaxOutput: 32000, InputPer1M: 15, OutputPer1M: 75, CacheReadPer1M: 1.5, Supports: capTools, Variants: []string{"none", "high", "max"}},
		{Provider: "openai", ID: "gpt-5.1-codex", Name: "GPT-5.1 Codex", Family: "gpt", Context: 400000, MaxOutput: 128000, InputPer1M: 1.25, OutputPer1M: 10, Supports: capTools, Variants: []string{"none", "minimal", "low", "medium", "high", "xhigh"}},
		{Provider: "openai", ID: "gpt-5-nano", Name: "GPT-5 Nano", Family: "gpt", Context: 400000, MaxOutput: 128000, InputPer1M: .05, OutputPer1M: .4, Supports: capTools},
		{Provider: "google", ID: "gemini-3-flash", Name: "Gemini 3 Flash", Family: "gemini", Context: 1000000, MaxOutput: 65536, InputPer1M: .5, OutputPer1M: 3, Supports: capTools, Variants: []string{"low", "high"}},
		{Provider: "google", ID: "gemini-3-flash-lite", Name: "Gemini 3 Flash Lite", Family: "gemini", Context: 1000000, MaxOutput: 32768, Supports: capTools},
		{Provider: "ollama", ID: "qwen3", Name: "Qwen 3", Family: "qwen", Context: 32768, MaxOutput: 8192, Supports: free},
		{Provider: "ollama", ID: "llama3.2", Name: "Llama 3.2", Family: "llama", Context: 131072, MaxOutput: 8192, Supports: free},
		{Provider: "deepseek", ID: "deepseek-chat", Name: "DeepSeek Chat", Family: "deepseek", Context: 128000, MaxOutput: 8192, InputPer1M: .27, OutputPer1M: 1.1, Supports: capTools},
		{Provider: "openrouter", ID: "auto", Name: "OpenRouter Auto", Family: "router", Context: 200000, MaxOutput: 32768, Supports: capTools},
	}
}

func FilterModels(models []ModelRecord, providerID, query string) []ModelRecord {
	query = strings.ToLower(strings.TrimSpace(query)); out := make([]ModelRecord, 0, len(models))
	for _, m := range models {
		if providerID != "" && !strings.EqualFold(m.Provider, providerID) { continue }
		if query != "" && !strings.Contains(strings.ToLower(m.ID+" "+m.Name+" "+m.Family), query) { continue }
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool { if out[i].Provider == out[j].Provider { return out[i].Name < out[j].Name }; return out[i].Provider < out[j].Provider })
	return out
}

func FuzzyScore(query string, m ModelRecord) int {
	query = strings.ToLower(strings.TrimSpace(query)); if query == "" { return 0 }
	candidate := strings.ToLower(m.FullID()+" "+m.Name)
	if candidate == query { return 1000 }; if strings.HasPrefix(candidate, query) { return 800 }
	if strings.Contains(candidate, query) { return 500 }
	qi, score := 0, 0
	for _, r := range candidate { if qi < len([]rune(query)) && r == []rune(query)[qi] { score += 10; qi++ } }
	if qi == len([]rune(query)) { return score }
	return -1
}
