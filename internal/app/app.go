package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

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
	if opts.Limit <= 0 {
		fmt.Fprintln(deps.Error, "limit must be greater than 0")
		return 2
	}
	if err := validateDeps(deps); err != nil {
		fmt.Fprintln(deps.Error, err)
		return 1
	}

	branches, err := deps.Navigator.RecentBranches(ctx, opts.Limit)
	if err != nil {
		fmt.Fprintln(deps.Error, err)
		return 1
	}

	current, err := deps.Git.CurrentBranch(ctx)
	if err != nil {
		fmt.Fprintln(deps.Error, err)
		return 1
	}

	candidates := make([]ui.Branch, 0, len(branches)+1)
	candidates = append(candidates, ui.Branch{Name: current, Current: true})
	for _, branch := range branches {
		candidates = append(candidates, ui.Branch{Name: branch})
	}

	result, err := deps.Terminal.Select(candidates)
	if err != nil {
		fmt.Fprintln(deps.Error, err)
		return 1
	}

	if result.Quit || result.AlreadyOn {
		return 0
	}

	switch opts.Action {
	case ActionCheckout:
		message, err := deps.Git.CheckoutBranch(ctx, result.Branch)
		if err != nil {
			fmt.Fprintln(deps.Error, err)
			return 1
		}
		printIfNotEmpty(deps.Output, message)
		return 0
	case ActionMerge:
		mergeResult, err := deps.Git.MergeBranch(ctx, result.Branch, git.MergeOptions{})
		printIfNotEmpty(deps.Output, mergeResult.Stdout)
		stderrOutput := strings.TrimSpace(mergeResult.Stderr)
		if err != nil {
			if stderrOutput != "" {
				fmt.Fprintln(deps.Error, stderrOutput)
				if !strings.Contains(err.Error(), stderrOutput) {
					fmt.Fprintln(deps.Error, err)
				}
			} else {
				fmt.Fprintln(deps.Error, err)
			}
			return 1
		}
		if stderrOutput != "" {
			fmt.Fprintln(deps.Error, stderrOutput)
		}
		return 0
	case ActionDelete:
		if err := handleDelete(ctx, deps.Git, deps.Input, deps.Output, deps.Error, result.Branch); err != nil {
			fmt.Fprintln(deps.Error, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(deps.Error, "%s action is not implemented yet\n", opts.Action)
		return 2
	}
}

func validateDeps(deps Dependencies) error {
	switch {
	case deps.Git == nil:
		return errors.New("git client is not configured")
	case deps.Navigator == nil:
		return errors.New("navigator is not configured")
	case deps.Terminal == nil:
		return errors.New("terminal UI is not configured")
	case deps.Input == nil:
		return errors.New("input reader is not configured")
	case deps.Output == nil:
		return errors.New("output writer is not configured")
	case deps.Error == nil:
		return errors.New("error writer is not configured")
	default:
		return nil
	}
}

func handleDelete(ctx context.Context, client GitClient, in io.Reader, out, errOut io.Writer, branch string) error {
	result, err := client.DeleteBranch(ctx, branch, git.DeleteOptions{})
	if err == nil {
		printIfNotEmpty(out, result.Stdout)
		printIfNotEmpty(errOut, result.Stderr)
		return nil
	}

	if errors.Is(err, git.ErrBranchNotFullyMerged) {
		printIfNotEmpty(errOut, result.Stderr)
		confirmed, confirmErr := confirmBranchDeletion(in, out, branch)
		if confirmErr != nil {
			return confirmErr
		}
		if !confirmed {
			return fmt.Errorf("branch deletion aborted")
		}
		forcedResult, forceErr := client.DeleteBranch(ctx, branch, git.DeleteOptions{Force: true})
		if forceErr != nil {
			printIfNotEmpty(errOut, forcedResult.Stderr)
			return forceErr
		}
		printIfNotEmpty(out, forcedResult.Stdout)
		printIfNotEmpty(errOut, forcedResult.Stderr)
		return nil
	}

	printIfNotEmpty(errOut, result.Stderr)
	return err
}

func confirmBranchDeletion(in io.Reader, out io.Writer, branch string) (bool, error) {
	if _, err := fmt.Fprintf(out, "Branch '%s' is not fully merged. Delete anyway? [y/N]: ", branch); err != nil {
		return false, err
	}

	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return false, nil
	}

	answer := strings.ToLower(line)
	return answer == "y" || answer == "yes", nil
}

func printIfNotEmpty(w io.Writer, message string) {
	if trimmed := strings.TrimSpace(message); trimmed != "" {
		fmt.Fprintln(w, trimmed)
	}
}
