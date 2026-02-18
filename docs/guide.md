# Using Sentinel with your own catalog

This guide walks through setting up Sentinel to keep your own model catalog up to date automatically.

## Prerequisites

- Go 1.26+
- A GitHub repo containing your model catalog
- API keys for the providers you want to track
- A GitHub personal access token with repo and PR permissions

## 1. Catalog structure

Sentinel expects your catalog repo to follow this layout:

```
your-catalog/
  version.txt                          # semver, e.g. "1.4.0"
  providers/
    openai/
      provider.yaml
      models/
        gpt-4o.yaml
        gpt-4o-mini.yaml
    anthropic/
      provider.yaml
      models/
        claude-sonnet-4-20250514.yaml
  manifest.yaml                        # auto-generated, do not edit
```

### version.txt

A single line with the current catalog version in semver format. Sentinel bumps this automatically -- MINOR for new models, PATCH for updates.

```
1.0.0
```

### provider.yaml

One per provider directory. Describes the provider itself.

```yaml
name: openai
display_name: OpenAI
provider_type: llm
supports_model_discovery: true
```

### Model files

One YAML file per model, named to match the model's `name` field. For example, `gpt-4o.yaml`:

```yaml
name: gpt-4o
display_name: GPT-4o
family: gpt-4o
status: stable
cost:
  input_per_1k: 0.0025
  output_per_1k: 0.01
limits:
  max_tokens: 128000
  max_completion_tokens: 16384
capabilities:
  - chat
  - function_calling
  - vision
  - json_mode
modalities:
  input:
    - text
    - image
  output:
    - text
```

You can add any extra fields you need (e.g., `api_type`, `custom_notes`). Sentinel preserves fields it doesn't know about during updates.

### Valid values

**status:** `stable`, `beta`, `preview`, `deprecated`

**capabilities:** `chat`, `completion`, `embedding`, `function_calling`, `vision`, `json_mode`, `json_schema`, `streaming`, `system_message`, `logprobs`, `image_generation`, `code_interpreter`

**modalities (input):** `text`, `image`, `audio`, `video`, `file`

**modalities (output):** `text`, `image`, `audio`, `video`, `file`

## 2. Install Sentinel

```bash
git clone https://github.com/midfusionlabs/sentinel.git
cd sentinel
make build
```

The binary is at `bin/sentinel`.

## 3. Configure

Copy the example config and edit it:

```bash
cp config.example.yaml config.yaml
```

Minimal configuration:

```yaml
catalog_path: "/path/to/your-catalog"
providers:
  - openai
sources:
  - api
```

Set your API keys as environment variables:

```bash
export OPENAI_API_KEY="sk-..."
export GITHUB_TOKEN="ghp_..."
```

For the full list of config options, see [config.example.yaml](../config.example.yaml).

## 4. Initialize your catalog

If you're starting from scratch, create the directory structure:

```bash
mkdir -p your-catalog/providers/openai/models
echo "1.0.0" > your-catalog/version.txt
```

Create `your-catalog/providers/openai/provider.yaml`:

```yaml
name: openai
display_name: OpenAI
provider_type: llm
supports_model_discovery: true
```

Then run discovery to populate it:

```bash
# Preview what Sentinel finds
sentinel discover --provider=openai

# Run a dry-run sync to see what would be written
sentinel sync --dry-run --providers=openai

# Run the actual sync (writes files locally, no PR)
# To create a PR, configure the github section in config.yaml
sentinel sync --providers=openai
```

## 5. Preview changes

Before running a full sync, use `diff` to see what would change:

```bash
sentinel diff
```

This compares discovered models against your catalog and prints a summary. Exit code `2` means changes were found, `0` means the catalog is already up to date.

## 6. Validate your catalog

Run validation independently to check your catalog for errors:

```bash
sentinel validate --catalog-path=/path/to/your-catalog
```

This checks every model file for:
- Required fields (`name`, `display_name`, `status`, `limits.max_tokens`, capabilities, modalities)
- Pricing sanity (`input_per_1k` and `output_per_1k` between 0 and 0.10)
- Limits ranges (max_tokens between 1,024 and 2,000,000)
- Filename consistency (`gpt-4o.yaml` must contain `name: gpt-4o`)

Errors block PRs. Warnings are included in the PR body but don't block.

