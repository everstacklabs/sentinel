# AGENTS.md — sentinel

## Project Overview

**sentinel** is a Go CLI tool that automatically discovers AI model metadata from provider APIs, diffs it against the [everstacklabs/everstack](https://github.com/everstacklabs/everstack) model catalog, and opens pull requests with validated changes. It runs on a 12-hour cron schedule and can also be invoked manually.

The pipeline: **discover → diff → validate → judge (optional) → write → version bump → manifest → git → PR**.

## Tech Stack

- **Language:** Go 1.23
- **CLI framework:** Cobra + Viper
- **Git operations:** go-git/v5 (in-process, no shell-out)
- **GitHub API:** google/go-github/v60 + oauth2
- **HTTP:** Custom rate-limited client with file-backed ETag/TTL cache
- **Config:** Viper (YAML file + env vars with `SENTINEL_` prefix)
- **CI:** GitHub Actions (ci.yml for build/test/lint, sync.yml for scheduled runs)

## Repository Structure

```
cmd/sentinel/main.go       # Entrypoint — all CLI commands defined here
internal/
  adapter/                       # Provider adapter interface + registry
    providers/openai/            # OpenAI adapter (only provider implemented so far)
  cache/                         # TTL file cache with ETag support
  catalog/                       # Catalog loader, YAML model structs, smart-merge writer, manifest generator
  config/                        # Viper config loader with env var bindings
  diff/                          # Changeset computation, rename detection, PR body rendering
  httpclient/                    # Rate-limited HTTP client with cache integration
  judge/                         # LLM-as-judge evaluation (Anthropic + OpenAI clients)
  pipeline/                      # Orchestrator: sync pipeline, git ops, GitHub PR creation
  validate/                      # Model validation rules (required fields, pricing sanity, limits)
docs/updater/design.md           # Full design document (architecture, phases, merge policy, risk gates)
config.example.yaml              # Documented config template
Makefile                         # build, test, lint, discover, diff, sync, validate targets
```

## CLI Commands

| Command | Purpose |
|---|---|
| `sync` | Full pipeline — discover, diff, validate, write, git, PR |
| `diff` | Preview changes only — exits with code 2 if changes found |
| `discover --provider=<name>` | Debug: print discovered models to stdout |
| `validate --catalog-path=<path>` | CI check: validate all catalog models |

**Exit codes:** 0 = success, 2 = changes detected (diff mode), 3 = policy blocked, 4 = source health failure.

## Key Architectural Patterns

### Adapter Registry
Providers self-register via `init()` using blank imports in `main.go`. To add a new provider, create a package under `internal/adapter/providers/<name>/` that implements `adapter.Adapter` and calls `adapter.Register()` in its `init()`. Then add the blank import in `main.go`.

### Smart Merge Writer
`catalog.SmartMergeWriter` uses `yaml.Node` trees to overlay discovered fields onto existing YAML files, preserving hand-edited keys, comments, and field ordering. It skips writing if no changes are detected.

### Risk Assessment
`pipeline.assessRisk()` evaluates changesets and returns `(draft, blocked, reason)`. Thresholds: >25 total changes, >3 deprecation candidates, or price deltas >35% / 2x trigger draft PRs. In `strict` mode, blocked changesets are rejected; in `relaxed` mode, they proceed as normal PRs.

### LLM-as-Judge
Disabled by default. When enabled, evaluates changesets for suspicious capabilities, pricing, or limits before writing. Non-fatal — failures log a warning and the pipeline continues. Supports `on_reject: "draft"` (mark PR as draft) or `"exclude"` (remove rejected models).

## Development

```bash
# Build
make build

# Run tests
make test

# Lint
make lint

# Quick commands (require config.yaml)
make discover    # openai discovery
make diff        # preview changes
make sync        # dry-run sync
make validate    # validate catalog
```

### Configuration

Copy `config.example.yaml` to `config.yaml`. Key env vars:
- `GITHUB_TOKEN` — for PR creation and catalog repo access
- `OPENAI_API_KEY` — for OpenAI model discovery
- `ANTHROPIC_API_KEY` — for LLM-as-judge (when enabled)

All config keys can be overridden via env vars with `SENTINEL_` prefix (e.g., `SENTINEL_CATALOG_PATH`).

## Testing Conventions

- Tests are co-located (`_test.go` in the same package)
- Table-driven tests throughout — follow the existing pattern when adding new tests
- Use `t.TempDir()` for filesystem isolation
- Mock interfaces (e.g., `LLMClient`) for external dependencies
- Integration tests use `//go:build integration` build tag and skip when API keys are unset
- Run `make test` — this runs `go test ./...`

## Code Style

- Standard Go conventions — `gofmt`, `golangci-lint`
- Functional options pattern for configurable types (see `httpclient`)
- Errors are returned, not panicked — pipeline isolates per-provider failures
- Unexported helpers are tested directly (tests are in the same package)
- YAML struct tags use `yaml:"snake_case"` and `json:"snake_case"` consistently

## CI/CD

- **ci.yml:** Runs on push/PR to `main`. Three parallel jobs: build, test, lint (Go 1.23).
- **sync.yml:** Scheduled cron at 6am and 6pm UTC. Checks out both sentinel and the catalog repo, builds, and runs sync. Also supports `workflow_dispatch` with `providers` and `dry_run` inputs.

## Adding a New Provider

1. Create `internal/adapter/providers/<name>/<name>.go`
2. Implement the `adapter.Adapter` interface (`Name()`, `Discover()`, `SupportedSources()`)
3. Call `adapter.Register(&YourAdapter{})` in the package's `init()`
4. Add `_ "github.com/everstacklabs/sentinel/internal/adapter/providers/<name>"` to `cmd/sentinel/main.go`
5. Add provider-specific config section to `config.example.yaml` if needed
6. Add the provider name to the `providers` list in config
7. Write unit tests + an integration test gated on `//go:build integration`

The design doc (`docs/updater/design.md`) lists 7 planned providers: openai, anthropic, google, cohere, mistral, openrouter, huggingface. Only openai is currently implemented.
