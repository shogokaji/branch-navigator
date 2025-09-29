package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type stubRunner struct {
	responses map[string]stubResponse
}

type stubResponse struct {
	output string
	err    error
}

func (s stubRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	key := strings.Join(args, "\x00")
	resp, ok := s.responses[key]
	if !ok {
		return nil, fmt.Errorf("unexpected command: %s", strings.Join(args, " "))
	}
	return []byte(resp.output), resp.err
}

func TestClientCurrentBranch(t *testing.T) {
	runner := stubRunner{responses: map[string]stubResponse{
		strings.Join([]string{"rev-parse", "--abbrev-ref", "HEAD"}, "\x00"): {output: "main\n"},
	}}

	client := NewClient(runner)
	branch, err := client.CurrentBranch(context.Background())
	if err != nil {
		t.Fatalf("CurrentBranch error: %v", err)
	}
	if branch != "main" {
		t.Fatalf("expected main, got %q", branch)
	}
}

func TestClientReflogBranchVisits(t *testing.T) {
	runner := stubRunner{responses: map[string]stubResponse{
		strings.Join([]string{"reflog", "--format=%gs"}, "\x00"): {output: strings.Join([]string{
			"checkout: moving from feature/foo to main",
			"switch branch: from main to feature/bar",
			"switch to branch 'bugfix'",
			"checkout: moving from bugfix to HEAD^",
			"merge feature/bar",
		}, "\n")},
	}}

	client := NewClient(runner)
	branches, err := client.ReflogBranchVisits(context.Background())
	if err != nil {
		t.Fatalf("ReflogBranchVisits error: %v", err)
	}

	expected := []string{"main", "feature/bar", "bugfix"}
	if len(branches) != len(expected) {
		t.Fatalf("expected %d branches, got %d (%v)", len(expected), len(branches), branches)
	}
	for i, want := range expected {
		if branches[i] != want {
			t.Fatalf("index %d: want %q, got %q", i, want, branches[i])
		}
	}
}

func TestClientReflogBranchVisitsEmpty(t *testing.T) {
	runner := stubRunner{responses: map[string]stubResponse{
		strings.Join([]string{"reflog", "--format=%gs"}, "\x00"): {output: ""},
	}}

	client := NewClient(runner)
	branches, err := client.ReflogBranchVisits(context.Background())
	if err != nil {
		t.Fatalf("ReflogBranchVisits error: %v", err)
	}
	if len(branches) != 0 {
		t.Fatalf("expected no branches, got %v", branches)
	}
}

func TestClientLocalBranches(t *testing.T) {
	runner := stubRunner{responses: map[string]stubResponse{
		strings.Join([]string{"for-each-ref", "--sort=-committerdate", "--format=%(refname:short)", "refs/heads"}, "\x00"): {output: "main\nfeature/foo\n\n"},
	}}

	client := NewClient(runner)
	branches, err := client.LocalBranches(context.Background())
	if err != nil {
		t.Fatalf("LocalBranches error: %v", err)
	}
	expected := []string{"main", "feature/foo"}
	if len(branches) != len(expected) {
		t.Fatalf("expected %d branches, got %d", len(expected), len(branches))
	}
	for i, want := range expected {
		if branches[i] != want {
			t.Fatalf("index %d: want %q, got %q", i, want, branches[i])
		}
	}
}

func TestClientRunErrorPropagation(t *testing.T) {
	stubErr := errors.New("boom")
	runner := stubRunner{responses: map[string]stubResponse{
		strings.Join([]string{"rev-parse", "--abbrev-ref", "HEAD"}, "\x00"): {output: "fatal: not a git repository\n", err: stubErr},
	}}

	client := NewClient(runner)
	_, err := client.CurrentBranch(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "fatal: not a git repository") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestExtractBranchFromReflogSubject(t *testing.T) {
	cases := map[string]string{
		"checkout: moving from main to feature/foo": "feature/foo",
		"checkout: moving to feature/bar":           "feature/bar",
		"switch branch: from main to bugfix":        "bugfix",
		"switch to branch 'topic/test'":             "topic/test",
		"reset: moving to release":                  "release",
		"checkout: moving from main to HEAD^":       "",
		"unrelated message":                         "",
		"checkout: moving from main to origin/main": "origin/main",
		"switch to branch 'HEAD~1'":                 "",
	}

	for input, want := range cases {
		got := extractBranchFromReflogSubject(input)
		if got != want {
			t.Errorf("input %q: want %q, got %q", input, want, got)
		}
	}
}
