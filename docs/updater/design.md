# Model Updater — Final Plan (Post-Review + Competitive Research)

## Context

The `model-catalog/` requires manual intervention to add/update 75+ models across 8 providers. No existing tool automates end-to-end model catalog maintenance. We build `sentinel` — a standalone Go CLI that discovers models from providers and opens PRs to the centralized catalog repo.

**Key constraint**: Zero changes to existing catalog schema or gateway runtime. The updater writes the same YAML format the catalog already uses.

---

## Prior Art & Lessons Learned

### models.dev (anomalyco/models.dev)
- 92 providers, 500+ models, TOML format, 286 community contributors
- **Only 5 of 92 providers have generation scripts** (`generate-vercel.ts`, `generate-helicone.ts`, etc.) — the rest are manual community PRs
- **No scheduled cron jobs** — updates are reactive, not proactive
- **Key pattern to adopt**: Smart merge — their generation scripts preserve manually-added fields when updating API-sourced data. We must do the same.
- **Key pattern to adopt**: Zod-style schema validation gates every PR. Proven at scale.
- **Gap we fill**: They rely on community scale (286 contributors) instead of automation. We automate the discovery loop itself.

### Vercel AI SDK (vercel/ai)
- 40+ provider packages, TypeScript types (not a data catalog)
- **Gateway package is semi-automated**: GitHub Action runs weekdays at 8am, fetches from Vercel's `/v1/models` API, auto-creates PRs
- **Provider API change notification**: Separate workflow creates GitHub issues when providers release API updates (low-cost early warning)
- **Intentionally minimal metadata**: No pricing, no context windows — links to provider docs instead
- **Key pattern to adopt**: Scheduled auto-PR with human review (proven in production at Vercel scale)
- **Key pattern to adopt**: API change notification workflow as an early warning system
- **Gap we fill**: They punted on metadata entirely. We track pricing, capabilities, limits — the hard stuff.

### What Nobody Has Built
No project combines: scheduled multi-provider discovery + metadata-rich catalog (pricing, limits, capabilities) + auto-PR with risk gates. That's our tool.

---

## What's In v1 / What's Deferred

| In v1 | Deferred |
|-------|----------|
| `(provider, name)` identity | `lineage_id`, alias mapping, canonical identity system |
| Existing `cost.input_per_1k` / `cost.output_per_1k` | `pricing_v2` multi-dimensional billing |
| Known taxonomy warnings in PR | Taxonomy governance as sync blocker |
| Explicit merge precedence rules | Numeric trust weights / confidence scores |
| Lightweight `x_updater` in-YAML metadata | Provenance sidecar JSON artifacts |
| Heuristic rename warnings in PR body | Automatic rename detection with schema changes |
| LLM disabled by default | LLM as authoritative source for any canonical field |
| Smart merge (preserve manual edits) | — (in v1 from day one, learned from models.dev) |
| API change notification workflow | — (in v1 Phase 5, learned from Vercel) |

---

## Architecture

```
[Config] → [CLI: sync] → [Per-provider adapters] → [Diff engine] → [Validation] → [YAML writer] → [Git + PR]
                                   ↓
                          ┌────────┼────────┐
                        [API]   [Docs]    [LLM]    ← configurable per provider
                          └────────┼────────┘
                                   ↓
                          [DiscoveredModel[]  ]
```

### Vertical-Slice Adapters (Not Over-Abstracted)

Each provider owns its full pipeline. No "generic source plugin framework." Shared code is limited to:
- HTTP client with caching, rate limiting, conditional fetch (ETag/If-Modified-Since)
- HTML extraction helpers
- YAML normalization utilities

Provider packages contain their own API parsing, docs parsing, and merge logic.

---

## Project Structure

