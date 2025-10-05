package app

import (
	"context"
	"io"

	"branch-navigator/internal/git"
	"branch-navigator/internal/ui"
)

type Action string

const (
	ActionCheckout Action = "checkout"
	ActionMerge    Action = "merge"
	ActionDelete   Action = "delete"
)

type Options struct {
	Action Action
	Limit  int
}

type Dependencies struct {
	Git       GitClient
	Navigator Navigator
	Terminal  Terminal
	Input     io.Reader
	Output    io.Writer
	Error     io.Writer
}

type GitClient interface {
	CurrentBranch(context.Context) (string, error)
	CheckoutBranch(context.Context, string) (string, error)
	MergeBranch(context.Context, string, git.MergeOptions) (git.MergeResult, error)
	DeleteBranch(context.Context, string, git.DeleteOptions) (git.DeleteResult, error)
}

type Navigator interface {
	RecentBranches(context.Context, int) ([]string, error)
}

type Terminal interface {
	Select([]ui.Branch) (ui.Result, error)
}

func Run(ctx context.Context, opts Options, deps Dependencies) int {
	return 0
}
