package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if cfg.Agent.Model != DefaultModel {
		t.Errorf("model = %q, want %q", cfg.Agent.Model, DefaultModel)
	}
	if cfg.Agent.MaxTokens != DefaultMaxTokens {
		t.Errorf("maxTokens = %d, want %d", cfg.Agent.MaxTokens, DefaultMaxTokens)
	}
	if cfg.Agent.MaxToolIterations != DefaultMaxToolIterations {
		t.Errorf("maxToolIterations = %d, want %d", cfg.Agent.MaxToolIterations, DefaultMaxToolIterations)
	}
	if cfg.Gateway.Host != DefaultHost {
		t.Errorf("host = %q, want %q", cfg.Gateway.Host, DefaultHost)
	}
	if cfg.Gateway.Port != DefaultPort {
		t.Errorf("port = %d, want %d", cfg.Gateway.Port, DefaultPort)
	}
	if cfg.Tools.ExecTimeout != DefaultExecTimeout {
		t.Errorf("execTimeout = %d, want %d", cfg.Tools.ExecTimeout, DefaultExecTimeout)
	}
	if !cfg.Tools.RestrictToWorkspace {
		t.Error("restrictToWorkspace should be true by default")
	}
	if !cfg.Skills.Enabled {
		t.Error("skills.enabled should be true by default")
	}
	if !cfg.AutoCompact.Enabled {
		t.Error("autoCompact.enabled should be true by default")
	}
	if cfg.AutoCompact.Threshold != 0.8 {
		t.Errorf("autoCompact.threshold = %v, want 0.8", cfg.AutoCompact.Threshold)
	}
	if cfg.AutoCompact.PreserveCount != 5 {
		t.Errorf("autoCompact.preserveCount = %d, want 5", cfg.AutoCompact.PreserveCount)
	}
	if cfg.Agent.Workspace == "" {
		t.Error("workspace should not be empty")
	}
}

func TestLoadConfig_NoFile(t *testing.T) {
	// Override config dir to a temp location
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Clear any env overrides
	t.Setenv("JAMES_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Agent.Model != DefaultModel {
		t.Errorf("expected default model %q, got %q", DefaultModel, cfg.Agent.Model)
	}
}

func TestLoadConfig_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Clear env overrides
	t.Setenv("JAMES_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")

	// Create config file
	cfgDir := filepath.Join(tmpDir, ".james-agent")
	os.MkdirAll(cfgDir, 0755)

	testCfg := map[string]any{
		"agent": map[string]any{
			"model":     "claude-opus-4-6",
			"maxTokens": 4096,
		},
		"provider": map[string]any{
			"apiKey": "sk-test-key",
		},
	}
	data, _ := json.MarshalIndent(testCfg, "", "  ")
	os.WriteFile(filepath.Join(cfgDir, "config.json"), data, 0644)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Agent.Model != "claude-opus-4-6" {
		t.Errorf("model = %q, want claude-opus-4-6", cfg.Agent.Model)
	}
	if cfg.Agent.MaxTokens != 4096 {
		t.Errorf("maxTokens = %d, want 4096", cfg.Agent.MaxTokens)
	}
	if cfg.Provider.APIKey != "sk-test-key" {
		t.Errorf("apiKey = %q, want sk-test-key", cfg.Provider.APIKey)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	tests := []struct {
		name    string
		envKey  string
		envVal  string
		wantKey string
	}{
		{"JAMES_API_KEY", "JAMES_API_KEY", "james-key", "james-key"},
		{"ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY", "anthropic-key", "anthropic-key"},
		{"ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_AUTH_TOKEN", "auth-token", "auth-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("JAMES_API_KEY", "")
			t.Setenv("ANTHROPIC_API_KEY", "")
			t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
			t.Setenv(tt.envKey, tt.envVal)

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig error: %v", err)
			}
			if cfg.Provider.APIKey != tt.wantKey {
				t.Errorf("apiKey = %q, want %q", cfg.Provider.APIKey, tt.wantKey)
			}
		})
	}
}

