package git

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func (c *Client) CheckoutBranch(ctx context.Context, branch string) (string, error) {
	panic("not implemented in tests")
}

func TestExtractBranchFromSubject(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		subject string
		branch  string
		ok      bool
	}{
		"move-from": {
			subject: "checkout: moving from main to feature/test",
			branch:  "feature/test",
			ok:      true,
		},
		"switching": {
			subject: "checkout: switching to 'bugfix/issue-42'",
			branch:  "bugfix/issue-42",
			ok:      true,
		},
		"moving-to": {
			subject: "checkout: moving to release",
			branch:  "release",
			ok:      true,
		},
		"empty": {
			subject: "",
			branch:  "",
			ok:      false,
		},
		"unsupported": {
			subject: "commit: add feature",
			branch:  "",
			ok:      false,
		},
		"missing-destination": {
			subject: "checkout: moving from main",
			branch:  "",
			ok:      false,
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			branch, ok := extractBranchFromSubject(tc.subject)
			if branch != tc.branch || ok != tc.ok {
				t.Fatalf("extractBranchFromSubject(%q) = (%q, %v), want (%q, %v)", tc.subject, branch, ok, tc.branch, tc.ok)
			}
		})
	}
}

func TestParseReflogSubjects(t *testing.T) {
	t.Parallel()

	input := "checkout: moving from main to feature/one\ncheckout: switching to 'feature/two'\ncommit: add something"
	got := parseReflogSubjects(input)
	want := []string{"feature/one", "feature/two"}

	if len(got) != len(want) {
		t.Fatalf("parseReflogSubjects returned %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseReflogSubjects[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

type scriptCall struct {
	args   []string
	output string
	err    error
}

type scriptRunner struct {
	testingT *testing.T
	calls    []scriptCall
	index    int
}

func (r *scriptRunner) Run(ctx context.Context, args ...string) (string, error) {
	if r.index >= len(r.calls) {
		r.testingT.Fatalf("unexpected git invocation: %v", args)
	}
	call := r.calls[r.index]
	r.index++
	if !reflect.DeepEqual(call.args, args) {
		r.testingT.Fatalf("unexpected args at call %d: got %v, want %v", r.index, args, call.args)
	}
	return call.output, call.err
}

func (r *scriptRunner) Exhausted() bool {
	return r.index == len(r.calls)
}

func TestClientCheckoutBranch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	gitErr := errors.New("checkout failed")

	cases := map[string]struct {
		calls     []scriptCall
		branch    string
		wantOut   string
		wantErr   error
		wantCalls int
	}{
		"success": {
			branch: "feature/test",
			calls: []scriptCall{
				{args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, output: "main"},
				{args: []string{"checkout", "feature/test"}, output: "Switched to branch 'feature/test'"},
			},
			wantOut:   "Switched to branch 'feature/test'",
			wantCalls: 2,
		},
		"already-on": {
			branch: "feature/test",
			calls: []scriptCall{
				{args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, output: "feature/test"},
			},
			wantOut:   "already on 'feature/test'",
			wantCalls: 1,
		},
		"failure": {
			branch: "feature/test",
			calls: []scriptCall{
				{args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, output: "main"},
				{args: []string{"checkout", "feature/test"}, err: gitErr},
			},
			wantErr:   gitErr,
			wantCalls: 2,
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runner := &scriptRunner{testingT: t, calls: tc.calls}
			client := NewClient(runner)

			out, err := client.CheckoutBranch(ctx, tc.branch)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if out != tc.wantOut {
				t.Fatalf("unexpected output: got %q, want %q", out, tc.wantOut)
			}

			if !runner.Exhausted() {
				t.Fatalf("not all git calls were consumed: %d of %d", runner.index, len(runner.calls))
			}

			if runner.index != tc.wantCalls {
				t.Fatalf("unexpected number of git calls: got %d, want %d", runner.index, tc.wantCalls)
			}
		})
	}
}
