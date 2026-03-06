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
		return strings.TrimSpace(raw)
	}
}

func buildProvider(providerCfg config.ProviderConfig, agentCfg config.AgentConfig) (api.ModelFactory, error) {
	providerType := NormalizeType(providerCfg.Type)
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
		return nil, fmt.Errorf(
			"unsupported provider type %q (supported: %s, %s, %s)",
			providerCfg.Type,
			TypeAnthropic,
			TypeOpenAI,
			TypeOpenAICompatible,
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
