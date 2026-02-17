package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/everstacklabs/sentinel/internal/diff"
	"github.com/everstacklabs/sentinel/internal/judge"
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
