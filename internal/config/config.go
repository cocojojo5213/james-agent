package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultModel             = "claude-sonnet-4-5-20250929"
	DefaultMaxTokens         = 8192
	DefaultTemperature       = 0.7
	DefaultMaxToolIterations = 20
	DefaultExecTimeout       = 60
	DefaultHost              = "0.0.0.0"
	DefaultPort              = 18790
	DefaultBufSize           = 100
)

type Config struct {
	Agent         AgentConfig         `json:"agent"`
	Channels      ChannelsConfig      `json:"channels"`
	Provider      ProviderConfig      `json:"provider"`
	Tools         ToolsConfig         `json:"tools"`
	Skills        SkillsConfig        `json:"skills"`
	Hooks         HooksConfig         `json:"hooks"`
	MCP           MCPConfig           `json:"mcp"`
	AutoCompact   AutoCompactConfig   `json:"autoCompact"`
	TokenTracking TokenTrackingConfig `json:"tokenTracking"`
	Gateway       GatewayConfig       `json:"gateway"`
	Log           LogConfig           `json:"log"`
	Security      SecurityConfig      `json:"security"`
}

type AgentConfig struct {
	Workspace         string  `json:"workspace"`
	Model             string  `json:"model"`
	MaxTokens         int     `json:"maxTokens"`
	Temperature       float64 `json:"temperature"`
	MaxToolIterations int     `json:"maxToolIterations"`
}

type ProviderConfig struct {
	Type     string          `json:"type,omitempty"` // "anthropic" (default), "openai", or "openai-compatible"
	APIKey   string          `json:"apiKey"`
	BaseURL  string          `json:"baseUrl,omitempty"`
	Fallback *ProviderConfig `json:"fallback,omitempty"`
}

type ChannelsConfig struct {
	Telegram TelegramConfig `json:"telegram"`
	Feishu   FeishuConfig   `json:"feishu"`
	WeCom    WeComConfig    `json:"wecom"`
	WhatsApp WhatsAppConfig `json:"whatsapp"`
	WebUI    WebUIConfig    `json:"webui"`
}

type TelegramConfig struct {
	Enabled   bool     `json:"enabled"`
	Token     string   `json:"token"`
	AllowFrom []string `json:"allowFrom"`
	Proxy     string   `json:"proxy,omitempty"`
}

type FeishuConfig struct {
	Enabled           bool     `json:"enabled"`
	AppID             string   `json:"appId"`
	AppSecret         string   `json:"appSecret"`
	VerificationToken string   `json:"verificationToken"`
	EncryptKey        string   `json:"encryptKey,omitempty"`
	Port              int      `json:"port,omitempty"`
	AllowFrom         []string `json:"allowFrom"`
}

type WeComConfig struct {
	Enabled        bool     `json:"enabled"`
	Token          string   `json:"token"`
	EncodingAESKey string   `json:"encodingAESKey"`
	ReceiveID      string   `json:"receiveId,omitempty"`
	Port           int      `json:"port,omitempty"`
	AllowFrom      []string `json:"allowFrom"`
}

type ToolsConfig struct {
	BraveAPIKey         string `json:"braveApiKey,omitempty"`
	ExecTimeout         int    `json:"execTimeout"`
	RestrictToWorkspace bool   `json:"restrictToWorkspace"`
}

type GatewayConfig struct {
	Host          string  `json:"host"`
	Port          int     `json:"port"`
	MaxConcurrent int     `json:"maxConcurrent,omitempty"`
	RateLimit     float64 `json:"rateLimit,omitempty"`
	RateBurst     int     `json:"rateBurst,omitempty"`
}

type LogConfig struct {
	Level  string `json:"level,omitempty"`  // "debug", "info", "warn", "error"
	Format string `json:"format,omitempty"` // "text", "json"
}

type SecurityConfig struct {
	SandboxEnabled bool     `json:"sandboxEnabled,omitempty"`
	AllowedDirs    []string `json:"allowedDirs,omitempty"`
}

type SkillsConfig struct {
	Enabled bool   `json:"enabled"`
	Dir     string `json:"dir,omitempty"` // 默认 workspace/skills
}

type HooksConfig struct {
	PreToolUse  []HookEntry `json:"preToolUse,omitempty"`
	PostToolUse []HookEntry `json:"postToolUse,omitempty"`
	Stop        []HookEntry `json:"stop,omitempty"`
}

type HookEntry struct {
	Command string `json:"command"`
	Pattern string `json:"pattern,omitempty"` // tool name regex
	Timeout int    `json:"timeout,omitempty"` // seconds
}

type MCPConfig struct {
	Servers []string `json:"servers,omitempty"`
}

type WhatsAppConfig struct {
	Enabled   bool     `json:"enabled"`
	JID       string   `json:"jid,omitempty"`
	StorePath string   `json:"storePath,omitempty"`
	AllowFrom []string `json:"allowFrom,omitempty"`
}

type WebUIConfig struct {
	Enabled   bool     `json:"enabled"`
	AllowFrom []string `json:"allowFrom,omitempty"`
}

type AutoCompactConfig struct {
	Enabled       bool    `json:"enabled"`
	Threshold     float64 `json:"threshold,omitempty"`
	PreserveCount int     `json:"preserveCount,omitempty"`
}

type TokenTrackingConfig struct {
	Enabled bool `json:"enabled"`
}

