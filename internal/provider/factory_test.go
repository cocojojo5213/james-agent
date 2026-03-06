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
		{"custom-provider", "custom-provider"},
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
