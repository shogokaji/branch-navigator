package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"branch-navigator/internal/app"
	"branch-navigator/internal/git"
	"branch-navigator/internal/navigator"
	"branch-navigator/internal/ui"
)

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, app.Run))
}

func resolveAction(checkout, merge, deleteBranch bool) (app.Action, error) {
	selected := []app.Action{}
	if checkout {
		selected = append(selected, app.ActionCheckout)
	}
	if merge {
		selected = append(selected, app.ActionMerge)
	}
	if deleteBranch {
		selected = append(selected, app.ActionDelete)
	}

	switch len(selected) {
	case 0:
		return app.ActionCheckout, nil
	case 1:
		return selected[0], nil
	default:
		return "", errors.New("only one of -c, -m, or -d may be specified")
	}
}

type gitProvider interface {
	app.GitClient
	navigator.GitService
}

var (
	gitFactory        = func() gitProvider { return git.NewDefaultClient() }
	navigatorFactory  = func(gitSvc navigator.GitService) (app.Navigator, error) { return navigator.New(gitSvc) }
	terminalFactory   = func(in io.Reader, out io.Writer) app.Terminal { return ui.New(in, out) }
	backgroundContext = func() context.Context { return context.Background() }
)

func runCLI(args []string, stdin io.Reader, stdout, stderr io.Writer, runner func(context.Context, app.Options, app.Dependencies) int) int {
	fs := flag.NewFlagSet("branch-navigator", flag.ContinueOnError)
	fs.SetOutput(stderr)
	checkout := fs.Bool("c", false, "checkout the selected branch (default)")
	merge := fs.Bool("m", false, "merge the selected branch into the current branch")
	deleteBranch := fs.Bool("d", false, "delete the selected local branch")
	limit := fs.Int("n", 10, "maximum number of branches to list")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	action, err := resolveAction(*checkout, *merge, *deleteBranch)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	ctx := backgroundContext()
	gitClient := gitFactory()
	navigatorClient, err := navigatorFactory(gitClient)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	terminal := terminalFactory(stdin, stdout)
	deps := app.Dependencies{
		Git:       gitClient,
		Navigator: navigatorClient,
		Terminal:  terminal,
		Input:     stdin,
		Output:    stdout,
		Error:     stderr,
	}

	return runner(ctx, app.Options{Action: action, Limit: *limit}, deps)
}
