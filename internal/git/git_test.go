package git

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

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
	stdout string
	stderr string
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
	return call.stdout, call.err
}

func (r *scriptRunner) RunWithCombinedOutput(ctx context.Context, args ...string) (string, string, error) {
	if r.index >= len(r.calls) {
		r.testingT.Fatalf("unexpected git invocation: %v", args)
	}
	call := r.calls[r.index]
	r.index++
	if !reflect.DeepEqual(call.args, args) {
		r.testingT.Fatalf("unexpected args at call %d: got %v, want %v", r.index, args, call.args)
	}
	return call.stdout, call.stderr, call.err
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
				{args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, stdout: "main"},
				{args: []string{"checkout", "feature/test"}, stdout: "Switched to branch 'feature/test'"},
			},
			wantOut:   "Switched to branch 'feature/test'",
			wantCalls: 2,
		},
		"already-on": {
			branch: "feature/test",
			calls: []scriptCall{
				{args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, stdout: "feature/test"},
			},
			wantOut:   "already on 'feature/test'",
			wantCalls: 1,
		},
		"failure": {
			branch: "feature/test",
			calls: []scriptCall{
				{args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, stdout: "main"},
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

func TestClientMergeBranch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mergeErr := errors.New("merge failed")

	cases := map[string]struct {
		calls   []scriptCall
		branch  string
		stdout  string
		stderr  string
		wantErr error
	}{
		"success": {
			branch: "feature/topic",
			stdout: "Updating abc..def",
			stderr: "",
			calls: []scriptCall{
				{args: []string{"merge", "feature/topic"}, stdout: "Updating abc..def"},
			},
		},
		"conflict": {
			branch:  "feature/topic",
			stdout:  "Auto-merging file.go",
			stderr:  "CONFLICT (content): Merge conflict in file.go",
			wantErr: mergeErr,
			calls: []scriptCall{
				{args: []string{"merge", "feature/topic"}, stdout: "Auto-merging file.go", stderr: "CONFLICT (content): Merge conflict in file.go", err: mergeErr},
			},
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runner := &scriptRunner{testingT: t, calls: tc.calls}
			client := NewClient(runner)

			result, err := client.MergeBranch(ctx, tc.branch, MergeOptions{})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Stdout != tc.stdout {
				t.Fatalf("unexpected stdout: got %q, want %q", result.Stdout, tc.stdout)
			}
			if result.Stderr != tc.stderr {
				t.Fatalf("unexpected stderr: got %q, want %q", result.Stderr, tc.stderr)
			}

			if !runner.Exhausted() {
				t.Fatalf("not all git calls were consumed: %d of %d", runner.index, len(runner.calls))
			}
		})
	}
}

func TestClientCurrentBranch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	errRunner := errors.New("rev-parse failed")

	cases := map[string]struct {
		client  *Client
		want    string
		wantErr error
	}{
		"success": {
			client: NewClient(&scriptRunner{
				testingT: t,
				calls: []scriptCall{
					{args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, stdout: "main"},
				},
			}),
			want: "main",
		},
		"runner-error": {
			client: NewClient(&scriptRunner{
				testingT: t,
				calls: []scriptCall{
					{args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, err: errRunner},
				},
			}),
			wantErr: errRunner,
		},
		"nil-client": {
			client:  nil,
			wantErr: errors.New("git client is not configured"),
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var (
				got string
				err error
			)
			if tc.client == nil {
				var nilClient *Client
				got, err = nilClient.CurrentBranch(ctx)
			} else {
				got, err = tc.client.CurrentBranch(ctx)
			}
			if tc.wantErr != nil {
				if err == nil || err.Error() != tc.wantErr.Error() {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Fatalf("unexpected branch: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClientReflogBranchMoves(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	reflogOutput := "checkout: moving from main to feature/one\ncheckout: switching to feature/two"
	reflogErr := errors.New("reflog failed")

	client := NewClient(&scriptRunner{
		testingT: t,
		calls: []scriptCall{
			{args: []string{"reflog", "--format=%gs"}, stdout: reflogOutput},
		},
	})

	branches, err := client.ReflogBranchMoves(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(branches, []string{"feature/one", "feature/two"}) {
		t.Fatalf("unexpected branches: %v", branches)
	}

	failing := NewClient(&scriptRunner{
		testingT: t,
		calls: []scriptCall{
			{args: []string{"reflog", "--format=%gs"}, err: reflogErr},
		},
	})

	if _, err := failing.ReflogBranchMoves(ctx); !errors.Is(err, reflogErr) {
		t.Fatalf("expected reflog error, got %v", err)
	}
}

func TestClientBranchesByCommitDate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	branches := "feature/one\nfeature/two"
	listErr := errors.New("for-each-ref failed")

	client := NewClient(&scriptRunner{
		testingT: t,
		calls: []scriptCall{
			{args: []string{"for-each-ref", "--format=%(refname:short)", "--sort=-committerdate", "refs/heads"}, stdout: branches},
		},
	})

	got, err := client.BranchesByCommitDate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"feature/one", "feature/two"}) {
		t.Fatalf("unexpected branches: %v", got)
	}

	failing := NewClient(&scriptRunner{
		testingT: t,
		calls: []scriptCall{
			{args: []string{"for-each-ref", "--format=%(refname:short)", "--sort=-committerdate", "refs/heads"}, err: listErr},
		},
	})
	if _, err := failing.BranchesByCommitDate(ctx); !errors.Is(err, listErr) {
		t.Fatalf("expected error %v, got %v", listErr, err)
	}
}

func TestClientBranchExists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	exitCmd := exec.Command("sh", "-c", "exit 1")
	showErr := exitCmd.Run()
	otherErr := errors.New("show-ref failed")

	cases := map[string]struct {
		calls   []scriptCall
		branch  string
		want    bool
		wantErr error
	}{
		"exists": {
			branch: "feature/one",
			calls: []scriptCall{
				{args: []string{"show-ref", "--verify", "--quiet", "refs/heads/feature/one"}},
			},
			want: true,
		},
		"missing": {
			branch: "feature/two",
			calls: []scriptCall{
				{args: []string{"show-ref", "--verify", "--quiet", "refs/heads/feature/two"}, err: showErr},
			},
			want: false,
		},
		"empty-name": {
			branch: "",
			want:   false,
		},
		"other-error": {
			branch: "feature/bad",
			calls: []scriptCall{
				{args: []string{"show-ref", "--verify", "--quiet", "refs/heads/feature/bad"}, err: otherErr},
			},
			wantErr: otherErr,
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runner := &scriptRunner{testingT: t, calls: tc.calls}
			client := NewClient(runner)

			got, err := client.BranchExists(ctx, tc.branch)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Fatalf("BranchExists(%q) = %v, want %v", tc.branch, got, tc.want)
			}
		})
	}
}

