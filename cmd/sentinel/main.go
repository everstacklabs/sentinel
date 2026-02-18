package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/everstacklabs/sentinel/internal/adapter"
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/ai21"      // register AI21 adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/anthropic" // register Anthropic adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/google"    // register Google adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/mistral"   // register Mistral adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/openai"    // register OpenAI adapter
	_ "github.com/everstacklabs/sentinel/internal/adapter/providers/perplexity" // register Perplexity adapter
	"github.com/everstacklabs/sentinel/internal/cache"
	"github.com/everstacklabs/sentinel/internal/catalog"
	"github.com/everstacklabs/sentinel/internal/config"
	"github.com/everstacklabs/sentinel/internal/diff"
	"github.com/everstacklabs/sentinel/internal/httpclient"
	"github.com/everstacklabs/sentinel/internal/pipeline"
	"github.com/everstacklabs/sentinel/internal/validate"

	ai21Adapter "github.com/everstacklabs/sentinel/internal/adapter/providers/ai21"
	anthropicAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/anthropic"
	googleAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/google"
	mistralAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/mistral"
	openaiAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/openai"
	perplexityAdapter "github.com/everstacklabs/sentinel/internal/adapter/providers/perplexity"
)

var cfgFile string

func main() {
	rootCmd := &cobra.Command{
		Use:   "sentinel",
		Short: "Automated model catalog updater",
		Long:  "Discovers models from provider APIs and opens PRs to update the model catalog.",
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
