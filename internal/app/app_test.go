package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"branch-navigator/internal/git"
	"branch-navigator/internal/ui"
)

type fakeGit struct {
	currentBranch string
	currentErr    error
	checkoutOut   string
	checkoutErr   error
	mergeResult   git.MergeResult
	mergeErr      error
	deleteResult  git.DeleteResult
	deleteErr     error
	checkoutCalls int
	mergeCalls    int
	deleteCalls   int
}

func (f *fakeGit) CurrentBranch(context.Context) (string, error) {
	return f.currentBranch, f.currentErr
}

func (f *fakeGit) CheckoutBranch(ctx context.Context, branch string) (string, error) {
	f.checkoutCalls++
	return f.checkoutOut, f.checkoutErr
}

func (f *fakeGit) MergeBranch(ctx context.Context, branch string, _ git.MergeOptions) (git.MergeResult, error) {
	f.mergeCalls++
	return f.mergeResult, f.mergeErr
}

func (f *fakeGit) DeleteBranch(ctx context.Context, branch string, _ git.DeleteOptions) (git.DeleteResult, error) {
	f.deleteCalls++
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

var _ GitClient = (*fakeGit)(nil)
var _ Navigator = (*fakeNavigator)(nil)
var _ Terminal = (*fakeTerminal)(nil)
