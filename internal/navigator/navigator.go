package navigator

import (
	"context"
	"fmt"
)

type Git interface {
	CurrentBranch(ctx context.Context) (string, error)
	ReflogBranchVisits(ctx context.Context) ([]string, error)
	LocalBranches(ctx context.Context) ([]string, error)
}

type Navigator struct {
	git Git
}

func NewNavigator(g Git) (*Navigator, error) {
	if g == nil {
		return nil, fmt.Errorf("navigator: git client is required")
	}
	return &Navigator{git: g}, nil
}

func (n *Navigator) RecentBranches(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		return []string{}, nil
	}

	current, err := n.git.CurrentBranch(ctx)
	if err != nil {
		return nil, err
	}

	locals, err := n.git.LocalBranches(ctx)
	if err != nil {
		return nil, err
	}

	localSet := make(map[string]struct{}, len(locals))
	for _, branch := range locals {
		localSet[branch] = struct{}{}
	}

	history, err := n.git.ReflogBranchVisits(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, limit)
	seen := make(map[string]struct{}, limit)

	appendCandidate := func(branch string) {
		if branch == "" {
			return
		}
		if branch == current {
			return
		}
		if _, ok := seen[branch]; ok {
			return
		}
		seen[branch] = struct{}{}
		result = append(result, branch)
	}

	for _, branch := range history {
		if len(result) >= limit {
			break
		}
		if _, ok := localSet[branch]; !ok {
			continue
		}
		appendCandidate(branch)
	}

	if len(result) >= limit {
		return result[:limit], nil
	}

	for _, branch := range locals {
		if len(result) >= limit {
			break
		}
		appendCandidate(branch)
	}

	if len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}
