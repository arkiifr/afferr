package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/arkiifr/afferr/internal/config"
)

// ProviderIDs is the stable provider vocabulary accepted by affer. Aliases are
// intentionally not silently invented: a typo should be actionable.
var ProviderIDs = []string{
	"openai", "anthropic", "google", "azure-openai", "bedrock", "vertex", "ollama", "lmstudio", "openrouter", "deepseek", "moonshot", "qwen", "zhipu", "minimax", "kimi", "groq", "cerebras", "fireworks", "together", "xai", "mistral", "cohere", "nvidia", "github-copilot", "zen", "affer-gateway",
}

type ProviderKind string

const (
	KindOpenAICompatible ProviderKind = "openai-compatible"
	KindAnthropic        ProviderKind = "anthropic"
	KindGemini           ProviderKind = "gemini"
)

type Definition struct {
	ID      string
	Name    string
	Kind    ProviderKind
	BaseURL string
	EnvKeys []string
}

var definitions = map[string]Definition{
	"openai": {"openai", "OpenAI", KindOpenAICompatible, "https://api.openai.com/v1", []string{"OPENAI_API_KEY"}},
	"anthropic": {"anthropic", "Anthropic", KindAnthropic, "https://api.anthropic.com", []string{"ANTHROPIC_API_KEY"}},
	"google": {"google", "Google Gemini", KindGemini, "https://generativelanguage.googleapis.com", []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}},
	"azure-openai": {"azure-openai", "Azure OpenAI", KindOpenAICompatible, "", []string{"AZURE_OPENAI_API_KEY"}},
	"bedrock": {"bedrock", "Amazon Bedrock", KindAnthropic, "", []string{"AWS_ACCESS_KEY_ID"}},
	"vertex": {"vertex", "Google Vertex AI", KindGemini, "", []string{"GOOGLE_APPLICATION_CREDENTIALS"}},
	"ollama": {"ollama", "Ollama", KindOpenAICompatible, "http://localhost:11434/v1", nil},
	"lmstudio": {"lmstudio", "LM Studio", KindOpenAICompatible, "http://localhost:1234/v1", nil},
	"openrouter": {"openrouter", "OpenRouter", KindOpenAICompatible, "https://openrouter.ai/api/v1", []string{"OPENROUTER_API_KEY"}},
	"deepseek": {"deepseek", "DeepSeek", KindOpenAICompatible, "https://api.deepseek.com/v1", []string{"DEEPSEEK_API_KEY"}},
	"moonshot": {"moonshot", "Moonshot", KindOpenAICompatible, "https://api.moonshot.ai/v1", []string{"MOONSHOT_API_KEY"}},
	"qwen": {"qwen", "Qwen", KindOpenAICompatible, "https://dashscope.aliyuncs.com/compatible-mode/v1", []string{"DASHSCOPE_API_KEY"}},
	"zhipu": {"zhipu", "Zhipu", KindOpenAICompatible, "https://open.bigmodel.cn/api/paas/v4", []string{"ZHIPUAI_API_KEY"}},
	"minimax": {"minimax", "MiniMax", KindOpenAICompatible, "https://api.minimax.io/v1", []string{"MINIMAX_API_KEY"}},
	"kimi": {"kimi", "Kimi", KindOpenAICompatible, "https://api.moonshot.ai/v1", []string{"MOONSHOT_API_KEY"}},
	"groq": {"groq", "Groq", KindOpenAICompatible, "https://api.groq.com/openai/v1", []string{"GROQ_API_KEY"}},
	"cerebras": {"cerebras", "Cerebras", KindOpenAICompatible, "https://api.cerebras.ai/v1", []string{"CEREBRAS_API_KEY"}},
	"fireworks": {"fireworks", "Fireworks AI", KindOpenAICompatible, "https://api.fireworks.ai/inference/v1", []string{"FIREWORKS_API_KEY"}},
	"together": {"together", "Together AI", KindOpenAICompatible, "https://api.together.xyz/v1", []string{"TOGETHER_API_KEY"}},
	"xai": {"xai", "xAI", KindOpenAICompatible, "https://api.x.ai/v1", []string{"XAI_API_KEY"}},
	"mistral": {"mistral", "Mistral", KindOpenAICompatible, "https://api.mistral.ai/v1", []string{"MISTRAL_API_KEY"}},
	"cohere": {"cohere", "Cohere", KindOpenAICompatible, "https://api.cohere.ai/compatibility/v1", []string{"COHERE_API_KEY"}},
	"nvidia": {"nvidia", "NVIDIA NIM", KindOpenAICompatible, "https://integrate.api.nvidia.com/v1", []string{"NVIDIA_API_KEY"}},
	"github-copilot": {"github-copilot", "GitHub Copilot", KindOpenAICompatible, "https://api.githubcopilot.com", []string{"GITHUB_TOKEN"}},
	"zen": {"zen", "Zen", KindOpenAICompatible, "https://example.com/zen/v1", []string{"AFFER_ZEN_KEY"}},
	"affer-gateway": {"affer-gateway", "Affer Gateway", KindOpenAICompatible, "https://api.affer.dev/v1", []string{"AFFER_API_KEY"}},
}

