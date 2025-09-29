package navigator

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type stubGit struct {
	current    string
	history    []string
	locals     []string
	currentErr error
	historyErr error
	localsErr  error
}

func (s *stubGit) CurrentBranch(context.Context) (string, error) {
	if s.currentErr != nil {
		return "", s.currentErr
	}
	return s.current, nil
}

func (s *stubGit) ReflogBranchVisits(context.Context) ([]string, error) {
	if s.historyErr != nil {
		return nil, s.historyErr
	}
	return append([]string(nil), s.history...), nil
}

func (s *stubGit) LocalBranches(context.Context) ([]string, error) {
	if s.localsErr != nil {
		return nil, s.localsErr
	}
	return append([]string(nil), s.locals...), nil
}

func TestNewNavigatorValidatesGit(t *testing.T) {
	if _, err := NewNavigator(nil); err == nil {
		t.Fatalf("expected error when Git is nil")
	}
}

func TestRecentBranchesUsesHistoryFirst(t *testing.T) {
	gitStub := &stubGit{
		current: "main",
		locals:  []string{"feature/b", "feature/a", "main"},
		history: []string{"feature/b", "main", "feature/a", "feature/b"},
	}

	navigator, err := NewNavigator(gitStub)
	if err != nil {
		t.Fatalf("NewNavigator error: %v", err)
	}

	got, err := navigator.RecentBranches(context.Background(), 3)
	if err != nil {
		t.Fatalf("RecentBranches error: %v", err)
	}

	want := []string{"feature/b", "feature/a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestRecentBranchesFallsBackToLocalBranches(t *testing.T) {
	gitStub := &stubGit{
		current: "main",
		locals:  []string{"feature/a", "feature/b", "main"},
		history: []string{"origin/main"},
	}

	navigator, err := NewNavigator(gitStub)
	if err != nil {
		t.Fatalf("NewNavigator error: %v", err)
	}

	got, err := navigator.RecentBranches(context.Background(), 2)
	if err != nil {
		t.Fatalf("RecentBranches error: %v", err)
	}

	want := []string{"feature/a", "feature/b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestRecentBranchesLimitZero(t *testing.T) {
	gitStub := &stubGit{current: "main"}
	navigator, err := NewNavigator(gitStub)
	if err != nil {
		t.Fatalf("NewNavigator error: %v", err)
	}

	got, err := navigator.RecentBranches(context.Background(), 0)
	if err != nil {
		t.Fatalf("RecentBranches error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestRecentBranchesPropagatesErrors(t *testing.T) {
	currentErr := errors.New("current branch error")
	gitStub := &stubGit{currentErr: currentErr}
	navigator, err := NewNavigator(gitStub)
	if err != nil {
		t.Fatalf("NewNavigator error: %v", err)
	}
	if _, err := navigator.RecentBranches(context.Background(), 5); !errors.Is(err, currentErr) {
		t.Fatalf("expected error %v, got %v", currentErr, err)
	}
}