func TestLoadConfig_EnvPriority(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// JAMES_API_KEY takes priority over ANTHROPIC_API_KEY
	t.Setenv("JAMES_API_KEY", "james-wins")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-loses")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "token-loses")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Provider.APIKey != "james-wins" {
		t.Errorf("apiKey = %q, want james-wins", cfg.Provider.APIKey)
	}
}

func TestLoadConfig_BaseURLEnv(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	t.Setenv("JAMES_API_KEY", "key")
	t.Setenv("ANTHROPIC_BASE_URL", "http://localhost:8080")
	t.Setenv("JAMES_BASE_URL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Provider.BaseURL != "http://localhost:8080" {
		t.Errorf("baseURL = %q, want http://localhost:8080", cfg.Provider.BaseURL)
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg := DefaultConfig()
	cfg.Provider.APIKey = "test-key"

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".james-agent", "config.json"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal saved config: %v", err)
	}
	if loaded.Provider.APIKey != "test-key" {
		t.Errorf("saved apiKey = %q, want test-key", loaded.Provider.APIKey)
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfgDir := filepath.Join(tmpDir, ".james-agent")
	os.MkdirAll(cfgDir, 0755)
	os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte("invalid json"), 0644)

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadConfig_EmptyWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfgDir := filepath.Join(tmpDir, ".james-agent")
	os.MkdirAll(cfgDir, 0755)

	// Config with empty workspace - should use default
	testCfg := map[string]any{
		"agent": map[string]any{
			"workspace": "",
		},
	}
	data, _ := json.MarshalIndent(testCfg, "", "  ")
	os.WriteFile(filepath.Join(cfgDir, "config.json"), data, 0644)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Agent.Workspace == "" {
		t.Error("workspace should not be empty")
	}
}

func TestLoadConfig_TelegramToken(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	t.Setenv("JAMES_TELEGRAM_TOKEN", "test-telegram-token")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Channels.Telegram.Token != "test-telegram-token" {
		t.Errorf("telegram token = %q, want test-telegram-token", cfg.Channels.Telegram.Token)
	}
}

func TestLoadConfig_JAMESBaseURL(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	t.Setenv("JAMES_BASE_URL", "http://james.local")
	t.Setenv("ANTHROPIC_BASE_URL", "http://anthropic.local")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	// JAMES_BASE_URL takes priority
	if cfg.Provider.BaseURL != "http://james.local" {
		t.Errorf("baseURL = %q, want http://james.local", cfg.Provider.BaseURL)
	}
}

func TestLoadConfig_WeComEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	t.Setenv("JAMES_WECOM_TOKEN", "wecom-token")
	t.Setenv("JAMES_WECOM_ENCODING_AES_KEY", "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG")
	t.Setenv("JAMES_WECOM_RECEIVE_ID", "wecom-receive-id")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.Channels.WeCom.Token != "wecom-token" {
		t.Errorf("wecom token = %q, want wecom-token", cfg.Channels.WeCom.Token)
	}
	if cfg.Channels.WeCom.EncodingAESKey != "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG" {
		t.Errorf("wecom aes key = %q, want configured value", cfg.Channels.WeCom.EncodingAESKey)
	}
	if cfg.Channels.WeCom.ReceiveID != "wecom-receive-id" {
		t.Errorf("wecom receiveId = %q, want wecom-receive-id", cfg.Channels.WeCom.ReceiveID)
	}
}