You can use this as a CI check on your catalog repo to catch manual editing mistakes.

## 7. Automated sync with GitHub Actions

To run Sentinel on a schedule, add a workflow to the repo that hosts Sentinel (not your catalog repo).

Create `.github/workflows/sync.yml`:

```yaml
name: Model Sync

on:
  schedule:
    - cron: "0 6,18 * * *"  # every 12 hours
  workflow_dispatch:
    inputs:
      providers:
        description: "Comma-separated providers to sync (blank for all)"
        required: false
        default: ""
      dry_run:
        description: "Dry run (preview changes only)"
        required: false
        type: boolean
        default: false

permissions:
  contents: write
  pull-requests: write

jobs:
  sync:
    runs-on: ubuntu-latest
    environment: prod  # use your environment name
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"

      - name: Build
        run: make build

      - name: Checkout catalog
        uses: actions/checkout@v4
        with:
          repository: your-org/your-catalog
          token: ${{ secrets.GH_PAT }}
          path: model-catalog

      - name: Run sync
        env:
          GITHUB_TOKEN: ${{ secrets.GH_PAT }}
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
          SENTINEL_CATALOG_PATH: ./model-catalog
          SENTINEL_GITHUB_OWNER: your-org
          SENTINEL_GITHUB_REPO: your-catalog
          SENTINEL_GITHUB_BASE_BRANCH: main
        run: |
          ARGS="sync"
          if [ "${{ github.event.inputs.dry_run }}" = "true" ]; then
            ARGS="$ARGS --dry-run"
          fi
          if [ -n "${{ github.event.inputs.providers }}" ]; then
            ARGS="$ARGS --providers=${{ github.event.inputs.providers }}"
          fi
          ./bin/sentinel $ARGS
```

### Required secrets

Add these to your GitHub environment (Settings > Environments > your environment > Secrets):

| Secret | Purpose |
|---|---|
| `GH_PAT` | Fine-grained token with `contents: write` and `pull-requests: write` on your catalog repo |
| `OPENAI_API_KEY` | OpenAI API key for model discovery |
| `ANTHROPIC_API_KEY` | Optional. Required if syncing Anthropic models or using LLM-as-judge |

### How PRs work

Each sync run creates one PR per provider. The PR includes:
- A table of new, updated, and unchanged models
- Field-level diffs for updated models
- Deprecation candidates (models in catalog but not discovered)
- Possible renames (heuristic matches)
- Validation warnings

PRs are opened as drafts when risk thresholds are exceeded (>25 changes, >3 deprecation candidates, or large price swings). Otherwise they're normal PRs ready for review.

Branch naming: `sentinel/<provider>-<timestamp>` (e.g., `sentinel/openai-20260218-060000`).

## 8. Enable LLM-as-judge (optional)

The judge sends your changeset to an LLM before writing, catching suspicious values like wrong capabilities or nonsensical pricing. It's disabled by default.

Add to your `config.yaml`:

```yaml
judge:
  enabled: true
  provider: "anthropic"       # or "openai"
  model: "claude-sonnet-4-20250514"
  on_reject: "draft"          # "draft" = mark PR as draft, "exclude" = remove rejected models
  max_tokens: 4096
```

Set `ANTHROPIC_API_KEY` (or `OPENAI_API_KEY` if using OpenAI as the judge provider).

The judge is non-fatal. If the LLM call fails, the pipeline logs a warning and continues without it.

## 9. Adding custom fields

You can add any fields to your model YAML files. Sentinel's smart merge preserves fields it doesn't manage. For example:

```yaml
name: gpt-4o
display_name: GPT-4o
family: gpt-4o
status: stable
# ... standard fields ...

# Custom fields -- Sentinel will not touch these
api_type: responses
deprecation_date: "2026-06-01"
internal_notes: "Preferred model for production traffic"
```

These survive every sync run. Sentinel only overwrites fields it has discovered data for.

## 10. Running as a CI validator

Add Sentinel's `validate` command to your catalog repo's CI to catch errors in manual edits:

```yaml
# In your catalog repo's CI workflow
- name: Validate catalog
  run: |
    # Install sentinel (or use a pre-built binary)
    sentinel validate --catalog-path=.
```

Exit code `1` means validation errors were found.
