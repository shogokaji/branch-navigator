package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"branch-navigator/internal/git"
	"branch-navigator/internal/ui"
)

type fakeGit struct {
	currentBranch string
	currentErr    error
	currentHook   func() (string, error)
	checkoutOut   string
	checkoutErr   error
	checkoutHook  func(string) (string, error)
	mergeResult   git.MergeResult
	mergeErr      error
	mergeHook     func(string, git.MergeOptions) (git.MergeResult, error)
	deleteResult  git.DeleteResult
	deleteErr     error
	deleteHook    func(string, git.DeleteOptions) (git.DeleteResult, error)
	checkoutCalls int
	mergeCalls    int
	deleteCalls   int
}

func (f *fakeGit) CurrentBranch(context.Context) (string, error) {
	if f.currentHook != nil {
		return f.currentHook()
	}
	return f.currentBranch, f.currentErr
}

func (f *fakeGit) CheckoutBranch(ctx context.Context, branch string) (string, error) {
	f.checkoutCalls++
	if f.checkoutHook != nil {
		return f.checkoutHook(branch)
	}
	return f.checkoutOut, f.checkoutErr
}

func (f *fakeGit) MergeBranch(ctx context.Context, branch string, opts git.MergeOptions) (git.MergeResult, error) {
	f.mergeCalls++
	if f.mergeHook != nil {
		return f.mergeHook(branch, opts)
	}
	return f.mergeResult, f.mergeErr
}

func (f *fakeGit) DeleteBranch(ctx context.Context, branch string, opts git.DeleteOptions) (git.DeleteResult, error) {
	f.deleteCalls++
	if f.deleteHook != nil {
		return f.deleteHook(branch, opts)
	}
	return f.deleteResult, f.deleteErr
}

type fakeNavigator struct {
	branches []string
	err      error
}

func (f *fakeNavigator) RecentBranches(ctx context.Context, limit int) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.branches, nil
}

type fakeTerminal struct {
	result ui.Result
	err    error
	last   []ui.Branch
}

func (f *fakeTerminal) Select(branches []ui.Branch) (ui.Result, error) {
	f.last = append([]ui.Branch(nil), branches...)
	if f.err != nil {
		return ui.Result{}, f.err
	}
	return f.result, nil
}

func TestRunCheckout(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	gitClient := &fakeGit{currentBranch: "main", checkoutOut: "Switched"}
	navigator := &fakeNavigator{branches: []string{"feature"}}
	terminal := &fakeTerminal{result: ui.Result{Branch: "feature"}}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run(ctx, Options{Action: ActionCheckout, Limit: 5}, Dependencies{
		Git:       gitClient,
		Navigator: navigator,
		Terminal:  terminal,
		Input:     strings.NewReader(""),
		Output:    stdout,
		Error:     stderr,
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if gitClient.checkoutCalls != 1 {
		t.Fatalf("expected checkout to be called once, got %d", gitClient.checkoutCalls)
	}
	if stdout.String() != "Switched\n" {
		t.Fatalf("expected checkout output, got %q", stdout.String())
	}
	if len(terminal.last) != 2 || terminal.last[0].Name != "main" || !terminal.last[0].Current {
		t.Fatalf("unexpected branches passed to UI: %+v", terminal.last)
	}
}

func TestRunMergeError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	gitClient := &fakeGit{currentBranch: "main", mergeResult: git.MergeResult{Stdout: "output", Stderr: "conflict"}, mergeErr: errors.New("merge failed")}
	navigator := &fakeNavigator{branches: []string{"feature"}}
	terminal := &fakeTerminal{result: ui.Result{Branch: "feature"}}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run(ctx, Options{Action: ActionMerge, Limit: 5}, Dependencies{
		Git:       gitClient,
		Navigator: navigator,
		Terminal:  terminal,
		Input:     strings.NewReader(""),
		Output:    stdout,
		Error:     stderr,
	})

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if gitClient.mergeCalls != 1 {
		t.Fatalf("expected merge to be called once, got %d", gitClient.mergeCalls)
	}
	if stdout.String() != "output\n" {
		t.Fatalf("expected merge stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "conflict") {
		t.Fatalf("expected conflict in stderr, got %q", stderr.String())
	}
}

func TestRunNavigatorFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	gitClient := &fakeGit{currentBranch: "main"}
	navigator := &fakeNavigator{err: errors.New("nav failed")}
	terminal := &fakeTerminal{}
	stderr := &bytes.Buffer{}

	code := Run(ctx, Options{Action: ActionCheckout, Limit: 5}, Dependencies{
		Git:       gitClient,
		Navigator: navigator,
		Terminal:  terminal,
		Input:     strings.NewReader(""),
		Output:    &bytes.Buffer{},
		Error:     stderr,
	})

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "nav failed") {
		t.Fatalf("expected navigator error in stderr, got %q", stderr.String())
	}
}

