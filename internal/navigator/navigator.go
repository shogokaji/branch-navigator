package navigator

import (
	"context"
	"errors"
	"strings"
)

// GitService describes the git functionality required by the navigator.
type GitService interface {
	CurrentBranch(ctx context.Context) (string, error)
	ReflogBranchMoves(ctx context.Context) ([]string, error)
	BranchesByCommitDate(ctx context.Context) ([]string, error)
	BranchExists(ctx context.Context, branch string) (bool, error)
}

// Navigator coordinates branch retrieval using GitService.
type Navigator struct {
	git GitService
}

// New constructs a Navigator bound to the provided GitService.
func New(git GitService) (*Navigator, error) {
	if git == nil {
		return nil, errors.New("git service is required")
	}
	return &Navigator{git: git}, nil
}

// RecentBranches returns up to limit recent branch names excluding the current branch, deduplicated.
func (n *Navigator) RecentBranches(ctx context.Context, limit int) ([]string, error) {
	if n == nil || n.git == nil {
		return nil, errors.New("navigator is not configured")
	}
	if limit <= 0 {
		return nil, nil
	}

	current, err := n.git.CurrentBranch(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]string, 0, limit)
	seen := map[string]struct{}{current: struct{}{}}

	reflogBranches, err := n.git.ReflogBranchMoves(ctx)
	if err != nil {
		return nil, err
	}

	results, err = n.appendBranches(ctx, results, reflogBranches, seen, limit)
	if err != nil {
		return nil, err
	}
	if len(results) >= limit {
		return results, nil
	}

	fallbackBranches, err := n.git.BranchesByCommitDate(ctx)
	if err != nil {
		return nil, err
	}

	results, err = n.appendBranches(ctx, results, fallbackBranches, seen, limit)
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (n *Navigator) appendBranches(ctx context.Context, current []string, candidates []string, seen map[string]struct{}, limit int) ([]string, error) {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}

		exists, err := n.git.BranchExists(ctx, candidate)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}

		seen[candidate] = struct{}{}
		current = append(current, candidate)
		if len(current) >= limit {
			break
		}
	}
	return current, nil
}
