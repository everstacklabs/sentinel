package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/catalog"
	"github.com/everstacklabs/sentinel/internal/config"
	"github.com/everstacklabs/sentinel/internal/diff"
	"github.com/everstacklabs/sentinel/internal/judge"
	"github.com/everstacklabs/sentinel/internal/validate"
)

// ExitCode constants for CLI.
const (
	ExitSuccess      = 0
	ExitChanges      = 2 // Changes detected (diff mode)
	ExitPolicyBlock  = 3 // Blocked by risk policy
	ExitSourceHealth = 4 // Source health failure
)

// Pipeline orchestrates the full sync workflow.
type Pipeline struct {
	cfg     *config.Config
	catalog *catalog.Catalog
}

// New creates a new Pipeline.
func New(cfg *config.Config) *Pipeline {
	return &Pipeline{cfg: cfg}
}

// LoadCatalog loads the existing catalog from disk.
func (p *Pipeline) LoadCatalog() error {
	cat, err := catalog.Load(p.cfg.CatalogPath)
	if err != nil {
		return fmt.Errorf("loading catalog: %w", err)
	}
	p.catalog = cat
	slog.Info("catalog loaded",
		"version", cat.Version,
		"providers", len(cat.Providers))
	return nil
}

// SyncResult holds the outcome of a sync for one provider.
type SyncResult struct {
	Provider    string
	ChangeSet   *diff.ChangeSet
	JudgeResult *judge.Result
	PRNumber    int
	PRDraft     bool
	Skipped     bool
	SkipReason  string
	Error       error
}

// Sync runs the full pipeline for the configured providers.
func (p *Pipeline) Sync(ctx context.Context) ([]SyncResult, error) {
	if err := p.LoadCatalog(); err != nil {
		return nil, err
	}

	var results []SyncResult

	for _, providerName := range p.cfg.Providers {
		result := p.syncProvider(ctx, providerName)
		results = append(results, result)
	}

	return results, nil
}

// Diff runs discovery and diff without writing changes.
func (p *Pipeline) Diff(ctx context.Context) ([]diff.ChangeSet, error) {
	if err := p.LoadCatalog(); err != nil {
		return nil, err
	}

	var changesets []diff.ChangeSet

	for _, providerName := range p.cfg.Providers {
		cs, err := p.discoverAndDiff(ctx, providerName)
		if err != nil {
			slog.Error("diff failed", "provider", providerName, "error", err)
			continue
		}
		changesets = append(changesets, *cs)
	}

	return changesets, nil
}

func (p *Pipeline) syncProvider(ctx context.Context, providerName string) SyncResult {
	result := SyncResult{Provider: providerName}

	// 1. Discover + diff
	cs, err := p.discoverAndDiff(ctx, providerName)
	if err != nil {
		result.Error = err
		return result
	}
	result.ChangeSet = cs

	if !cs.HasChanges() {
		slog.Info("no changes detected", "provider", providerName)
		result.Skipped = true
		result.SkipReason = "no changes"
		return result
	}

	// 2. Risk assessment
	draft, blocked, reason := assessRisk(cs)
	if blocked {
		result.Skipped = true
		result.SkipReason = reason
		slog.Warn("sync blocked by policy", "provider", providerName, "reason", reason)
		return result
	}
	result.PRDraft = draft

	// 3. Validate new/updated models
	valResult := p.validateChanges(cs)
	if valResult.HasErrors() {
		result.Error = fmt.Errorf("validation failed:\n%s", validate.FormatResult(valResult))
		return result
	}

	// 4. LLM Judge (non-fatal)
	judgeResult, err := p.runJudge(ctx, cs)
	if err != nil {
		slog.Warn("judge evaluation failed, continuing", "provider", providerName, "error", err)
	} else if judgeResult != nil {
		result.JudgeResult = judgeResult
		behavior := judge.OnRejectBehavior(p.cfg.Judge.OnReject)
		if forceDraft := judge.ApplyToChangeSet(cs, judgeResult, behavior); forceDraft {
			result.PRDraft = true
		}
		if !cs.HasChanges() {
			slog.Info("all models rejected by judge, skipping", "provider", providerName)
			result.Skipped = true
			result.SkipReason = "all models rejected by judge"
			return result
		}
	}

	if p.cfg.DryRun {
		slog.Info("dry run — would create PR", "provider", providerName, "draft", draft)
		return result
	}

	// 4. Write changes
	writer := catalog.NewWriter(p.cfg.CatalogPath)
	for _, m := range cs.New {
		if _, err := writer.WriteModel(providerName, m.Model); err != nil {
			result.Error = fmt.Errorf("writing new model %s: %w", m.Name, err)
			return result
		}
	}
	for _, u := range cs.Updated {
		if _, err := writer.WriteModel(providerName, u.Model); err != nil {
			result.Error = fmt.Errorf("writing updated model %s: %w", u.Name, err)
			return result
		}
	}

	// 5. Update x_updater metadata
	p.updateMetadata(providerName, cs)

	// 6. Bump version
	if err := p.bumpVersion(cs); err != nil {
		result.Error = fmt.Errorf("bumping version: %w", err)
		return result
	}

	// 7. Regenerate manifest
	if err := catalog.GenerateManifest(p.cfg.CatalogPath); err != nil {
		result.Error = fmt.Errorf("generating manifest: %w", err)
		return result
	}

	// 9. Git + PR (if GitHub is configured)
	if p.cfg.GitHub.Token != "" {
		prNum, err := p.createPR(ctx, providerName, cs, result.PRDraft, result.JudgeResult)
		if err != nil {
			result.Error = fmt.Errorf("creating PR: %w", err)
			return result
		}
		result.PRNumber = prNum
	}

	return result
}