func DefinitionFor(id string) (Definition, bool) { d, ok := definitions[strings.ToLower(strings.TrimSpace(id))]; return d, ok }

func Definitions() []Definition {
	out := make([]Definition, 0, len(ProviderIDs))
	for _, id := range ProviderIDs { if d, ok := definitions[id]; ok { out = append(out, d) } }
	return out
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  jsonRaw         `json:"parameters"`
}

// jsonRaw avoids exposing a second JSON type in the public provider API while
// still allowing schemas to pass through without lossy map conversions.
type jsonRaw = json.RawMessage

type Usage struct {
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	TotalTokens  int     `json:"total_tokens,omitempty"`
	Cost         float64 `json:"cost,omitempty"`
}

type Request struct {
	Model       string
	Messages    []Message
	Tools       []ToolDefinition
	ToolChoice  any
	Variant     map[string]any
	Temperature *float64
	MaxTokens   int
}

type Event struct {
	Type         string
	Text         string
	ToolCalls    []ToolCall
	Usage        Usage
	FinishReason string
	Error        error
}

type Client interface {
	Stream(ctx context.Context, request Request) (<-chan Event, error)
	Provider() string
}

type Factory struct {
	Config config.Config
}

func (f Factory) Client(id string) (Client, error) {
	providerID, model, err := SplitModel(id)
	if err != nil { return nil, err }
	_ = model
	def, ok := DefinitionFor(providerID)
	if !ok { return nil, fmt.Errorf("unknown provider %q", providerID) }
	pc := f.Config.Provider[providerID]
	if pc.Type == "" { pc.Type = string(def.Kind) }
	if pc.Options.BaseURL == "" { pc.Options.BaseURL = def.BaseURL }
	key, err := ResolveAuth(providerID, pc.Options, def)
	if err != nil { return nil, err }
	switch ProviderKind(pc.Type) {
	case KindAnthropic:
		return NewAnthropicClient(providerID, pc.Options.BaseURL, key, pc.Options.Headers), nil
	case KindGemini:
		return NewGeminiClient(providerID, pc.Options.BaseURL, key, pc.Options.Headers), nil
	default:
		return NewOpenAIClient(providerID, pc.Options.BaseURL, key, pc.Options.Headers), nil
	}
}

func SplitModel(value string) (providerID, model string, err error) {
	value = strings.TrimSpace(value)
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" { return "", "", fmt.Errorf("model must be provider/model, got %q", value) }
	if _, ok := DefinitionFor(parts[0]); !ok { return "", "", fmt.Errorf("unknown provider %q", parts[0]) }
	return strings.ToLower(parts[0]), parts[1], nil
}
