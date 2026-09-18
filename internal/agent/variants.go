package agent

import (
	"strings"

	"github.com/arkiifr/afferr/internal/config"
	"github.com/arkiifr/afferr/internal/provider"
)

func VariantNames(model string) []string { id, _, err := provider.SplitModel(model); if err != nil { return []string{"none"} }; switch id { case "anthropic": return []string{"none", "high", "max"}; case "openai", "azure-openai", "openrouter", "deepseek", "xai": return []string{"none", "minimal", "low", "medium", "high", "xhigh"}; case "google", "vertex": return []string{"none", "low", "high"}; default: return []string{"none"} } }
func CycleVariant(model, current string) string { values := VariantNames(model); current = strings.ToLower(current); for i, value := range values { if value == current { return values[(i+1)%len(values)] } }; return values[0] }

func VariantPayload(cfg config.Config, model, variant string) map[string]any {
	providerID, modelID, err := provider.SplitModel(model); if err != nil || variant == "" || variant == "none" { return nil }
	if pc, ok := cfg.Provider[providerID]; ok { if override, ok := pc.Models[modelID]; ok { if custom, ok := override.Variants[variant]; ok { out := map[string]any{}; for k, v := range custom { out[k] = v }; return out } } }
	switch providerID {
	case "anthropic":
		budget := 8192; if variant == "max" { budget = 32768 }; return map[string]any{"thinking": map[string]any{"type": "enabled", "budget_tokens": budget}}
	case "google", "vertex": return map[string]any{"thinkingLevel": strings.ToUpper(variant)}
	default: return map[string]any{"reasoning_effort": variant}
	}
}