func (p *Pipeline) discoverAndDiff(ctx context.Context, providerName string) (*diff.ChangeSet, error) {
	a, err := adapter.Get(providerName)
	if err != nil {
		return nil, err
	}

	// Pre-discovery health check.
	if err := p.checkSourceHealth(ctx, a, providerName); err != nil {
		return nil, err
	}

	sources := make([]adapter.SourceType, 0, len(p.cfg.Sources))
	for _, s := range p.cfg.Sources {
		sources = append(sources, adapter.SourceType(s))
	}

	discovered, err := a.Discover(ctx, adapter.DiscoverOptions{
		Sources:  sources,
		NoCache:  p.cfg.NoCache,
		CacheDir: p.cfg.CacheDir,
	})
	if err != nil {
		return nil, fmt.Errorf("discovering models: %w", err)
	}

	discovered = deduplicateDiscovered(discovered)
	slog.Info("discovery complete", "provider", providerName, "models", len(discovered))

	// Post-discovery model count threshold check.
	if err := p.checkModelCountThreshold(a, discovered, providerName); err != nil {
		return nil, err
	}

	// Get existing models for this provider
	existing := make(map[string]*catalog.Model)
	if pc, ok := p.catalog.Providers[providerName]; ok {
		existing = pc.Models
	}

	opts := diff.DiffOptions{
		TrackDisplayName: p.cfg.Diff.TrackDisplayName,
	}
	cs := diff.Compute(providerName, discovered, existing, opts)
	return cs, nil
}

func (p *Pipeline) validateChanges(cs *diff.ChangeSet) *validate.Result {
	result := &validate.Result{}

	for _, m := range cs.New {
		filename := m.Name + ".yaml"
		r := validate.ValidateModel(m.Model, filename)
		result.Issues = append(result.Issues, r.Issues...)
	}
	for _, u := range cs.Updated {
		filename := u.Name + ".yaml"
		r := validate.ValidateModel(u.Model, filename)
		result.Issues = append(result.Issues, r.Issues...)
	}

	return result
}

func (p *Pipeline) updateMetadata(provider string, cs *diff.ChangeSet) {
	now := time.Now().UTC().Format(time.RFC3339)
	writer := catalog.NewWriter(p.cfg.CatalogPath)

	allModels := make([]*catalog.Model, 0)
	for _, m := range cs.New {
		allModels = append(allModels, m.Model)
	}
	for _, u := range cs.Updated {
		allModels = append(allModels, u.Model)
	}

	for _, m := range allModels {
		m.XUpdater = &catalog.XUpdater{
			LastVerifiedAt: now,
			Sources:        p.cfg.Sources,
		}
		_, _ = writer.WriteModel(provider, m)
	}
}

func (p *Pipeline) bumpVersion(cs *diff.ChangeSet) error {
	versionPath := filepath.Join(p.cfg.CatalogPath, "version.txt")
	data, err := os.ReadFile(versionPath)
	if err != nil {
		return err
	}

	version := strings.TrimSpace(string(data))
	newVersion, err := bumpSemver(version, len(cs.New) > 0)
	if err != nil {
		return err
	}

	return os.WriteFile(versionPath, []byte(newVersion+"\n"), 0o644)
}

// bumpSemver increments MINOR for new models, PATCH for updates only.
func bumpSemver(version string, hasNew bool) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid semver: %s", version)
	}

	var major, minor, patch int
	_, _ = fmt.Sscanf(parts[0], "%d", &major)
	_, _ = fmt.Sscanf(parts[1], "%d", &minor)
	_, _ = fmt.Sscanf(parts[2], "%d", &patch)

	if hasNew {
		minor++
		patch = 0
	} else {
		patch++
	}

	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}

