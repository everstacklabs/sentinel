package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all configuration for the sentinel.
type Config struct {
	CatalogPath string          `mapstructure:"catalog_path"`
	CacheDir    string          `mapstructure:"cache_dir"`
	CacheTTL    string          `mapstructure:"cache_ttl"`
	Providers   []string        `mapstructure:"providers"`
	Sources     []string        `mapstructure:"sources"`
	DryRun      bool            `mapstructure:"dry_run"`
	NoCache     bool            `mapstructure:"no_cache"`
	RiskMode    string          `mapstructure:"risk_mode"`
	GitHub      GitHubConfig    `mapstructure:"github"`
	OpenAI      OpenAIConfig    `mapstructure:"openai"`
	Anthropic   AnthropicConfig `mapstructure:"anthropic"`
	Google      GoogleConfig    `mapstructure:"google"`
	Mistral     MistralConfig   `mapstructure:"mistral"`
	Cohere      CohereConfig    `mapstructure:"cohere"`
	Groq        GroqConfig      `mapstructure:"groq"`
	DeepSeek    DeepSeekConfig  `mapstructure:"deepseek"`
	XAI         XAIConfig       `mapstructure:"xai"`
	TogetherAI  TogetherAIConfig  `mapstructure:"togetherai"`
	Cerebras    CerebrasConfig   `mapstructure:"cerebras"`
	Fireworks   FireworksConfig  `mapstructure:"fireworks"`
	DeepInfra   DeepInfraConfig  `mapstructure:"deepinfra"`
	NVIDIA      NVIDIAConfig     `mapstructure:"nvidia"`
	Alibaba     AlibabaConfig    `mapstructure:"alibaba"`
	MiniMax     MiniMaxConfig    `mapstructure:"minimax"`
	MoonshotAI  MoonshotAIConfig `mapstructure:"moonshotai"`
	Nebius      NebiusConfig     `mapstructure:"nebius"`
	SiliconFlow SiliconFlowConfig `mapstructure:"siliconflow"`
	Inception   InceptionConfig  `mapstructure:"inception"`
	Llama       LlamaConfig      `mapstructure:"llama"`
	Upstage     UpstageConfig    `mapstructure:"upstage"`
	Nova        NovaConfig       `mapstructure:"nova"`
	NovitaAI    NovitaAIConfig   `mapstructure:"novitaai"`
	Friendli    FriendliConfig   `mapstructure:"friendli"`
	StepFun     StepFunConfig    `mapstructure:"stepfun"`
	ZhipuAI     ZhipuAIConfig    `mapstructure:"zhipuai"`
	Venice      VeniceConfig     `mapstructure:"venice"`
	Bailing     BailingConfig    `mapstructure:"bailing"`
	Judge       JudgeConfig      `mapstructure:"judge"`
	Diff        DiffConfig      `mapstructure:"diff"`
	Health      HealthConfig    `mapstructure:"health"`
	LogLevel    string          `mapstructure:"log_level"`
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

// CohereConfig holds Cohere-specific settings.
type CohereConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// GroqConfig holds Groq-specific settings.
type GroqConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// DeepSeekConfig holds DeepSeek-specific settings.
type DeepSeekConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// XAIConfig holds xAI (Grok)-specific settings.
type XAIConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// TogetherAIConfig holds Together AI-specific settings.
type TogetherAIConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// CerebrasConfig holds Cerebras-specific settings.
type CerebrasConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// FireworksConfig holds Fireworks AI-specific settings.
type FireworksConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// DeepInfraConfig holds DeepInfra-specific settings.
type DeepInfraConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// NVIDIAConfig holds NVIDIA NIM-specific settings.
type NVIDIAConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// AlibabaConfig holds Alibaba/DashScope-specific settings.
type AlibabaConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// MiniMaxConfig holds MiniMax-specific settings.
type MiniMaxConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// MoonshotAIConfig holds Moonshot AI-specific settings.
type MoonshotAIConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// NebiusConfig holds Nebius-specific settings.
type NebiusConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// SiliconFlowConfig holds SiliconFlow-specific settings.
type SiliconFlowConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// InceptionConfig holds Inception Labs-specific settings.
type InceptionConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// LlamaConfig holds Meta Llama API-specific settings.
type LlamaConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// UpstageConfig holds Upstage-specific settings.
type UpstageConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// NovaConfig holds Amazon Nova-specific settings.
type NovaConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// NovitaAIConfig holds Novita AI-specific settings.
type NovitaAIConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// FriendliConfig holds Friendli-specific settings.
type FriendliConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// StepFunConfig holds StepFun-specific settings.
type StepFunConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// ZhipuAIConfig holds Zhipu AI-specific settings.
type ZhipuAIConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// VeniceConfig holds Venice AI-specific settings.
type VeniceConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// BailingConfig holds Bailing-specific settings.
type BailingConfig struct {
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

// DiffConfig holds diff behavior settings.
type DiffConfig struct {
	TrackDisplayName bool `mapstructure:"track_display_name"`
}

// HealthConfig holds source health check settings.
type HealthConfig struct {
	Enabled   bool    `mapstructure:"enabled"`
	Threshold float64 `mapstructure:"threshold"`
}

// Load reads configuration from file, environment, and defaults.
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("catalog_path", "../model-catalog")
	v.SetDefault("cache_dir", defaultCacheDir())
	v.SetDefault("cache_ttl", "1h")
	v.SetDefault("providers", []string{"openai"})
	v.SetDefault("sources", []string{"api", "docs"})
	v.SetDefault("dry_run", false)
	v.SetDefault("no_cache", false)
	v.SetDefault("risk_mode", "strict")
	v.SetDefault("log_level", "info")
	v.SetDefault("github.base_branch", "main")
	v.SetDefault("openai.base_url", "https://api.openai.com/v1")
	v.SetDefault("anthropic.base_url", "https://api.anthropic.com/v1")
	v.SetDefault("google.base_url", "https://generativelanguage.googleapis.com/v1beta")
	v.SetDefault("mistral.base_url", "https://api.mistral.ai/v1")
	v.SetDefault("cohere.base_url", "https://api.cohere.com/v2")
	v.SetDefault("groq.base_url", "https://api.groq.com/openai/v1")
	v.SetDefault("deepseek.base_url", "https://api.deepseek.com")
	v.SetDefault("xai.base_url", "https://api.x.ai/v1")
	v.SetDefault("togetherai.base_url", "https://api.together.xyz/v1")
	v.SetDefault("cerebras.base_url", "https://api.cerebras.ai/v1")
	v.SetDefault("fireworks.base_url", "https://api.fireworks.ai/inference/v1")
	v.SetDefault("deepinfra.base_url", "https://api.deepinfra.com/v1/openai")
	v.SetDefault("nvidia.base_url", "https://integrate.api.nvidia.com/v1")
	v.SetDefault("alibaba.base_url", "https://dashscope-intl.aliyuncs.com/compatible-mode/v1")
	v.SetDefault("minimax.base_url", "https://api.minimax.io/v1")
	v.SetDefault("moonshotai.base_url", "https://api.moonshot.ai/v1")
	v.SetDefault("nebius.base_url", "https://api.tokenfactory.nebius.com/v1")
	v.SetDefault("siliconflow.base_url", "https://api.siliconflow.com/v1")
	v.SetDefault("inception.base_url", "https://api.inceptionlabs.ai/v1")
	v.SetDefault("llama.base_url", "https://api.llama.com/compat/v1")
	v.SetDefault("upstage.base_url", "https://api.upstage.ai/v1/solar")
	v.SetDefault("nova.base_url", "https://api.nova.amazon.com/v1")
	v.SetDefault("novitaai.base_url", "https://api.novita.ai/openai")
	v.SetDefault("friendli.base_url", "https://api.friendli.ai/serverless/v1")
	v.SetDefault("stepfun.base_url", "https://api.stepfun.com/v1")
	v.SetDefault("zhipuai.base_url", "https://open.bigmodel.cn/api/paas/v4")
	v.SetDefault("venice.base_url", "https://api.venice.ai/api/v1")
	v.SetDefault("bailing.base_url", "https://api.tbox.cn/api/llm/v1")
	v.SetDefault("diff.track_display_name", false)
	v.SetDefault("health.enabled", true)
	v.SetDefault("health.threshold", 0.90)
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
	_ = v.BindEnv("cohere.api_key", "COHERE_API_KEY")
	_ = v.BindEnv("groq.api_key", "GROQ_API_KEY")
	_ = v.BindEnv("deepseek.api_key", "DEEPSEEK_API_KEY")
	_ = v.BindEnv("xai.api_key", "XAI_API_KEY")
	_ = v.BindEnv("togetherai.api_key", "TOGETHER_API_KEY")
	_ = v.BindEnv("cerebras.api_key", "CEREBRAS_API_KEY")
	_ = v.BindEnv("fireworks.api_key", "FIREWORKS_API_KEY")
	_ = v.BindEnv("deepinfra.api_key", "DEEPINFRA_API_KEY")
	_ = v.BindEnv("nvidia.api_key", "NVIDIA_API_KEY")
	_ = v.BindEnv("alibaba.api_key", "DASHSCOPE_API_KEY")
	_ = v.BindEnv("minimax.api_key", "MINIMAX_API_KEY")
	_ = v.BindEnv("moonshotai.api_key", "MOONSHOT_API_KEY")
	_ = v.BindEnv("nebius.api_key", "NEBIUS_API_KEY")
	_ = v.BindEnv("siliconflow.api_key", "SILICONFLOW_API_KEY")
	_ = v.BindEnv("inception.api_key", "INCEPTION_API_KEY")
	_ = v.BindEnv("llama.api_key", "LLAMA_API_KEY")
	_ = v.BindEnv("upstage.api_key", "UPSTAGE_API_KEY")
	_ = v.BindEnv("nova.api_key", "NOVA_API_KEY")
	_ = v.BindEnv("novitaai.api_key", "NOVITA_API_KEY")
	_ = v.BindEnv("friendli.api_key", "FRIENDLI_TOKEN")
	_ = v.BindEnv("stepfun.api_key", "STEPFUN_API_KEY")
	_ = v.BindEnv("zhipuai.api_key", "ZHIPU_API_KEY")
	_ = v.BindEnv("venice.api_key", "VENICE_API_KEY")
	_ = v.BindEnv("bailing.api_key", "BAILING_API_TOKEN")
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