func TestRunQuitAndAlreadyOn(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	navigator := &fakeNavigator{branches: []string{"feature"}}

	quitTerminal := &fakeTerminal{result: ui.Result{Quit: true}}
	quitCode := Run(ctx, Options{Action: ActionCheckout, Limit: 5}, Dependencies{
		Git:       &fakeGit{currentBranch: "main"},
		Navigator: navigator,
		Terminal:  quitTerminal,
		Input:     strings.NewReader(""),
		Output:    &bytes.Buffer{},
		Error:     &bytes.Buffer{},
	})
	if quitCode != 0 {
		t.Fatalf("expected exit code 0 for quit, got %d", quitCode)
	}

	alreadyTerminal := &fakeTerminal{result: ui.Result{AlreadyOn: true, Branch: "main"}}
	alreadyCode := Run(ctx, Options{Action: ActionCheckout, Limit: 5}, Dependencies{
		Git:       &fakeGit{currentBranch: "main"},
		Navigator: navigator,
		Terminal:  alreadyTerminal,
		Input:     strings.NewReader(""),
		Output:    &bytes.Buffer{},
		Error:     &bytes.Buffer{},
	})
	if alreadyCode != 0 {
		t.Fatalf("expected exit code 0 when already on branch, got %d", alreadyCode)
	}
}

func TestRunDeleteForceFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	gitClient := &fakeGit{currentBranch: "main", deleteResult: git.DeleteResult{Stdout: "Deleted"}}
	navigator := &fakeNavigator{branches: []string{"feature"}}
	terminal := &fakeTerminal{result: ui.Result{Branch: "feature"}}
	stdout := &bytes.Buffer{}

	code := Run(ctx, Options{Action: ActionDelete, Limit: 5}, Dependencies{
		Git:       gitClient,
		Navigator: navigator,
		Terminal:  terminal,
		Input:     strings.NewReader(""),
		Output:    stdout,
		Error:     &bytes.Buffer{},
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if gitClient.deleteCalls != 1 {
		t.Fatalf("expected delete to be called once, got %d", gitClient.deleteCalls)
	}
	if stdout.String() != "Deleted\n" {
		t.Fatalf("expected delete output, got %q", stdout.String())
	}
}

func TestRunMergeSuccessWithWarnings(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	gitClient := &fakeGit{
		currentBranch: "main",
		mergeResult:   git.MergeResult{Stdout: "Already up to date", Stderr: "warning"},
	}
	navigator := &fakeNavigator{branches: []string{"feature"}}
	terminal := &fakeTerminal{result: ui.Result{Branch: "feature"}}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run(ctx, Options{Action: ActionMerge, Limit: 5}, Dependencies{
		Git:       gitClient,
		Navigator: navigator,
		Terminal:  terminal,
		Input:     strings.NewReader(""),
		Output:    stdout,
		Error:     stderr,
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if stdout.String() != "Already up to date\n" {
		t.Fatalf("unexpected merge stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning") {
		t.Fatalf("expected warning in stderr, got %q", stderr.String())
	}
}

func TestRunLimitValidationAndDeps(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	errBuf := &bytes.Buffer{}

	code := Run(ctx, Options{Action: ActionCheckout, Limit: 0}, Dependencies{
		Git:       &fakeGit{currentBranch: "main"},
		Navigator: &fakeNavigator{},
		Terminal:  &fakeTerminal{},
		Input:     strings.NewReader(""),
		Output:    &bytes.Buffer{},
		Error:     errBuf,
	})
	if code != 2 {
		t.Fatalf("expected exit code 2 for invalid limit, got %d", code)
	}
	if !strings.Contains(errBuf.String(), "limit must be greater than 0") {
		t.Fatalf("expected limit error, got %q", errBuf.String())
	}

	deps := Dependencies{}
	deps.Error = &bytes.Buffer{}
	code = Run(ctx, Options{Action: ActionCheckout, Limit: 5}, deps)
	if code != 1 {
		t.Fatalf("expected exit code 1 when deps missing, got %d", code)
	}
	if !strings.Contains(deps.Error.(*bytes.Buffer).String(), "git client is not configured") {
		t.Fatalf("expected dependency error, got %q", deps.Error.(*bytes.Buffer).String())
	}
}

func TestRunTerminalAndGitErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	gitErr := errors.New("current branch failed")
	gitClient := &fakeGit{currentHook: func() (string, error) { return "", gitErr }}
	deps := Dependencies{
		Git:       gitClient,
		Navigator: &fakeNavigator{},
		Terminal:  &fakeTerminal{},
		Input:     strings.NewReader(""),
		Output:    &bytes.Buffer{},
		Error:     &bytes.Buffer{},
	}

	code := Run(ctx, Options{Action: ActionCheckout, Limit: 5}, deps)
	if code != 1 {
		t.Fatalf("expected exit code 1 for git error, got %d", code)
	}
	if !strings.Contains(deps.Error.(*bytes.Buffer).String(), gitErr.Error()) {
		t.Fatalf("expected git error, got %q", deps.Error.(*bytes.Buffer).String())
	}

	uiErr := errors.New("ui failed")
	gitClient.currentHook = nil
	gitClient.currentBranch = "main"
	deps.Error = &bytes.Buffer{}
	deps.Terminal = &fakeTerminal{err: uiErr}
	code = Run(ctx, Options{Action: ActionCheckout, Limit: 5}, deps)
	if code != 1 {
		t.Fatalf("expected exit code 1 for UI error, got %d", code)
	}
	if !strings.Contains(deps.Error.(*bytes.Buffer).String(), uiErr.Error()) {
		t.Fatalf("expected UI error, got %q", deps.Error.(*bytes.Buffer).String())
	}
}

func TestRunDeleteNotFullyMergedConfirm(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var deleteCalls []git.DeleteOptions
	gitClient := &fakeGit{currentBranch: "main"}
	gitClient.deleteHook = func(branch string, opts git.DeleteOptions) (git.DeleteResult, error) {
		deleteCalls = append(deleteCalls, opts)
		if len(deleteCalls) == 1 {
			return git.DeleteResult{Stderr: "not fully merged"}, fmt.Errorf("%w", git.ErrBranchNotFullyMerged)
		}
		return git.DeleteResult{Stdout: "Deleted forcefully"}, nil
	}

	input := strings.NewReader("y\n")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run(ctx, Options{Action: ActionDelete, Limit: 5}, Dependencies{
		Git:       gitClient,
		Navigator: &fakeNavigator{branches: []string{"feature"}},
		Terminal:  &fakeTerminal{result: ui.Result{Branch: "feature"}},
		Input:     input,
		Output:    stdout,
		Error:     stderr,
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if len(deleteCalls) != 2 || deleteCalls[0].Force || !deleteCalls[1].Force {
		t.Fatalf("unexpected delete call sequence: %+v", deleteCalls)
	}
	if !strings.Contains(stdout.String(), "Deleted forcefully") {
		t.Fatalf("expected forced delete output, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "not fully merged") {
		t.Fatalf("expected warning in stderr, got %q", stderr.String())
	}
}

func TestHandleDeleteAbort(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	abortErr := fmt.Errorf("%w", git.ErrBranchNotFullyMerged)
	gitClient := &fakeGit{deleteHook: func(branch string, opts git.DeleteOptions) (git.DeleteResult, error) {
		if opts.Force {
			return git.DeleteResult{Stdout: "forced"}, nil
		}
		return git.DeleteResult{Stderr: "not fully merged"}, abortErr
	}}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	err := handleDelete(ctx, gitClient, strings.NewReader("n\n"), stdout, stderr, "feature")
	if err == nil || err.Error() != "branch deletion aborted" {
		t.Fatalf("expected abort error, got %v", err)
	}
	if !strings.Contains(stderr.String(), "not fully merged") {
		t.Fatalf("expected warning logged, got %q", stderr.String())
	}
}

func TestValidateDeps(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		deps Dependencies
		want string
	}{
		{"missing-git", Dependencies{}, "git client"},
		{"missing-navigator", Dependencies{Git: &fakeGit{}}, "navigator"},
		{"missing-terminal", Dependencies{Git: &fakeGit{}, Navigator: &fakeNavigator{}}, "terminal"},
		{"missing-input", Dependencies{Git: &fakeGit{}, Navigator: &fakeNavigator{}, Terminal: &fakeTerminal{}}, "input"},
		{"missing-output", Dependencies{Git: &fakeGit{}, Navigator: &fakeNavigator{}, Terminal: &fakeTerminal{}, Input: strings.NewReader("")}, "output"},
		{"missing-error", Dependencies{Git: &fakeGit{}, Navigator: &fakeNavigator{}, Terminal: &fakeTerminal{}, Input: strings.NewReader(""), Output: &bytes.Buffer{}}, "error"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateDeps(tc.deps)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}

	if err := validateDeps(Dependencies{
		Git:       &fakeGit{},
		Navigator: &fakeNavigator{},
		Terminal:  &fakeTerminal{},
		Input:     strings.NewReader(""),
		Output:    &bytes.Buffer{},
		Error:     &bytes.Buffer{},
	}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

var _ GitClient = (*fakeGit)(nil)
var _ Navigator = (*fakeNavigator)(nil)
var _ Terminal = (*fakeTerminal)(nil)