// runJudge creates an LLM client and evaluates the changeset.
// Returns (nil, nil) when the judge is disabled.
func (p *Pipeline) runJudge(ctx context.Context, cs *diff.ChangeSet) (*judge.Result, error) {
	if !p.cfg.Judge.Enabled {
		return nil, nil
	}

	var client judge.LLMClient

	switch p.cfg.Judge.Provider {
	case "anthropic":
		apiKey := p.cfg.Anthropic.APIKey
		if apiKey == "" {
			return nil, fmt.Errorf("anthropic API key required when judge.provider=anthropic")
		}
		client = judge.NewAnthropicClient(
			apiKey,
			p.cfg.Anthropic.BaseURL,
			p.cfg.Judge.Model,
			p.cfg.Judge.MaxTokens,
		)
	case "openai":
		apiKey := p.cfg.OpenAI.APIKey
		if apiKey == "" {
			return nil, fmt.Errorf("openai API key required when judge.provider=openai")
		}
		client = judge.NewOpenAIClient(
			apiKey,
			p.cfg.OpenAI.BaseURL,
			p.cfg.Judge.Model,
			p.cfg.Judge.MaxTokens,
		)
	default:
		return nil, fmt.Errorf("unsupported judge provider: %s", p.cfg.Judge.Provider)
	}

	j := judge.New(client, p.cfg.Judge.Model, false)
	return j.Evaluate(ctx, cs)
}

// deduplicateDiscovered merges models discovered from multiple sources.
// API entries take priority; docs-sourced cost data fills gaps for API models missing cost.
func deduplicateDiscovered(models []adapter.DiscoveredModel) []adapter.DiscoveredModel {
	byName := make(map[string]*adapter.DiscoveredModel, len(models))
	var order []string

	for i := range models {
		m := &models[i]
		existing, ok := byName[m.Name]
		if !ok {
			byName[m.Name] = m
			order = append(order, m.Name)
			continue
		}

		// API source takes priority over docs.
		if existing.DiscoveredBy == adapter.SourceAPI && m.DiscoveredBy != adapter.SourceAPI {
			// Fill in cost data from docs if API model has none.
			if existing.Cost == nil && m.Cost != nil {
				existing.Cost = m.Cost
			}
		} else if m.DiscoveredBy == adapter.SourceAPI && existing.DiscoveredBy != adapter.SourceAPI {
			// Replace docs entry with API entry, preserving docs cost if needed.
			docsCost := existing.Cost
			byName[m.Name] = m
			if m.Cost == nil && docsCost != nil {
				m.Cost = docsCost
			}
		}
		// If both are from the same source, keep the first one.
	}

	result := make([]adapter.DiscoveredModel, 0, len(byName))
	for _, name := range order {
		result = append(result, *byName[name])
	}
	return result
}

// SourceHealthError indicates a source health check failure (exit code 4).
type SourceHealthError struct {
	Provider string
	Reason   string
}

func (e *SourceHealthError) Error() string {
	return fmt.Sprintf("source health check failed for %s: %s", e.Provider, e.Reason)
}

// checkSourceHealth performs a pre-discovery liveness probe.
func (p *Pipeline) checkSourceHealth(ctx context.Context, a adapter.Adapter, providerName string) error {
	hc, ok := a.(adapter.HealthChecker)
	if !ok || !p.cfg.Health.Enabled {
		return nil
	}
	slog.Info("running health check", "provider", providerName)
	if err := hc.HealthCheck(ctx); err != nil {
		return &SourceHealthError{Provider: providerName, Reason: fmt.Sprintf("liveness probe failed: %v", err)}
	}
	slog.Info("health check passed", "provider", providerName)
	return nil
}

// checkModelCountThreshold validates that the discovery returned a reasonable number of models.
func (p *Pipeline) checkModelCountThreshold(a adapter.Adapter, discovered []adapter.DiscoveredModel, providerName string) error {
	hc, ok := a.(adapter.HealthChecker)
	if !ok || !p.cfg.Health.Enabled {
		return nil
	}
	min := hc.MinExpectedModels()
	if min == 0 {
		return nil
	}
	threshold := p.cfg.Health.Threshold
	requiredMin := int(float64(min) * threshold)
	if len(discovered) < requiredMin {
		return &SourceHealthError{
			Provider: providerName,
			Reason:   fmt.Sprintf("discovered %d models, below threshold %d (min=%d × %.0f%%)", len(discovered), requiredMin, min, threshold*100),
		}
	}
	return nil
}

// assessRisk evaluates the changeset against risk gates.
// Returns: (draft, blocked, reason)
func assessRisk(cs *diff.ChangeSet) (bool, bool, string) {
	// Changed models > 25 → draft PR
	draft := cs.TotalChanged() > 25

	// Deprecation candidates > 3 → draft PR
	if len(cs.DeprecationCandidates) > 3 {
		draft = true
	}

	// Check for large price deltas
	for _, u := range cs.Updated {
		for _, c := range u.Changes {
			if c.Field == "cost.input_per_1k" || c.Field == "cost.output_per_1k" {
				oldVal, okOld := c.OldValue.(float64)
				newVal, okNew := c.NewValue.(float64)
				if okOld && okNew && oldVal > 0 {
					delta := (newVal - oldVal) / oldVal
					if delta > 0.35 || delta < -0.35 || newVal > oldVal*2 {
						draft = true
					}
				}
			}
		}
	}

	return draft, false, ""
}