func TestClientDeleteBranch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	gitErr := errors.New("git delete failed")

	cases := map[string]struct {
		calls      []scriptCall
		branch     string
		options    DeleteOptions
		wantResult DeleteResult
		wantErr    error
	}{
		"success": {
			branch: "feature/topic",
			calls: []scriptCall{
				{args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, stdout: "main"},
				{args: []string{"branch", "-d", "feature/topic"}, stdout: "Deleted branch feature/topic (was abc1234)."},
			},
			wantResult: DeleteResult{Stdout: "Deleted branch feature/topic (was abc1234)."},
		},
		"force": {
			branch:  "feature/topic",
			options: DeleteOptions{Force: true},
			calls: []scriptCall{
				{args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, stdout: "main"},
				{args: []string{"branch", "-D", "feature/topic"}, stdout: "Deleted branch feature/topic (was abc1234)."},
			},
			wantResult: DeleteResult{Stdout: "Deleted branch feature/topic (was abc1234)."},
		},
		"not-fully-merged": {
			branch: "feature/topic",
			calls: []scriptCall{
				{args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, stdout: "main"},
				{args: []string{"branch", "-d", "feature/topic"}, stderr: "error: The branch 'feature/topic' is not fully merged.", err: gitErr},
			},
			wantResult: DeleteResult{Stderr: "error: The branch 'feature/topic' is not fully merged."},
			wantErr:    ErrBranchNotFullyMerged,
		},
		"current-branch": {
			branch: "main",
			calls: []scriptCall{
				{args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, stdout: "main"},
			},
			wantErr: ErrDeleteCurrentBranch,
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runner := &scriptRunner{testingT: t, calls: tc.calls}
			client := NewClient(runner)

			result, err := client.DeleteBranch(ctx, tc.branch, tc.options)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Stdout != tc.wantResult.Stdout {
				t.Fatalf("unexpected stdout: got %q, want %q", result.Stdout, tc.wantResult.Stdout)
			}
			if result.Stderr != tc.wantResult.Stderr {
				t.Fatalf("unexpected stderr: got %q, want %q", result.Stderr, tc.wantResult.Stderr)
			}

			if !runner.Exhausted() {
				t.Fatalf("not all git calls were consumed: %d of %d", runner.index, len(runner.calls))
			}
		})
	}
}

func TestCLIRunCommands(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cli := NewCLI()

	out, err := cli.Run(ctx, "--version")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "git version") {
		t.Fatalf("unexpected git version output: %q", out)
	}

	_, stderr, err := cli.RunWithCombinedOutput(ctx, "--invalid-flag")
	if err == nil {
		t.Fatal("expected error for invalid flag")
	}
	if stderr == "" {
		t.Fatalf("expected stderr output for invalid flag")
	}
}