func DefaultConfig() *Config {
	home := resolveHomeDir()
	return &Config{
		Agent: AgentConfig{
			Workspace:         filepath.Join(home, ".james-agent", "workspace"),
			Model:             DefaultModel,
			MaxTokens:         DefaultMaxTokens,
			Temperature:       DefaultTemperature,
			MaxToolIterations: DefaultMaxToolIterations,
		},
		Provider: ProviderConfig{},
		Channels: ChannelsConfig{},
		Tools: ToolsConfig{
			ExecTimeout:         DefaultExecTimeout,
			RestrictToWorkspace: true,
		},
		Skills: SkillsConfig{
			Enabled: true,
		},
		AutoCompact: AutoCompactConfig{
			Enabled:       true,
			Threshold:     0.8,
			PreserveCount: 5,
		},
		Gateway: GatewayConfig{
			Host: DefaultHost,
			Port: DefaultPort,
		},
	}
}

func ConfigDir() string {
	home := resolveHomeDir()
	return filepath.Join(home, ".james-agent")
}

func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.json")
}

func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	// Environment variable overrides
	providerTypeEnv := strings.TrimSpace(os.Getenv("JAMES_PROVIDER"))
	jamesAPIKeyEnv := os.Getenv("JAMES_API_KEY")
	jamesModelEnv := strings.TrimSpace(os.Getenv("JAMES_MODEL"))
	anthropicAPIKeyEnv := os.Getenv("ANTHROPIC_API_KEY")
	anthropicAuthTokenEnv := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	jamesOpenAIAPIKeyEnv := os.Getenv("JAMES_OPENAI_API_KEY")
	openAIAPIKeyEnv := os.Getenv("OPENAI_API_KEY")
	jamesBaseURLEnv := os.Getenv("JAMES_BASE_URL")
	jamesOpenAIBaseURLEnv := os.Getenv("JAMES_OPENAI_BASE_URL")
	openAIBaseURLEnv := os.Getenv("OPENAI_BASE_URL")
	anthropicBaseURLEnv := os.Getenv("ANTHROPIC_BASE_URL")

	providerTypePinned := false
	if providerTypeEnv != "" {
		cfg.Provider.Type = providerTypeEnv
		providerTypePinned = true
	}

	setOpenAICompatibleType := func() {
		if providerTypePinned {
			return
		}
		if cfg.Provider.Type == "" || strings.EqualFold(cfg.Provider.Type, "anthropic") {
			cfg.Provider.Type = "openai-compatible"
		}
	}

	openAIKeySelected := false
	if jamesAPIKeyEnv != "" {
		cfg.Provider.APIKey = jamesAPIKeyEnv
	}
	if anthropicAPIKeyEnv != "" && cfg.Provider.APIKey == "" {
		cfg.Provider.APIKey = anthropicAPIKeyEnv
	}
	if anthropicAuthTokenEnv != "" && cfg.Provider.APIKey == "" {
		cfg.Provider.APIKey = anthropicAuthTokenEnv
	}
	if jamesOpenAIAPIKeyEnv != "" && cfg.Provider.APIKey == "" {
		cfg.Provider.APIKey = jamesOpenAIAPIKeyEnv
		openAIKeySelected = true
		setOpenAICompatibleType()
	}
	if openAIAPIKeyEnv != "" && cfg.Provider.APIKey == "" {
		cfg.Provider.APIKey = openAIAPIKeyEnv
		openAIKeySelected = true
		setOpenAICompatibleType()
	}
	if jamesBaseURLEnv != "" {
		cfg.Provider.BaseURL = jamesBaseURLEnv
	}
	if jamesOpenAIBaseURLEnv != "" && cfg.Provider.BaseURL == "" {
		cfg.Provider.BaseURL = jamesOpenAIBaseURLEnv
		setOpenAICompatibleType()
	}
	if openAIBaseURLEnv != "" &&
		cfg.Provider.BaseURL == "" &&
		(openAIKeySelected || strings.EqualFold(cfg.Provider.Type, "openai") || strings.EqualFold(cfg.Provider.Type, "openai-compatible")) {
		cfg.Provider.BaseURL = openAIBaseURLEnv
		setOpenAICompatibleType()
	}
	if anthropicBaseURLEnv != "" && cfg.Provider.BaseURL == "" {
		cfg.Provider.BaseURL = anthropicBaseURLEnv
	}
	if token := os.Getenv("JAMES_TELEGRAM_TOKEN"); token != "" {
		cfg.Channels.Telegram.Token = token
	}
	if appID := os.Getenv("JAMES_FEISHU_APP_ID"); appID != "" {
		cfg.Channels.Feishu.AppID = appID
	}
	if appSecret := os.Getenv("JAMES_FEISHU_APP_SECRET"); appSecret != "" {
		cfg.Channels.Feishu.AppSecret = appSecret
	}
	if token := os.Getenv("JAMES_WECOM_TOKEN"); token != "" {
		cfg.Channels.WeCom.Token = token
	}
	if aesKey := os.Getenv("JAMES_WECOM_ENCODING_AES_KEY"); aesKey != "" {
		cfg.Channels.WeCom.EncodingAESKey = aesKey
	}
	if receiveID := os.Getenv("JAMES_WECOM_RECEIVE_ID"); receiveID != "" {
		cfg.Channels.WeCom.ReceiveID = receiveID
	}
	if jamesModelEnv != "" {
		cfg.Agent.Model = jamesModelEnv
	}

	if cfg.Agent.Workspace == "" {
		cfg.Agent.Workspace = DefaultConfig().Agent.Workspace
	}

	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(ConfigPath(), data, 0644)
}

func resolveHomeDir() string {
	if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
		return home
	}
	home, _ := os.UserHomeDir()
	return home
}
