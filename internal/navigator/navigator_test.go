package navigator

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeGit struct {
	current      string
	reflog       []string
	fallback     []string
	exists       map[string]bool
	errCurrent   error
	errReflog    error
	errFallback  error
	errExists    error
	existsErrFor string
}

func (f *fakeGit) CurrentBranch(ctx context.Context) (string, error) {
	return f.current, f.errCurrent
}

func (f *fakeGit) ReflogBranchMoves(ctx context.Context) ([]string, error) {
	if f.errReflog != nil {
		return nil, f.errReflog
	}
	return append([]string(nil), f.reflog...), nil
}

func (f *fakeGit) BranchesByCommitDate(ctx context.Context) ([]string, error) {
	if f.errFallback != nil {
		return nil, f.errFallback
	}
	return append([]string(nil), f.fallback...), nil
}

func (f *fakeGit) BranchExists(ctx context.Context, branch string) (bool, error) {
	if f.errExists != nil && (f.existsErrFor == "" || f.existsErrFor == branch) {
		return false, f.errExists
	}
	if f.exists == nil {
		return false, nil
	}
	return f.exists[branch], nil
}

func TestNew(t *testing.T) {
	t.Parallel()

	if _, err := New(nil); err == nil {
		t.Fatal("expected error when git service is nil")
	}

	nav, err := New(&fakeGit{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nav == nil {
		t.Fatal("expected non-nil navigator")
	}
}

func TestNavigatorRecentBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	errExists := errors.New("branch exists failure")
	reflogUnavailable := errors.New("reflog unavailable")
	reflogFailed := errors.New("reflog failed")
	fallbackFailed := errors.New("fallback failed")

	cases := map[string]struct {
		limit           int
		git             *fakeGit
		want            []string
		wantErr         error
		wantErrContains []error
	}{
		"reflog-only": {
			limit: 3,
			git: &fakeGit{
				current:  "main",
				reflog:   []string{"feature/one", "main", "feature/two"},
				fallback: []string{"main", "feature/three"},
				exists: map[string]bool{
					"feature/one":   true,
					"feature/two":   true,
					"feature/three": true,
				},
			},
			want: []string{"feature/one", "feature/two", "feature/three"},
		},
		"fallback-needed": {
			limit: 2,
			git: &fakeGit{
				current:  "main",
				reflog:   []string{"origin/main", "feature/x"},
				fallback: []string{"feature/y"},
				exists: map[string]bool{
					"feature/x": true,
					"feature/y": true,
				},
			},
			want: []string{"feature/x", "feature/y"},
		},
		"limit-zero": {
			limit: 0,
			git:   &fakeGit{current: "main"},
			want:  nil,
		},
		"reflog-error-fallback": {
			limit: 2,
			git: &fakeGit{
				current:   "main",
				errReflog: reflogUnavailable,
				fallback:  []string{"feature/a", "feature/b"},
				exists: map[string]bool{
					"feature/a": true,
					"feature/b": true,
				},
			},
			want: []string{"feature/a", "feature/b"},
		},
		"branch-exists-error": {
			limit: 1,
			git: &fakeGit{
				current:      "main",
				reflog:       []string{"feature/broken"},
				exists:       map[string]bool{"feature/broken": true},
				errExists:    errExists,
				existsErrFor: "feature/broken",
			},
			wantErr: errExists,
		},
		"reflog-and-fallback-error": {
			limit: 2,
			git: &fakeGit{
				current:     "main",
				errReflog:   reflogFailed,
				errFallback: fallbackFailed,
			},
			wantErr: fallbackFailed,
			wantErrContains: []error{
				reflogFailed,
				fallbackFailed,
			},
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			nav, err := New(tc.git)
			if err != nil {
				t.Fatalf("New returned error: %v", err)
			}

			got, err := nav.RecentBranches(ctx, tc.limit)
			if tc.wantErr != nil || len(tc.wantErrContains) > 0 {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				for _, expected := range tc.wantErrContains {
					if !errors.Is(err, expected) {
						t.Fatalf("expected error to include %v, got %v", expected, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unexpected branches: got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNavigatorMissingConfiguration(t *testing.T) {
	t.Parallel()

	var nav *Navigator
	if _, err := nav.RecentBranches(context.Background(), 5); err == nil {
		t.Fatal("expected error when navigator is nil")
	}
}
