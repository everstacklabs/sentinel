package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all configuration for the sentinel.
type Config struct {
	CatalogPath string           `mapstructure:"catalog_path"`
	CacheDir    string           `mapstructure:"cache_dir"`
	CacheTTL    string           `mapstructure:"cache_ttl"`
	Providers   []string         `mapstructure:"providers"`
	Sources     []string         `mapstructure:"sources"`
	DryRun      bool             `mapstructure:"dry_run"`
	NoCache     bool             `mapstructure:"no_cache"`
	RiskMode    string           `mapstructure:"risk_mode"`
	GitHub      GitHubConfig     `mapstructure:"github"`
	OpenAI      OpenAIConfig     `mapstructure:"openai"`
	Anthropic   AnthropicConfig  `mapstructure:"anthropic"`
	Google      GoogleConfig     `mapstructure:"google"`
	Mistral     MistralConfig    `mapstructure:"mistral"`
	Judge       JudgeConfig      `mapstructure:"judge"`
	LogLevel    string           `mapstructure:"log_level"`
}

// GitHubConfig holds GitHub-related settings.
type GitHubConfig struct {
	Token      string `mapstructure:"token"`
	Owner      string `mapstructure:"owner"`
	Repo       string `mapstructure:"repo"`
	BaseBranch string `mapstructure:"base_branch"`
}

// OpenAIConfig holds OpenAI-specific settings.
type OpenAIConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// AnthropicConfig holds Anthropic-specific settings.
type AnthropicConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// GoogleConfig holds Google/Gemini-specific settings.
type GoogleConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// MistralConfig holds Mistral-specific settings.
type MistralConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// JudgeConfig holds LLM-as-judge settings.
type JudgeConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Provider  string `mapstructure:"provider"`
	Model     string `mapstructure:"model"`
	OnReject  string `mapstructure:"on_reject"`
	MaxTokens int    `mapstructure:"max_tokens"`
}

// Load reads configuration from file, environment, and defaults.
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("catalog_path", "../model-catalog")
	v.SetDefault("cache_dir", defaultCacheDir())
	v.SetDefault("cache_ttl", "1h")
	v.SetDefault("providers", []string{"openai"})
	v.SetDefault("sources", []string{"api"})
	v.SetDefault("dry_run", false)
	v.SetDefault("no_cache", false)
	v.SetDefault("risk_mode", "strict")
	v.SetDefault("log_level", "info")
	v.SetDefault("github.base_branch", "main")
	v.SetDefault("openai.base_url", "https://api.openai.com/v1")
	v.SetDefault("anthropic.base_url", "https://api.anthropic.com/v1")
	v.SetDefault("google.base_url", "https://generativelanguage.googleapis.com/v1beta")
	v.SetDefault("mistral.base_url", "https://api.mistral.ai/v1")
	v.SetDefault("judge.enabled", false)
	v.SetDefault("judge.provider", "anthropic")
	v.SetDefault("judge.model", "claude-sonnet-4-20250514")
	v.SetDefault("judge.on_reject", "draft")
	v.SetDefault("judge.max_tokens", 4096)

	// Config file
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.config/sentinel")
	}

	// Environment variables
	v.SetEnvPrefix("SENTINEL")
	v.AutomaticEnv()

	// Bind specific env vars
	_ = v.BindEnv("github.token", "GITHUB_TOKEN")
	_ = v.BindEnv("openai.api_key", "OPENAI_API_KEY")
	_ = v.BindEnv("anthropic.api_key", "ANTHROPIC_API_KEY")
	_ = v.BindEnv("anthropic.base_url", "SENTINEL_ANTHROPIC_BASE_URL")
	_ = v.BindEnv("google.api_key", "GEMINI_API_KEY")
	_ = v.BindEnv("google.base_url", "SENTINEL_GOOGLE_BASE_URL")
	_ = v.BindEnv("mistral.api_key", "MISTRAL_API_KEY")
	_ = v.BindEnv("mistral.base_url", "SENTINEL_MISTRAL_BASE_URL")
	_ = v.BindEnv("judge.enabled", "SENTINEL_JUDGE_ENABLED")
	_ = v.BindEnv("judge.provider", "SENTINEL_JUDGE_PROVIDER")
	_ = v.BindEnv("judge.model", "SENTINEL_JUDGE_MODEL")
	_ = v.BindEnv("judge.on_reject", "SENTINEL_JUDGE_ON_REJECT")
	_ = v.BindEnv("judge.max_tokens", "SENTINEL_JUDGE_MAX_TOKENS")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Resolve catalog path to absolute
	if !filepath.IsAbs(cfg.CatalogPath) {
		abs, err := filepath.Abs(cfg.CatalogPath)
		if err != nil {
			return nil, fmt.Errorf("resolving catalog path: %w", err)
		}
		cfg.CatalogPath = abs
	}

	return &cfg, nil
}

func defaultCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/sentinel-cache"
	}
	return filepath.Join(home, ".cache", "sentinel")
}
