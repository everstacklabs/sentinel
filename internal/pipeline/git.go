package pipeline

import (
	"fmt"
	"time"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// GitOps handles git operations for the catalog repo.
type GitOps struct {
	repo       *git.Repository
	worktree   *git.Worktree
	token      string
	branchName string
}

// OpenRepo opens a git repository at the given path.
func OpenRepo(path, token string) (*GitOps, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("opening repo: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("getting worktree: %w", err)
	}

	return &GitOps{repo: repo, worktree: wt, token: token}, nil
}

// CreateBranch creates and checks out a new branch.
func (g *GitOps) CreateBranch(name string) error {
	headRef, err := g.repo.Head()
	if err != nil {
		return fmt.Errorf("getting HEAD: %w", err)
	}

	branchRef := plumbing.NewBranchReferenceName(name)
	ref := plumbing.NewHashReference(branchRef, headRef.Hash())

	if err := g.repo.Storer.SetReference(ref); err != nil {
		return fmt.Errorf("creating branch ref: %w", err)
	}

	if err := g.worktree.Checkout(&git.CheckoutOptions{
		Branch: branchRef,
	}); err != nil {
		return err
	}

	g.branchName = name
	return nil
}

// AddAll stages all changes.
func (g *GitOps) AddAll() error {
	_, err := g.worktree.Add(".")
	return err
}

// Commit creates a commit with the given message.
func (g *GitOps) Commit(message string) error {
	_, err := g.worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "sentinel",
			Email: "sentinel@everstack.dev",
			When:  time.Now(),
		},
	})
	return err
}

// Push pushes the current branch to origin.
func (g *GitOps) Push() error {
	branchName, err := g.pushBranch()
	if err != nil {
		return err
	}

	return g.repo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []gitconfig.RefSpec{
			gitconfig.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branchName, branchName)),
		},
		Auth: &githttp.BasicAuth{
			Username: "x-access-token",
			Password: g.token,
		},
	})
}

func (g *GitOps) pushBranch() (string, error) {
	if g.branchName != "" {
		return g.branchName, nil
	}

	headRef, err := g.repo.Head()
	if err != nil {
		return "", fmt.Errorf("getting HEAD: %w", err)
	}
	if !headRef.Name().IsBranch() {
		return "", fmt.Errorf("HEAD is not a branch")
	}

	return headRef.Name().Short(), nil
}
