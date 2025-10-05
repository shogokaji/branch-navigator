package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"branch-navigator/internal/git"
	"branch-navigator/internal/navigator"
	"branch-navigator/internal/ui"
)

type action string

const (
	actionCheckout action = "checkout"
	actionMerge    action = "merge"
	actionDelete   action = "delete"
)

const usageText = `Usage: branch-navigator [-c|-m|-d] [-n N] [-h]

Options:
  -c	checkout the selected branch (default)
  -m	merge the selected branch into the current branch
  -d	delete the selected local branch
  -n	maximum number of branches to list (default 10)
      --limit N	alias for -n
  -h	show this help message
`

type cliOptions struct {
	action action
	limit  int
}

func main() {
	opts, err := parseArgs(os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx := context.Background()
	client := git.NewDefaultClient()
	nav, err := navigator.New(client)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	branches, err := nav.RecentBranches(ctx, opts.limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	current, err := client.CurrentBranch(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	uiBranches := make([]ui.Branch, 0, len(branches)+1)
	uiBranches = append(uiBranches, ui.Branch{Name: current, Current: true})
	for _, branch := range branches {
		uiBranches = append(uiBranches, ui.Branch{Name: branch})
	}

	terminal := ui.New(os.Stdin, os.Stdout)
	result, err := terminal.Select(uiBranches)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if result.Quit || result.AlreadyOn {
		return
	}

	switch opts.action {
	case actionCheckout:
		message, err := client.CheckoutBranch(ctx, result.Branch)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		printIfNotEmpty(os.Stdout, message)
	case actionMerge:
		mergeResult, err := client.MergeBranch(ctx, result.Branch, git.MergeOptions{})
		printIfNotEmpty(os.Stdout, mergeResult.Stdout)
		stderrOutput := strings.TrimSpace(mergeResult.Stderr)
		if err != nil {
			if stderrOutput != "" {
				fmt.Fprintln(os.Stderr, stderrOutput)
				if !strings.Contains(err.Error(), stderrOutput) {
					fmt.Fprintln(os.Stderr, err)
				}
			} else {
				fmt.Fprintln(os.Stderr, err)
			}
			os.Exit(1)
		}
		if stderrOutput != "" {
			fmt.Fprintln(os.Stderr, stderrOutput)
		}
	case actionDelete:
		if err := handleDeleteAction(ctx, client, os.Stdin, os.Stdout, os.Stderr, result.Branch); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "%s action is not implemented yet\n", opts.action)
		os.Exit(2)
	}
}

func parseArgs(args []string, usageOut, errorOut io.Writer) (cliOptions, error) {
	fs := flag.NewFlagSet("branch-navigator", flag.ContinueOnError)
	fs.SetOutput(errorOut)

	fs.Usage = func() {
		fmt.Fprint(usageOut, usageText)
	}

	opts := cliOptions{limit: 10}
	checkout := fs.Bool("c", false, "checkout the selected branch (default)")
	merge := fs.Bool("m", false, "merge the selected branch into the current branch")
	deleteBranch := fs.Bool("d", false, "delete the selected local branch")
	fs.IntVar(&opts.limit, "n", 10, "maximum number of branches to list")
	fs.IntVar(&opts.limit, "limit", 10, "maximum number of branches to list")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return cliOptions{}, flag.ErrHelp
		}
		return cliOptions{}, err
	}

	act, err := resolveAction(*checkout, *merge, *deleteBranch)
	if err != nil {
		return cliOptions{}, err
	}

	if opts.limit <= 0 {
		return cliOptions{}, fmt.Errorf("limit must be greater than 0")
	}

	opts.action = act
	return opts, nil
}

func resolveAction(checkout, merge, deleteBranch bool) (action, error) {
	selected := []action{}
	if checkout {
		selected = append(selected, actionCheckout)
	}
	if merge {
		selected = append(selected, actionMerge)
	}
	if deleteBranch {
		selected = append(selected, actionDelete)
	}

	switch len(selected) {
	case 0:
		return actionCheckout, nil
	case 1:
		return selected[0], nil
	default:
		return "", errors.New("only one of -c, -m, or -d may be specified")
	}
}

func printIfNotEmpty(w io.Writer, message string) {
	if trimmed := strings.TrimSpace(message); trimmed != "" {
		fmt.Fprintln(w, trimmed)
	}
}

func handleDeleteAction(ctx context.Context, client *git.Client, in io.Reader, out, errOut io.Writer, branch string) error {
	if client == nil {
		return fmt.Errorf("git client is not configured")
	}

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