func TestLoadConfig_OpenAIEnvSetsOpenAICompatible(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	t.Setenv("JAMES_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	t.Setenv("JAMES_OPENAI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "openai-proxy-key")
	t.Setenv("JAMES_PROVIDER", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Provider.APIKey != "openai-proxy-key" {
		t.Errorf("apiKey = %q, want openai-proxy-key", cfg.Provider.APIKey)
	}
	if cfg.Provider.Type != "openai-compatible" {
		t.Errorf("provider type = %q, want openai-compatible", cfg.Provider.Type)
	}
}

func TestLoadConfig_OpenAIEnvUpdatesAnthropicTypeWhenNoKey(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfgDir := filepath.Join(tmpDir, ".james-agent")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configData := []byte(`{"provider":{"type":"anthropic","apiKey":""}}`)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), configData, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("JAMES_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	t.Setenv("JAMES_OPENAI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "proxy-key")
	t.Setenv("JAMES_PROVIDER", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Provider.Type != "openai-compatible" {
		t.Errorf("provider type = %q, want openai-compatible", cfg.Provider.Type)
	}
}

func TestLoadConfig_MyclawProviderPinsType(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	t.Setenv("JAMES_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	t.Setenv("JAMES_OPENAI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "proxy-key")
	t.Setenv("JAMES_PROVIDER", "anthropic")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Provider.Type != "anthropic" {
		t.Errorf("provider type = %q, want anthropic", cfg.Provider.Type)
	}
}

func TestLoadConfig_OpenAIBaseURLEnv(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	t.Setenv("JAMES_BASE_URL", "")
	t.Setenv("JAMES_OPENAI_BASE_URL", "https://proxy.example.com/v1")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("ANTHROPIC_BASE_URL", "")
	t.Setenv("JAMES_PROVIDER", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Provider.BaseURL != "https://proxy.example.com/v1" {
		t.Errorf("baseURL = %q, want https://proxy.example.com/v1", cfg.Provider.BaseURL)
	}
	if cfg.Provider.Type != "openai-compatible" {
		t.Errorf("provider type = %q, want openai-compatible", cfg.Provider.Type)
	}
}

func TestLoadConfig_MyclawModelEnv(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	t.Setenv("JAMES_MODEL", "grok-5")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Agent.Model != "grok-5" {
		t.Errorf("model = %q, want grok-5", cfg.Agent.Model)
	}
}

func TestLoadConfig_ThirdPartyEnvAutoDetect(t *testing.T) {
	tests := []struct {
		envVar       string
		envValue     string
		wantType     string
	}{
		{"DEEPSEEK_API_KEY", "sk-deepseek-test", "deepseek"},
		{"GROQ_API_KEY", "gsk_groq-test", "groq"},
		{"XAI_API_KEY", "xai-test-key", "xai"},
		{"MISTRAL_API_KEY", "mistral-test-key", "mistral"},
		{"SILICONFLOW_API_KEY", "sf-test-key", "siliconflow"},
	}
	for _, tt := range tests {
		t.Run(tt.envVar, func(t *testing.T) {
			tmpDir := t.TempDir()
			origHome := os.Getenv("HOME")
			t.Setenv("HOME", tmpDir)
			defer os.Setenv("HOME", origHome)

			t.Setenv("JAMES_API_KEY", "")
			t.Setenv("ANTHROPIC_API_KEY", "")
			t.Setenv("OPENAI_API_KEY", "")
			t.Setenv(tt.envVar, tt.envValue)

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig error: %v", err)
			}
			if cfg.Provider.APIKey != tt.envValue {
				t.Errorf("apiKey = %q, want %q", cfg.Provider.APIKey, tt.envValue)
			}
			if cfg.Provider.Type != tt.wantType {
				t.Errorf("provider type = %q, want %q", cfg.Provider.Type, tt.wantType)
			}
		})
	}
}

func TestLoadConfig_JamesAPIKeyOverridesThirdParty(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	t.Setenv("DEEPSEEK_API_KEY", "sk-deepseek")
	t.Setenv("JAMES_API_KEY", "sk-james-override")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Provider.APIKey != "sk-james-override" {
		t.Errorf("apiKey = %q, want sk-james-override (JAMES_API_KEY should take priority)", cfg.Provider.APIKey)
	}
}
