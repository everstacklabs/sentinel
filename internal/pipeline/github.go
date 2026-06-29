package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/everstacklabs/sentinel/internal/diff"
	"github.com/everstacklabs/sentinel/internal/judge"
	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

// createPR creates a GitHub PR for catalog changes.
func (p *Pipeline) createPR(ctx context.Context, provider string, cs *diff.ChangeSet, draft bool, judgeResult *judge.Result) (int, error) {
	branchName := fmt.Sprintf("sentinel/%s-%s", provider, time.Now().Format("20060102-150405"))
	commitMsg := fmt.Sprintf("chore(catalog): update %s models", provider)

	// Git operations
	gitOps, err := OpenRepo(p.cfg.CatalogPath, p.cfg.GitHub.Token)
	if err != nil {
		return 0, err
	}

	if err := gitOps.CreateBranch(branchName); err != nil {
		return 0, fmt.Errorf("creating branch: %w", err)
	}

	if err := gitOps.AddAll(); err != nil {
		return 0, fmt.Errorf("staging changes: %w", err)
	}

	if err := gitOps.Commit(commitMsg); err != nil {
		return 0, fmt.Errorf("committing: %w", err)
	}

	if err := gitOps.Push(); err != nil {
		return 0, fmt.Errorf("pushing: %w", err)
	}

	// Create PR
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: p.cfg.GitHub.Token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	title := fmt.Sprintf("chore(catalog): update %s models", provider)
	body := diff.RenderPRBody(cs)
	if section := judge.RenderSection(judgeResult); section != "" {
		body += "\n" + section
	}

	pr, _, err := client.PullRequests.Create(ctx, p.cfg.GitHub.Owner, p.cfg.GitHub.Repo, &github.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &branchName,
		Base:  &p.cfg.GitHub.BaseBranch,
		Draft: &draft,
	})
	if err != nil {
		return 0, fmt.Errorf("creating PR: %w", err)
	}

	slog.Info("PR created",
		"provider", provider,
		"number", pr.GetNumber(),
		"draft", draft,
		"url", pr.GetHTMLURL())

	return pr.GetNumber(), nil
}

// createBatchPR creates a single GitHub PR for all safe catalog changes in a run.
func (p *Pipeline) createBatchPR(ctx context.Context, changesets []*diff.ChangeSet, draft bool, judgeResults map[string]*judge.Result) (int, bool, error) {
	branchName := fmt.Sprintf("sentinel/model-sync-%s", time.Now().UTC().Format("20060102-150405"))
	commitMsg := "chore(catalog): sync provider model catalog"

	gitOps, err := OpenRepo(p.cfg.CatalogPath, p.cfg.GitHub.Token)
	if err != nil {
		return 0, false, err
	}

	if err := gitOps.CreateBranch(branchName); err != nil {
		return 0, false, fmt.Errorf("creating branch: %w", err)
	}

	if err := gitOps.AddAll(); err != nil {
		return 0, false, fmt.Errorf("staging changes: %w", err)
	}

	if err := gitOps.Commit(commitMsg); err != nil {
		return 0, false, fmt.Errorf("committing: %w", err)
	}

	if err := gitOps.Push(); err != nil {
		return 0, false, fmt.Errorf("pushing: %w", err)
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: p.cfg.GitHub.Token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	title := "chore(catalog): sync provider model catalog"
	body := renderBatchPRBody(changesets, judgeResults)
	pr, _, err := client.PullRequests.Create(ctx, p.cfg.GitHub.Owner, p.cfg.GitHub.Repo, &github.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &branchName,
		Base:  &p.cfg.GitHub.BaseBranch,
		Draft: &draft,
	})
	if err != nil {
		return 0, false, fmt.Errorf("creating PR: %w", err)
	}

	autoMergeEnabled := false
	if p.cfg.Automation.AutoMerge && !draft {
		if err := p.enableAutoMerge(ctx, pr, p.cfg.Automation.MergeMethod); err != nil {
			return pr.GetNumber(), false, fmt.Errorf("enabling auto-merge: %w", err)
		}
		autoMergeEnabled = true
	}

	slog.Info("batch PR created",
		"number", pr.GetNumber(),
		"draft", draft,
		"auto_merge_enabled", autoMergeEnabled,
		"url", pr.GetHTMLURL())

	return pr.GetNumber(), autoMergeEnabled, nil
}

