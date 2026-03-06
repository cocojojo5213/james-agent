package provider

import (
	"testing"

	"github.com/cocojojo5213/james-agent/internal/config"
)

func TestNormalizeType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", TypeAnthropic},
		{"anthropic", TypeAnthropic},
		{"openai", TypeOpenAI},
		{"openai-compatible", TypeOpenAICompatible},
		{"openai_compatible", TypeOpenAICompatible},
		{"openai compatible", TypeOpenAICompatible},
		{"deepseek", "deepseek"},
		{"groq", "groq"},
		{"xai", "xai"},
		{"unknown-xyz", "unknown-xyz"},
	}

	for _, tt := range tests {
		got := NormalizeType(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDisplayType(t *testing.T) {
	if got := DisplayType(""); got != "anthropic (default)" {
		t.Fatalf("DisplayType(\"\") = %q, want anthropic (default)", got)
	}
	if got := DisplayType("openai_compatible"); got != "openai-compatible" {
		t.Fatalf("DisplayType(openai_compatible) = %q, want openai-compatible", got)
	}
	if got := DisplayType("deepseek"); got != "deepseek" {
		t.Fatalf("DisplayType(deepseek) = %q, want deepseek", got)
	}
}

func TestBuildModelFactory_KnownThirdParty(t *testing.T) {
	providers := []string{"deepseek", "groq", "xai", "together", "mistral", "moonshot", "openrouter", "siliconflow", "ollama"}
	for _, ptype := range providers {
		cfg := &config.Config{
			Provider: config.ProviderConfig{
				Type:   ptype,
				APIKey: "test-key",
			},
			Agent: config.AgentConfig{
				MaxTokens:   4096,
				Temperature: 0.7,
			},
		}
		factory, err := BuildModelFactory(cfg)
		if err != nil {
			t.Errorf("BuildModelFactory(%q) returned error: %v", ptype, err)
			continue
		}
		if factory == nil {
			t.Errorf("BuildModelFactory(%q) returned nil factory", ptype)
		}
	}
}

func TestBuildModelFactory_ThirdPartyWithCustomBaseURL(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "deepseek",
			APIKey:  "test-key",
			BaseURL: "https://custom-proxy.example.com/v1",
		},
		Agent: config.AgentConfig{
			Model:       "deepseek-reasoner",
			MaxTokens:   4096,
			Temperature: 0.7,
		},
	}
	factory, err := BuildModelFactory(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if factory == nil {
		t.Fatal("factory should not be nil")
	}
}

func TestBuildModelFactory_UnsupportedProvider(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type: "unknown-provider",
		},
	}

	_, err := BuildModelFactory(cfg)
	if err == nil {
		t.Fatal("expected unsupported provider error")
	}
}

func TestLookupPreset(t *testing.T) {
	preset, ok := LookupPreset("deepseek")
	if !ok {
		t.Fatal("expected deepseek preset to exist")
	}
	if preset.BaseURL != "https://api.deepseek.com" {
		t.Errorf("deepseek baseURL = %q, want https://api.deepseek.com", preset.BaseURL)
	}
	if preset.DefaultModel != "deepseek-chat" {
		t.Errorf("deepseek defaultModel = %q, want deepseek-chat", preset.DefaultModel)
	}

	_, ok = LookupPreset("nonexistent")
	if ok {
		t.Error("expected nonexistent preset to not exist")
	}
}
