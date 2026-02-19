package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/everstacklabs/sentinel/internal/adapter"
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/ai21"        // register AI21 adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/alibaba"     // register Alibaba adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/anthropic"   // register Anthropic adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/bailing"     // register Bailing adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/cerebras"    // register Cerebras adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/cohere"      // register Cohere adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/deepinfra"   // register DeepInfra adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/deepseek"    // register DeepSeek adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/fireworks"   // register Fireworks adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/friendli"    // register Friendli adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/google"      // register Google adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/groq"        // register Groq adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/inception"   // register Inception adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/llama"       // register Meta Llama adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/minimax"     // register MiniMax adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/mistral"     // register Mistral adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/moonshotai"  // register Moonshot AI adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/nebius"      // register Nebius adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/nova"        // register Amazon Nova adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/novitaai"    // register Novita AI adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/nvidia"      // register NVIDIA adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/openai"      // register OpenAI adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/perplexity"  // register Perplexity adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/siliconflow" // register SiliconFlow adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/stepfun"     // register StepFun adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/togetherai"  // register Together AI adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/upstage"     // register Upstage adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/venice"      // register Venice adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/xai"         // register xAI adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/zhipuai"     // register Zhipu AI adapter
	"github.com/everstacklabs/sentinel/internal/cache"
	"github.com/everstacklabs/sentinel/internal/catalog"
	"github.com/everstacklabs/sentinel/internal/config"
	"github.com/everstacklabs/sentinel/internal/diff"
	"github.com/everstacklabs/sentinel/internal/httpclient"
	"github.com/everstacklabs/sentinel/internal/pipeline"
	"github.com/everstacklabs/sentinel/internal/validate"

	ai21Adapter "github.com/everstacklabs/sentinel/internal/adapter/providers/ai21"
	alibabaAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/alibaba"
	anthropicAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/anthropic"
	bailingAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/bailing"
	cerebrasAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/cerebras"
	cohereAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/cohere"
	deepinfraAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/deepinfra"
	deepseekAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/deepseek"
	fireworksAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/fireworks"
	friendliAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/friendli"
	googleAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/google"
	groqAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/groq"
	inceptionAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/inception"
	llamaAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/llama"
	minimaxAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/minimax"
	mistralAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/mistral"
	moonshotaiAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/moonshotai"
	nebiusAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/nebius"
	novaAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/nova"
	novitaaiAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/novitaai"
	nvidiaAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/nvidia"
	openaiAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/openai"
	perplexityAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/perplexity"
	siliconflowAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/siliconflow"
	stepfunAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/stepfun"
	togetheraiAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/togetherai"
	upstageAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/upstage"
	veniceAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/venice"
	xaiAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/xai"
	zhipuaiAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/zhipuai"
)

var cfgFile string

func main() {
	rootCmd := &cobra.Command{
		Use:   "sentinel",
		Short: "Keeps your AI model catalog in sync with reality.",
		Long:  "An open-source tool that discovers AI models from provider APIs and opens PRs to keep your catalog up to date.",
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./config.yaml)")

	rootCmd.AddCommand(
		syncCmd(),
		diffCmd(),
		discoverCmd(),
		validateCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func syncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Full pipeline: discover → diff → validate → write → PR",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			configureAdapters(cfg)

			p := pipeline.New(cfg)
			results, err := p.Sync(cmd.Context())
			if err != nil {
				return err
			}

			for _, r := range results {
				if r.Error != nil {
					slog.Error("sync failed", "provider", r.Provider, "error", r.Error)
				} else if r.Skipped {
					slog.Info("sync skipped", "provider", r.Provider, "reason", r.SkipReason)
				} else if r.PRNumber > 0 {
					slog.Info("PR created", "provider", r.Provider, "pr", r.PRNumber, "draft", r.PRDraft)
				} else {
					slog.Info("sync complete", "provider", r.Provider)
				}
			}

			return nil
		},
	}

	cmd.Flags().Bool("dry-run", false, "Show what would change without writing")
	cmd.Flags().StringSlice("providers", nil, "Providers to sync (default: all configured)")

	return cmd
}

func diffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "Show what would change (no writes)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			configureAdapters(cfg)

			p := pipeline.New(cfg)
			changesets, err := p.Diff(cmd.Context())
			if err != nil {
				return err
			}

			hasChanges := false
			for _, cs := range changesets {
				fmt.Println(diff.RenderDiffSummary(&cs))
				if cs.HasChanges() {
					hasChanges = true
				}
			}

			if hasChanges {
				os.Exit(pipeline.ExitChanges)
			}
			return nil
		},
	}
}

func discoverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discovery only, print models to stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			configureAdapters(cfg)

			provider, _ := cmd.Flags().GetString("provider")
			if provider == "" {
				return fmt.Errorf("--provider is required")
			}

			a, err := adapter.Get(provider)
			if err != nil {
				return err
			}

			sources := make([]adapter.SourceType, 0, len(cfg.Sources))
			for _, s := range cfg.Sources {
				sources = append(sources, adapter.SourceType(s))
			}

			models, err := a.Discover(cmd.Context(), adapter.DiscoverOptions{
				Sources:  sources,
				NoCache:  cfg.NoCache,
				CacheDir: cfg.CacheDir,
			})
			if err != nil {
				return err
			}

			for _, m := range models {
				fmt.Printf("%-40s %-20s %-10s %s\n", m.Name, m.Family, m.Status, m.DiscoveredBy)
			}

			fmt.Printf("\nTotal: %d models\n", len(models))
			return nil
		},
	}

	cmd.Flags().String("provider", "", "Provider to discover models from")
	_ = cmd.MarkFlagRequired("provider")

	return cmd
}

func validateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate existing catalog (CI check)",
		RunE: func(cmd *cobra.Command, args []string) error {
			catalogPath, _ := cmd.Flags().GetString("catalog-path")
			if catalogPath == "" {
				cfg, err := loadConfig()
				if err != nil {
					return err
				}
				catalogPath = cfg.CatalogPath
			}

			cat, err := catalog.Load(catalogPath)
			if err != nil {
				return fmt.Errorf("loading catalog: %w", err)
			}

			result := validate.ValidateCatalog(cat)
			fmt.Println(validate.FormatResult(result))

			if result.HasErrors() {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().String("catalog-path", "", "Path to model catalog (default: from config)")

	return cmd
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return cfg, nil
}

func configureAdapters(cfg *config.Config) {
	// Set up cache
	var fileCache *cache.FileCache
	if !cfg.NoCache {
		ttl, err := time.ParseDuration(cfg.CacheTTL)
		if err != nil {
			ttl = time.Hour
		}
		fc, err := cache.New(cfg.CacheDir, ttl)
		if err != nil {
			slog.Warn("failed to create cache, continuing without", "error", err)
		} else {
			fileCache = fc
		}
	}

	// Set up HTTP client
	opts := []httpclient.Option{
		httpclient.WithRateLimit(10), // 10 RPS default
	}
	if fileCache != nil {
		opts = append(opts, httpclient.WithCache(fileCache))
	}
	if cfg.NoCache {
		opts = append(opts, httpclient.WithNoCache())
	}
	client := httpclient.New(opts...)

	// Configure OpenAI adapter
	if a, err := adapter.Get("openai"); err == nil {
		if oa, ok := a.(*openaiAdapter.OpenAI); ok {
			apiKey := cfg.OpenAI.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("OPENAI_API_KEY")
			}
			oa.Configure(apiKey, cfg.OpenAI.BaseURL, client)
		}
	}

	// Configure Anthropic adapter
	if a, err := adapter.Get("anthropic"); err == nil {
		if aa, ok := a.(*anthropicAdapter.Anthropic); ok {
			apiKey := cfg.Anthropic.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("ANTHROPIC_API_KEY")
			}
			aa.Configure(apiKey, cfg.Anthropic.BaseURL, client)
		}
	}

	// Configure Google adapter
	if a, err := adapter.Get("google"); err == nil {
		if ga, ok := a.(*googleAdapter.Google); ok {
			apiKey := cfg.Google.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("GEMINI_API_KEY")
			}
			ga.Configure(apiKey, cfg.Google.BaseURL, client)
		}
	}

	// Configure Mistral adapter
	if a, err := adapter.Get("mistral"); err == nil {
		if ma, ok := a.(*mistralAdapter.Mistral); ok {
			apiKey := cfg.Mistral.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("MISTRAL_API_KEY")
			}
			ma.Configure(apiKey, cfg.Mistral.BaseURL, client)
		}
	}

	// Configure Cohere adapter
	if a, err := adapter.Get("cohere"); err == nil {
		if ca, ok := a.(*cohereAdapter.Cohere); ok {
			apiKey := cfg.Cohere.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("COHERE_API_KEY")
			}
			ca.Configure(apiKey, cfg.Cohere.BaseURL, client)
		}
	}

	// Configure Groq adapter
	if a, err := adapter.Get("groq"); err == nil {
		if ga, ok := a.(*groqAdapter.Groq); ok {
			apiKey := cfg.Groq.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("GROQ_API_KEY")
			}
			ga.Configure(apiKey, cfg.Groq.BaseURL, client)
		}
	}

	// Configure DeepSeek adapter
	if a, err := adapter.Get("deepseek"); err == nil {
		if da, ok := a.(*deepseekAdapter.DeepSeek); ok {
			apiKey := cfg.DeepSeek.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("DEEPSEEK_API_KEY")
			}
			da.Configure(apiKey, cfg.DeepSeek.BaseURL, client)
		}
	}

	// Configure xAI adapter
	if a, err := adapter.Get("xai"); err == nil {
		if xa, ok := a.(*xaiAdapter.XAI); ok {
			apiKey := cfg.XAI.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("XAI_API_KEY")
			}
			xa.Configure(apiKey, cfg.XAI.BaseURL, client)
		}
	}

	// Configure Together AI adapter
	if a, err := adapter.Get("togetherai"); err == nil {
		if ta, ok := a.(*togetheraiAdapter.TogetherAI); ok {
			apiKey := cfg.TogetherAI.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("TOGETHER_API_KEY")
			}
			ta.Configure(apiKey, cfg.TogetherAI.BaseURL, client)
		}
	}

	// Configure Cerebras adapter
	if a, err := adapter.Get("cerebras"); err == nil {
		if ca, ok := a.(*cerebrasAdapter.Cerebras); ok {
			apiKey := cfg.Cerebras.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("CEREBRAS_API_KEY")
			}
			ca.Configure(apiKey, cfg.Cerebras.BaseURL, client)
		}
	}

	// Configure Fireworks adapter
	if a, err := adapter.Get("fireworks"); err == nil {
		if fa, ok := a.(*fireworksAdapter.Fireworks); ok {
			apiKey := cfg.Fireworks.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("FIREWORKS_API_KEY")
			}
			fa.Configure(apiKey, cfg.Fireworks.BaseURL, client)
		}
	}

	// Configure DeepInfra adapter
	if a, err := adapter.Get("deepinfra"); err == nil {
		if da, ok := a.(*deepinfraAdapter.DeepInfra); ok {
			apiKey := cfg.DeepInfra.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("DEEPINFRA_API_KEY")
			}
			da.Configure(apiKey, cfg.DeepInfra.BaseURL, client)
		}
	}

	// Configure NVIDIA adapter
	if a, err := adapter.Get("nvidia"); err == nil {
		if na, ok := a.(*nvidiaAdapter.NVIDIA); ok {
			apiKey := cfg.NVIDIA.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("NVIDIA_API_KEY")
			}
			na.Configure(apiKey, cfg.NVIDIA.BaseURL, client)
		}
	}

	// Configure Alibaba adapter
	if a, err := adapter.Get("alibaba"); err == nil {
		if aa, ok := a.(*alibabaAdapter.Alibaba); ok {
			apiKey := cfg.Alibaba.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("DASHSCOPE_API_KEY")
			}
			aa.Configure(apiKey, cfg.Alibaba.BaseURL, client)
		}
	}

	// Configure MiniMax adapter
	if a, err := adapter.Get("minimax"); err == nil {
		if ma, ok := a.(*minimaxAdapter.MiniMax); ok {
			apiKey := cfg.MiniMax.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("MINIMAX_API_KEY")
			}
			ma.Configure(apiKey, cfg.MiniMax.BaseURL, client)
		}
	}

	// Configure Moonshot AI adapter
	if a, err := adapter.Get("moonshotai"); err == nil {
		if ma, ok := a.(*moonshotaiAdapter.MoonshotAI); ok {
			apiKey := cfg.MoonshotAI.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("MOONSHOT_API_KEY")
			}
			ma.Configure(apiKey, cfg.MoonshotAI.BaseURL, client)
		}
	}

	// Configure Nebius adapter
	if a, err := adapter.Get("nebius"); err == nil {
		if na, ok := a.(*nebiusAdapter.Nebius); ok {
			apiKey := cfg.Nebius.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("NEBIUS_API_KEY")
			}
			na.Configure(apiKey, cfg.Nebius.BaseURL, client)
		}
	}

	// Configure SiliconFlow adapter
	if a, err := adapter.Get("siliconflow"); err == nil {
		if sa, ok := a.(*siliconflowAdapter.SiliconFlow); ok {
			apiKey := cfg.SiliconFlow.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("SILICONFLOW_API_KEY")
			}
			sa.Configure(apiKey, cfg.SiliconFlow.BaseURL, client)
		}
	}

	// Configure Inception adapter
	if a, err := adapter.Get("inception"); err == nil {
		if ia, ok := a.(*inceptionAdapter.Inception); ok {
			apiKey := cfg.Inception.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("INCEPTION_API_KEY")
			}
			ia.Configure(apiKey, cfg.Inception.BaseURL, client)
		}
	}

	// Configure Meta Llama adapter
	if a, err := adapter.Get("llama"); err == nil {
		if la, ok := a.(*llamaAdapter.Llama); ok {
			apiKey := cfg.Llama.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("LLAMA_API_KEY")
			}
			la.Configure(apiKey, cfg.Llama.BaseURL, client)
		}
	}

	// Configure Upstage adapter
	if a, err := adapter.Get("upstage"); err == nil {
		if ua, ok := a.(*upstageAdapter.Upstage); ok {
			apiKey := cfg.Upstage.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("UPSTAGE_API_KEY")
			}
			ua.Configure(apiKey, cfg.Upstage.BaseURL, client)
		}
	}

	// Configure Amazon Nova adapter
	if a, err := adapter.Get("nova"); err == nil {
		if na, ok := a.(*novaAdapter.Nova); ok {
			apiKey := cfg.Nova.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("NOVA_API_KEY")
			}
			na.Configure(apiKey, cfg.Nova.BaseURL, client)
		}
	}

	// Configure Novita AI adapter
	if a, err := adapter.Get("novitaai"); err == nil {
		if na, ok := a.(*novitaaiAdapter.NovitaAI); ok {
			apiKey := cfg.NovitaAI.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("NOVITA_API_KEY")
			}
			na.Configure(apiKey, cfg.NovitaAI.BaseURL, client)
		}
	}

	// Configure Friendli adapter
	if a, err := adapter.Get("friendli"); err == nil {
		if fa, ok := a.(*friendliAdapter.Friendli); ok {
			apiKey := cfg.Friendli.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("FRIENDLI_TOKEN")
			}
			fa.Configure(apiKey, cfg.Friendli.BaseURL, client)
		}
	}

	// Configure StepFun adapter
	if a, err := adapter.Get("stepfun"); err == nil {
		if sa, ok := a.(*stepfunAdapter.StepFun); ok {
			apiKey := cfg.StepFun.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("STEPFUN_API_KEY")
			}
			sa.Configure(apiKey, cfg.StepFun.BaseURL, client)
		}
	}

	// Configure Zhipu AI adapter
	if a, err := adapter.Get("zhipuai"); err == nil {
		if za, ok := a.(*zhipuaiAdapter.ZhipuAI); ok {
			apiKey := cfg.ZhipuAI.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("ZHIPU_API_KEY")
			}
			za.Configure(apiKey, cfg.ZhipuAI.BaseURL, client)
		}
	}

	// Configure Venice adapter
	if a, err := adapter.Get("venice"); err == nil {
		if va, ok := a.(*veniceAdapter.Venice); ok {
			apiKey := cfg.Venice.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("VENICE_API_KEY")
			}
			va.Configure(apiKey, cfg.Venice.BaseURL, client)
		}
	}

	// Configure Bailing adapter
	if a, err := adapter.Get("bailing"); err == nil {
		if ba, ok := a.(*bailingAdapter.Bailing); ok {
			apiKey := cfg.Bailing.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("BAILING_API_TOKEN")
			}
			ba.Configure(apiKey, cfg.Bailing.BaseURL, client)
		}
	}

	// Configure docs-only adapters (no API key needed)
	if a, err := adapter.Get("perplexity"); err == nil {
		if pa, ok := a.(*perplexityAdapter.Perplexity); ok {
			pa.Configure(client)
		}
	}
	if a, err := adapter.Get("ai21"); err == nil {
		if aa, ok := a.(*ai21Adapter.AI21); ok {
			aa.Configure(client)
		}
	}
}

func init() {
	// Suppress unused import errors
	_ = context.Background
}