func (p *Pipeline) enableAutoMerge(ctx context.Context, pr *github.PullRequest, method string) error {
	nodeID := pr.GetNodeID()
	if nodeID == "" {
		return fmt.Errorf("pull request node id is missing")
	}

	payload := struct {
		Query     string            `json:"query"`
		Variables map[string]string `json:"variables"`
	}{
		Query: `mutation EnablePullRequestAutoMerge($pullRequestId: ID!, $mergeMethod: PullRequestMergeMethod!) {
  enablePullRequestAutoMerge(input: {pullRequestId: $pullRequestId, mergeMethod: $mergeMethod}) {
    pullRequest { number }
  }
}`,
		Variables: map[string]string{
			"pullRequestId": nodeID,
			"mergeMethod":   normalizeMergeMethod(method),
		},
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return fmt.Errorf("encoding graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.github.com/graphql", &body)
	if err != nil {
		return fmt.Errorf("creating graphql request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.GitHub.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling github graphql: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading github graphql response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("github graphql returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("decoding github graphql response: %w", err)
	}
	if len(result.Errors) > 0 {
		messages := make([]string, 0, len(result.Errors))
		for _, graphErr := range result.Errors {
			messages = append(messages, graphErr.Message)
		}
		return fmt.Errorf("github graphql error: %s", strings.Join(messages, "; "))
	}

	return nil
}

func normalizeMergeMethod(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "merge":
		return "MERGE"
	case "rebase":
		return "REBASE"
	default:
		return "SQUASH"
	}
}

func renderBatchPRBody(changesets []*diff.ChangeSet, judgeResults map[string]*judge.Result) string {
	var b strings.Builder

	var totalNew, totalUpdated, totalDeprecation int
	for _, cs := range changesets {
		totalNew += len(cs.New)
		totalUpdated += len(cs.Updated)
		totalDeprecation += len(cs.DeprecationCandidates)
	}

	fmt.Fprintf(&b, "## Automated Model Catalog Sync\n\n")
	fmt.Fprintf(&b, "**Summary**: %d providers, %d new, %d updated, %d deprecation candidates\n\n",
		len(changesets), totalNew, totalUpdated, totalDeprecation)
	b.WriteString("Sentinel included only non-draft changes that passed source health, validation, risk gates, and judge checks for this run.\n\n")

	b.WriteString("### Providers\n\n")
	b.WriteString("| Provider | New | Updated | Deprecation Candidates |\n")
	b.WriteString("|----------|-----|---------|------------------------|\n")
	for _, cs := range changesets {
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d |\n",
			cs.Provider, len(cs.New), len(cs.Updated), len(cs.DeprecationCandidates))
	}
	b.WriteString("\n")

	for _, cs := range changesets {
		fmt.Fprintf(&b, "## %s\n\n", cs.Provider)

		if len(cs.New) > 0 {
			b.WriteString("### New Models\n\n")
			b.WriteString("| Model | Family | Status | Context Window |\n")
			b.WriteString("|-------|--------|--------|----------------|\n")
			for _, m := range cs.New {
				fmt.Fprintf(&b, "| `%s` | %s | %s | %d |\n",
					m.Name, markdownCell(m.Model.Family), markdownCell(m.Model.Status), m.Model.Limits.MaxTokens)
			}
			b.WriteString("\n")
		}

		if len(cs.Updated) > 0 {
			b.WriteString("### Updated Models\n\n")
			b.WriteString("| Model | Changed Fields | Details |\n")
			b.WriteString("|-------|----------------|---------|\n")
			for _, u := range cs.Updated {
				fields := make([]string, 0, len(u.Changes))
				details := make([]string, 0, len(u.Changes))
				for _, c := range u.Changes {
					fields = append(fields, c.Field)
					details = append(details, fmt.Sprintf("%s: %v -> %v", c.Field, c.OldValue, c.NewValue))
				}
				fmt.Fprintf(&b, "| `%s` | %s | %s |\n",
					u.Name, markdownCell(strings.Join(fields, ", ")), markdownCell(strings.Join(details, "; ")))
			}
			b.WriteString("\n")
		}

		if len(cs.DeprecationCandidates) > 0 {
			b.WriteString("### Deprecation Candidates\n\n")
			b.WriteString("These models exist in the catalog but were not found by the provider API. Sentinel does not delete them automatically.\n\n")
			for _, m := range cs.DeprecationCandidates {
				fmt.Fprintf(&b, "- `%s` (%s)\n", m.Name, m.Model.Family)
			}
			b.WriteString("\n")
		}

		if section := judge.RenderSection(judgeResults[cs.Provider]); section != "" {
			b.WriteString(section)
			b.WriteString("\n")
		}
	}

	b.WriteString("---\n")
	b.WriteString("*Generated by sentinel batch sync*\n")

	return b.String()
}

func markdownCell(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return value
}