```
sentinel/
├── cmd/sentinel/main.go          # Cobra CLI entrypoint
├── internal/
│   ├── adapter/
│   │   ├── adapter.go                 # Adapter interface + DiscoveredModel type
│   │   ├── registry.go                # Global adapter registry (Register/Get)
│   │   └── providers/
│   │       ├── openai/                # API + docs
│   │       ├── anthropic/             # Docs-first (no /models API)
│   │       ├── google/                # API + docs
│   │       ├── cohere/                # API + docs
│   │       ├── mistral/               # API + docs
│   │       ├── openrouter/            # API only
│   │       └── huggingface/           # API only
│   ├── httpclient/                    # Shared HTTP client (cache, rate limit, ETag)
│   ├── htmlutil/                      # HTML extraction helpers
│   ├── catalog/
│   │   ├── catalog.go                 # Load existing catalog from disk
│   │   ├── model.go                   # Model/Provider YAML structs
│   │   ├── writer.go                  # Smart-merge YAML writer (preserves manual edits)
│   │   └── manifest.go                # Manifest regeneration (Go, replaces shell script)
│   ├── diff/
│   │   ├── diff.go                    # Compare discovered vs existing → ChangeSet
│   │   ├── changeset.go               # new, updated, deprecation_candidates, possible_renames, unchanged
│   │   └── render.go                  # Markdown PR body renderer
│   ├── validate/
│   │   └── validate.go                # Schema validation + pricing/limits sanity checks
│   ├── pipeline/
│   │   ├── pipeline.go                # Orchestrate: write → changelog → version → manifest → git → PR
│   │   ├── git.go                     # Branch, commit, push (go-git)
│   │   └── github.go                  # Create PR with risk-based draft logic (go-github)
│   ├── config/
│   │   └── config.go                  # Config struct + loader (viper)
│   └── cache/
│       └── file.go                    # File-based cache (~/.cache/sentinel/, TTL-based)
├── config.example.yaml
├── .github/workflows/
│   ├── sync.yaml                      # Scheduled sync (Mon+Thu 6am UTC)
│   └── provider-changes.yaml          # API change notification (creates issues)
├── go.mod / go.sum / Makefile / README.md / LICENSE
```

---

## Key Interfaces

### Adapter
```go
type Adapter interface {
    Name() string
    Discover(ctx context.Context, opts DiscoverOptions) ([]DiscoveredModel, error)
    SupportedSources() []SourceType  // api, docs, llm
}
```

### DiscoveredModel (matches existing YAML schema)
```go
type DiscoveredModel struct {
    Name          string     `yaml:"name"`
    DisplayName   string     `yaml:"display_name"`
    Family        string     `yaml:"family"`
    Status        string     `yaml:"status"`
    Cost          *Cost      `yaml:"cost,omitempty"`
    Limits        Limits     `yaml:"limits"`
    Capabilities  []string   `yaml:"capabilities"`
    Modalities    Modalities `yaml:"modalities"`
    DiscoveredBy  string     `yaml:"-"`  // "api", "docs", or "llm" — for PR metadata only
}
```

### ChangeSet
```go
type ChangeSet struct {
    New                  []ModelChange    // discovered, not in catalog
    Updated              []ModelUpdate    // exists, fields changed
    DeprecationCandidates []ModelChange   // in catalog, not discovered (PR warning only)
    PossibleRenames      []RenamePair     // heuristic: same family + similar limits/cost
    Unchanged            int
}
```

---

## Smart Merge (Learned from models.dev)

The YAML writer does NOT blindly overwrite existing model files. Instead:

1. **Load existing YAML** — parse the current model file into a map preserving all fields
2. **Overlay discovered fields** — only update fields the adapter has authoritative data for
3. **Preserve manual additions** — any fields in the existing YAML that the adapter didn't discover are kept as-is (e.g., hand-tuned `x_updater`, manually added `api_type`, comments)
4. **Preserve field order** — write fields in the same order as the existing file; new fields appended at the end
5. **Track which fields changed** — for the PR diff summary

This means a human can add a field like `api_type: responses` to a model file, and the updater won't clobber it on the next run — even if the API/docs sources don't know about that field.

**Exception**: If a discovered field conflicts with an existing value, the merge policy (below) determines what to do.

---

## Merge Policy (v1 — Explicit Rules, No Scores)

| Field | Source of truth | Conflict behavior |
|-------|----------------|-------------------|
| Model existence, IDs | API | — |
| Status (stable/beta/deprecated) | API when present | If API absent, keep current |
| Pricing (cost.*) | Docs | If API and docs disagree: keep current, draft PR, include conflict table |
| Limits (max_tokens, etc.) | API preferred, docs fallback | If conflict: keep current, flag in PR |
| Capabilities | API preferred, docs fallback | Union, warn on unknown values |
| Modalities | API preferred, docs fallback | Union, warn on unknown values |
| Display name, family | Docs preferred, API fallback | — |

