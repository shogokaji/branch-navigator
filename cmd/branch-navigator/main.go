package main

import (
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

func main() {
	checkout := flag.Bool("c", false, "checkout the selected branch (default)")
	merge := flag.Bool("m", false, "merge the selected branch into the current branch")
	deleteBranch := flag.Bool("d", false, "delete the selected local branch")
	limit := flag.Int("n", 10, "maximum number of branches to list")

	flag.Parse()

	act, err := resolveAction(*checkout, *merge, *deleteBranch)
	if err != nil {
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

	branches, err := nav.RecentBranches(ctx, *limit)
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

	switch act {
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
	default:
		fmt.Fprintf(os.Stderr, "%s action is not implemented yet\n", act)
		os.Exit(2)
	}
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
