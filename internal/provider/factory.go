package provider

import (
	"fmt"
	"strings"

	"github.com/cexll/agentsdk-go/pkg/api"
	"github.com/cexll/agentsdk-go/pkg/model"
	"github.com/cocojojo5213/james-agent/internal/config"
)

const (
	TypeAnthropic        = "anthropic"
	TypeOpenAI           = "openai"
	TypeOpenAICompatible = "openai-compatible"
)

// Preset holds auto-fill values for a known OpenAI-compatible provider.
type Preset struct {
	BaseURL      string
	DefaultModel string
}

// KnownProviders maps shorthand type names to their API base URLs and default models.
var KnownProviders = map[string]Preset{
	"deepseek":    {BaseURL: "https://api.deepseek.com", DefaultModel: "deepseek-chat"},
	"groq":        {BaseURL: "https://api.groq.com/openai/v1", DefaultModel: "llama-3.3-70b-versatile"},
	"xai":         {BaseURL: "https://api.x.ai/v1", DefaultModel: "grok-5"},
	"together":    {BaseURL: "https://api.together.xyz/v1", DefaultModel: "meta-llama/Llama-3.3-70B-Instruct-Turbo"},
	"mistral":     {BaseURL: "https://api.mistral.ai/v1", DefaultModel: "mistral-large-latest"},
	"moonshot":    {BaseURL: "https://api.moonshot.cn/v1", DefaultModel: "moonshot-v1-8k"},
	"zhipu":       {BaseURL: "https://open.bigmodel.cn/api/paas/v4", DefaultModel: "glm-4"},
	"yi":          {BaseURL: "https://api.lingyiwanwu.com/v1", DefaultModel: "yi-large"},
	"siliconflow": {BaseURL: "https://api.siliconflow.cn/v1", DefaultModel: "deepseek-ai/DeepSeek-V3"},
	"openrouter":  {BaseURL: "https://openrouter.ai/api/v1", DefaultModel: ""},
	"ollama":      {BaseURL: "http://localhost:11434/v1", DefaultModel: "llama3"},
	"volcengine":  {BaseURL: "https://ark.cn-beijing.volces.com/api/v3", DefaultModel: ""},
	"baichuan":    {BaseURL: "https://api.baichuan-ai.com/v1", DefaultModel: "Baichuan4"},
	"minimax":     {BaseURL: "https://api.minimax.chat/v1", DefaultModel: "abab6.5s-chat"},
	"infini":      {BaseURL: "https://cloud.infini-ai.com/maas/v1", DefaultModel: ""},
}

// LookupPreset returns the preset for a provider type if it exists.
func LookupPreset(providerType string) (Preset, bool) {
	p, ok := KnownProviders[strings.TrimSpace(strings.ToLower(providerType))]
	return p, ok
}

// NormalizeType returns canonical provider type names used by runtime wiring.
func NormalizeType(raw string) string {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	switch normalized {
	case "":
		return TypeAnthropic
	case TypeAnthropic:
		return TypeAnthropic
	case TypeOpenAI:
		return TypeOpenAI
	case TypeOpenAICompatible, "openai_compatible", "openai compatible":
		return TypeOpenAICompatible
	default:
		if _, ok := KnownProviders[normalized]; ok {
			return normalized
		}
		return normalized
	}
}

func DisplayType(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return TypeAnthropic + " (default)"
	}

	normalized := NormalizeType(raw)
	switch normalized {
	case TypeAnthropic, TypeOpenAI, TypeOpenAICompatible:
		return normalized
	default:
		if _, ok := KnownProviders[normalized]; ok {
			return normalized
		}
		return strings.TrimSpace(raw)
	}
}

func buildProvider(providerCfg config.ProviderConfig, agentCfg config.AgentConfig) (api.ModelFactory, error) {
	providerType := NormalizeType(providerCfg.Type)

	// Check if this is a known third-party provider shorthand
	if preset, ok := KnownProviders[providerType]; ok {
		baseURL := providerCfg.BaseURL
		if baseURL == "" {
			baseURL = preset.BaseURL
		}
		modelName := agentCfg.Model
		if modelName == "" && preset.DefaultModel != "" {
			modelName = preset.DefaultModel
		}
		temp := agentCfg.Temperature
		return &model.OpenAIProvider{
			APIKey:      providerCfg.APIKey,
			BaseURL:     baseURL,
			ModelName:   modelName,
			MaxTokens:   agentCfg.MaxTokens,
			Temperature: &temp,
		}, nil
	}

	switch providerType {
	case TypeOpenAI, TypeOpenAICompatible:
		temp := agentCfg.Temperature
		return &model.OpenAIProvider{
			APIKey:      providerCfg.APIKey,
			BaseURL:     providerCfg.BaseURL,
			ModelName:   agentCfg.Model,
			MaxTokens:   agentCfg.MaxTokens,
			Temperature: &temp,
		}, nil
	case TypeAnthropic:
		temp := agentCfg.Temperature
		return &model.AnthropicProvider{
			APIKey:      providerCfg.APIKey,
			BaseURL:     providerCfg.BaseURL,
			ModelName:   agentCfg.Model,
			MaxTokens:   agentCfg.MaxTokens,
			Temperature: &temp,
		}, nil
	default:
		supported := []string{TypeAnthropic, TypeOpenAI, TypeOpenAICompatible}
		for name := range KnownProviders {
			supported = append(supported, name)
		}
		return nil, fmt.Errorf(
			"unsupported provider type %q (supported: %s)",
			providerCfg.Type,
			strings.Join(supported, ", "),
		)
	}
}

func BuildModelFactory(cfg *config.Config) (api.ModelFactory, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	return buildProvider(cfg.Provider, cfg.Agent)
}

// BuildFallbackFactory returns the fallback provider factory if configured.
func BuildFallbackFactory(cfg *config.Config) (api.ModelFactory, error) {
	if cfg == nil || cfg.Provider.Fallback == nil {
		return nil, nil
	}
	return buildProvider(*cfg.Provider.Fallback, cfg.Agent)
}