**LLM (when enabled)**: May only propose `display_name`, `family`, `capabilities` candidates. May never set `cost`, `limits`, `status`. If only LLM evidence exists for a model, no file changes — candidates listed in PR body only.

---

## Schema Validation (Learned from models.dev)

Every model file (new or updated) must pass validation before the PR is created. models.dev validates with Zod schemas on every PR — we do the equivalent in Go:

**Required fields**: `name`, `display_name`, `status`, `limits.max_tokens`, at least one capability, non-empty modalities (input + output)
**Pricing sanity**: `input_per_1k` between 0 and 0.10, non-embedding models need non-zero `output_per_1k`
**Limits sanity**: `max_tokens` between 1024 and 2,000,000; `max_completion_tokens` <= `max_tokens`
**Naming consistency**: filename must match `name` field (e.g., `gpt-4o.yaml` has `name: "gpt-4o"`)
**Type checks**: Capabilities from known set (warn on unknown, don't block), modalities from known set

The `validate` command can also run standalone as a CI check on the catalog repo, independent of the updater.

---

## Risk Gates & PR Policy

**One PR per provider per run.** Provider isolation — one failure doesn't block others.

| Condition | Action |
|-----------|--------|
| Source health < 90% for provider | Abort PR for that provider |
| Changed models > 25 | Draft PR |
| Deprecation candidates > 3 | Draft PR |
| Any price delta > 35% or 2x | Draft PR |
| Unknown taxonomy values | Warning in PR body (not a blocker) |
| All clear | Normal PR |

---

## Rename Detection (Heuristic, No Schema Change)

If one model disappears and one appears in the same provider with:
- Same `family`
- Near-identical `limits` (within 10%)
- Near-identical `cost` (within 20%)

→ Add a `### Possible Renames` section in PR body. Human resolves. No `lineage_id`, no alias mapping, no schema changes.

---

## CLI Commands

| Command | Description |
|---------|-------------|
| `sentinel sync` | Full pipeline: discover → diff → validate → write → PR |
| `sentinel diff` | Show what would change (no writes) |
| `sentinel discover --provider=openai` | Discovery only, print to stdout |
| `sentinel validate --catalog-path=<path>` | Validate existing catalog (CI check) |

**Flags**: `--providers`, `--sources`, `--dry-run`, `--no-cache`, `--config`, `--risk-mode=strict|relaxed`

**Exit codes**: 0 (success/no changes), 2 (changes detected in diff mode), 3 (blocked by policy), 4 (source health failure)

---

## Catalog Output (Backward-Compatible)

Model YAML keeps all existing fields unchanged. One optional addition:

```yaml
# Appended to model YAML, ignored by current parsers
x_updater:
  last_verified_at: "2026-02-16T06:00:00Z"
  sources: ["api", "docs"]
```

Manifest regeneration reimplemented in Go (deterministic, replaces `generate-manifest.sh`).

---

## Rollback Playbook

Current sync checks `cachedVersion == remoteVersion` only (not semantic ordering). Any changed `version.txt` triggers sync. Therefore:

1. **On bad merge**: Revert the merge commit, bump `version.txt` forward (patch), push.
2. **Gateway picks up new version** on next sync cycle (or manual trigger).
3. **Never** roll version backward — always bump forward for audit trail.

---

## Docs Scraping Feasibility (Phase 1 Spike)

Before committing to colly, Phase 1 includes a spike to test whether target pricing pages return useful content via HTTP GET:
- OpenAI pricing page (`openai.com/api/pricing/`)
- Anthropic models page (`docs.anthropic.com/en/docs/about-claude/models`)
- Google pricing page (`ai.google.dev/pricing`)

If JS-rendered, evaluate `chromedp` or `rod` as alternatives. Decision gates Phase 2.

---

## Go Dependencies

| Dependency | Purpose |
|---|---|
| `cobra` + `viper` | CLI + config |
| `yaml.v3` | YAML read/write with field ordering |
| `colly/v2` (or `chromedp`/`rod`) | Docs scraping (decision after spike) |
| `go-git/v5` | Git operations |
| `go-github/v60` | PR creation |
| `x/time/rate` | Rate limiting |

LLM client dependencies deferred to Phase 4.

---

## Implementation Phases

### Phase 1: Core pipeline + OpenAI end-to-end
- [ ] Init Go module, cobra CLI, config loading (viper)
- [ ] Adapter interface, DiscoveredModel type, adapter registry
- [ ] Catalog loader (read existing YAML from disk)
- [ ] Smart-merge YAML writer (preserve manual edits, match existing format exactly)
- [ ] Diff engine (ChangeSet: new, updated, deprecation candidates, possible renames)
- [ ] Schema validation rules (required fields, pricing range, limits sanity, naming consistency)
- [ ] OpenAI adapter: API source (`GET /v1/models`)
- [ ] OpenAI adapter: docs source (pricing page — spike on colly vs headless)
- [ ] Merge logic for OpenAI (API existence + docs pricing)
- [ ] Pipeline: smart-merge files → changelog → version bump → manifest (Go impl) → git branch → PR
- [ ] Risk gates (draft PR thresholds)
- [ ] PR body markdown renderer (changes table, conflict table, warnings, possible renames)
- [ ] File-based cache (TTL, conditional fetch with ETag/If-Modified-Since)
- [ ] `discover`, `diff`, `sync`, `validate` commands working for OpenAI
- [ ] **Deliverable**: One working provider, end-to-end PR flow with risk gates and smart merge

### Phase 2: Anthropic + docs-first + conflict reporting
- [ ] Anthropic adapter (docs-only — no /models API)
- [ ] Docs scraping for Anthropic models + pricing pages
- [ ] Conflict reporting (when sources disagree, keep current + draft PR + conflict table)
- [ ] Source health checks (% of expected models returned)
- [ ] **Deliverable**: Two providers, docs-first pattern proven, conflict handling working

### Phase 3: Remaining providers
- [ ] Google adapter (API + docs)
- [ ] Cohere adapter (API + docs)
- [ ] Mistral adapter (API + docs)
- [ ] OpenRouter adapter (API only — free `/models` endpoint)
- [ ] HuggingFace adapter (API only)
- [ ] Per-provider rate limiter profiles
- [ ] Per-host concurrency cap (4 per host, 16 global)
- [ ] **Deliverable**: Full provider matrix, per-provider PR isolation

### Phase 4: LLM assist + governance hardening
- [ ] Optional LLM source (disabled by default)
- [ ] LLM extraction for `display_name`, `family`, `capabilities` only
- [ ] LLM candidates section in PR body (no file changes from LLM-only evidence)
- [ ] Rename heuristic refinement based on real-world data
- [ ] Known taxonomy value list (warn on unknown, don't block)
- [ ] **Deliverable**: LLM opt-in works, governance warnings without blocking

### Phase 5: CI schedule + production hardening
- [ ] GitHub Action: scheduled sync workflow (Mon+Thu 6am UTC, matrix by provider)
- [ ] GitHub Action: provider API change notification (creates issues when /models endpoints change — learned from Vercel)
- [ ] Soak testing (run for 2+ weeks, review PRs manually)
- [ ] Golden file tests (re-run with same input → zero diff)
- [ ] Rollback runbook documented
- [ ] README, config.example.yaml, contributing guide (adapter authoring docs for community)
- [ ] **Deliverable**: Production-ready scheduled automation with early warning system

---

## Test Scenarios

| Scenario | Expected behavior |
|----------|-------------------|
| Same lineage, new model name | `possible_rename` warning in PR, not new + deprecated |
| Model missing without explicit deprecation | Deprecation candidate in PR body only, no file changes |
| API and docs pricing disagree | Keep current value, draft PR, conflict table in body |
| LLM-only discovery | No model file changes, candidates in PR body |
| Unknown capability value | Warning in PR, not a sync blocker |
| Generated YAML loaded by existing catalog loader | Must parse identically |
| Re-run with unchanged inputs | Zero diff (deterministic output) |
| > 25 model changes in one provider | Draft PR |
| Price delta > 35% | Draft PR |
| Provider source returns < 90% expected models | Abort PR for that provider |
| One provider fails | Other providers' PRs still created |
| Version bump | MINOR for new models, PATCH for updates only, never auto-MAJOR |
| Manual field preserved after update | Hand-added `api_type` field survives smart merge |
| Validation blocks malformed model | Missing required fields prevent PR creation |
