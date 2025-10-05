package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"branch-navigator/internal/app"
	"branch-navigator/internal/git"
	"branch-navigator/internal/navigator"
	"branch-navigator/internal/ui"
)

type stubGit struct {
	current    string
	reflog     []string
	branches   []string
	branchSet  map[string]bool
	checkoutTo string
	mergeErr   error
	deleteErr  error
}

func (s *stubGit) CurrentBranch(context.Context) (string, error) {
	if s.current == "" {
		return "main", nil
	}
	return s.current, nil
}

func (s *stubGit) ReflogBranchMoves(context.Context) ([]string, error) {
	return append([]string(nil), s.reflog...), nil
}

func (s *stubGit) BranchesByCommitDate(context.Context) ([]string, error) {
	return append([]string(nil), s.branches...), nil
}

func (s *stubGit) BranchExists(ctx context.Context, branch string) (bool, error) {
	if s.branchSet == nil {
		return true, nil
	}
	exists, ok := s.branchSet[branch]
	if !ok {
		return false, nil
	}
	return exists, nil
}

func (s *stubGit) CheckoutBranch(ctx context.Context, branch string) (string, error) {
	s.checkoutTo = branch
	return "checked", nil
}

func (s *stubGit) MergeBranch(ctx context.Context, branch string, opts git.MergeOptions) (git.MergeResult, error) {
	return git.MergeResult{}, s.mergeErr
}

func (s *stubGit) DeleteBranch(ctx context.Context, branch string, opts git.DeleteOptions) (git.DeleteResult, error) {
	return git.DeleteResult{}, s.deleteErr
}

type stubNavigator struct {
	branches []string
	err      error
}

func (s *stubNavigator) RecentBranches(context.Context, int) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]string(nil), s.branches...), nil
}

type stubTerminal struct {
	result ui.Result
	err    error
}

func (s *stubTerminal) Select([]ui.Branch) (ui.Result, error) {
	if s.err != nil {
		return ui.Result{}, s.err
	}
	return s.result, nil
}

type runnerCall struct {
	options app.Options
	deps    app.Dependencies
}

func TestResolveAction(t *testing.T) {
	t.Parallel()

	action, err := resolveAction(false, false, false)
	if err != nil || action != app.ActionCheckout {
		t.Fatalf("expected checkout action, got %v, %v", action, err)
	}

	action, err = resolveAction(false, true, false)
	if err != nil || action != app.ActionMerge {
		t.Fatalf("expected merge action, got %v, %v", action, err)
	}

	action, err = resolveAction(false, false, true)
	if err != nil || action != app.ActionDelete {
		t.Fatalf("expected delete action, got %v, %v", action, err)
	}

	if _, err := resolveAction(true, true, false); err == nil {
		t.Fatal("expected error when multiple actions specified")
	}
}

func TestRunCLISuccess(t *testing.T) {
	t.Parallel()

	origGitFactory := gitFactory
	origNavigatorFactory := navigatorFactory
	origTerminalFactory := terminalFactory
	origContext := backgroundContext
	defer func() {
		gitFactory = origGitFactory
		navigatorFactory = origNavigatorFactory
		terminalFactory = origTerminalFactory
		backgroundContext = origContext
	}()

	gitStub := &stubGit{current: "main"}
	navStub := &stubNavigator{branches: []string{"feature"}}
	termStub := &stubTerminal{result: ui.Result{Branch: "feature"}}

	gitFactory = func() gitProvider { return gitStub }
	navigatorFactory = func(g navigator.GitService) (app.Navigator, error) {
		return navStub, nil
	}
	terminalFactory = func(in io.Reader, out io.Writer) app.Terminal { return termStub }
	backgroundContext = func() context.Context { return context.Background() }

	var calls []runnerCall
	runner := func(ctx context.Context, opts app.Options, deps app.Dependencies) int {
		calls = append(calls, runnerCall{options: opts, deps: deps})
		return 0
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runCLI([]string{"-n", "7"}, strings.NewReader(""), stdout, stderr, runner)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if len(calls) != 1 {
		t.Fatalf("expected runner to be invoked once, got %d", len(calls))
	}
	if calls[0].options.Limit != 7 || calls[0].options.Action != app.ActionCheckout {
		t.Fatalf("unexpected options: %+v", calls[0].options)
	}
}

func TestRunCLIParseError(t *testing.T) {
	t.Parallel()

	stderr := &bytes.Buffer{}
	code := runCLI([]string{"-n", "bad"}, strings.NewReader(""), &bytes.Buffer{}, stderr, func(context.Context, app.Options, app.Dependencies) int { return 0 })
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "parse error") {
		t.Fatalf("expected parse error, got %q", stderr.String())
	}
}

func TestRunCLINavigatorError(t *testing.T) {
	t.Parallel()

	origGitFactory := gitFactory
	origNavigatorFactory := navigatorFactory
	origTerminalFactory := terminalFactory
	defer func() {
		gitFactory = origGitFactory
		navigatorFactory = origNavigatorFactory
		terminalFactory = origTerminalFactory
	}()

	gitFactory = func() gitProvider { return &stubGit{} }
	called := false
	navigatorFactory = func(g navigator.GitService) (app.Navigator, error) {
		called = true
		return nil, errors.New("nav failed")
	}
	terminalFactory = func(in io.Reader, out io.Writer) app.Terminal { return &stubTerminal{} }

	stderr := &bytes.Buffer{}
	code := runCLI(nil, strings.NewReader(""), &bytes.Buffer{}, stderr, func(context.Context, app.Options, app.Dependencies) int { return 0 })
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !called {
		t.Fatal("expected navigatorFactory to be invoked")
	}
	if !strings.Contains(stderr.String(), "nav failed") {
		t.Fatalf("expected navigator error, got %q", stderr.String())
	}
}

var _ app.GitClient = (*stubGit)(nil)
var _ navigator.GitService = (*stubGit)(nil)
var _ app.Navigator = (*stubNavigator)(nil)
var _ app.Terminal = (*stubTerminal)(nil)
