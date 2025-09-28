package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
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

	fmt.Printf("branch-navigator TODO: action=%s, limit=%d\n", act, *limit)
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
