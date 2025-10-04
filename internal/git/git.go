package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes git commands.
type Runner interface {
	Run(ctx context.Context, args ...string) (string, error)
}

// CombinedRunner exposes stdout and stderr for a git invocation.
type CombinedRunner interface {
	RunWithCombinedOutput(ctx context.Context, args ...string) (string, string, error)
}

// CLI executes git commands using the local git binary.
type CLI struct{}

// NewCLI constructs a CLI Runner.
func NewCLI() *CLI {
	return &CLI{}
}

// Run invokes the git binary with the provided arguments.
func (c *CLI) Run(ctx context.Context, args ...string) (string, error) {
	stdout, _, err := c.RunWithCombinedOutput(ctx, args...)
	return stdout, err
}

// RunWithCombinedOutput invokes git and returns trimmed stdout and stderr strings.
func (c *CLI) RunWithCombinedOutput(ctx context.Context, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	outStr := strings.TrimSpace(stdout.String())
	errStr := strings.TrimSpace(stderr.String())
	if err != nil {
		if errStr != "" {
			return outStr, errStr, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, errStr)
		}
		return outStr, errStr, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	return outStr, errStr, nil
}

// Client provides higher-level git helpers used by the navigator.
type Client struct {
	runner Runner
}

// NewClient constructs a Client using the supplied Runner.
func NewClient(r Runner) *Client {
	return &Client{runner: r}
}

// NewDefaultClient constructs a Client backed by the CLI Runner.
func NewDefaultClient() *Client {
	return NewClient(NewCLI())
}

// FastForwardStrategy controls the fast-forward behavior of git merge.
type FastForwardStrategy int

const (
	// FastForwardDefault defers to git's default fast-forward behavior.
	FastForwardDefault FastForwardStrategy = iota
	// FastForwardOnly enforces fast-forward merges only.
	FastForwardOnly
	// FastForwardNoFF disables fast-forward merges.
	FastForwardNoFF
)

// MergeOptions configures merge behavior.
type MergeOptions struct {
	FastForward FastForwardStrategy
	ExtraArgs   []string
}

// MergeResult captures stdout and stderr emitted by git merge.
type MergeResult struct {
	Stdout string
	Stderr string
}

func (opts MergeOptions) args() []string {
	args := make([]string, 0, len(opts.ExtraArgs)+1)
	switch opts.FastForward {
	case FastForwardOnly:
		args = append(args, "--ff-only")
	case FastForwardNoFF:
		args = append(args, "--no-ff")
	}
	args = append(args, opts.ExtraArgs...)
	return args
}

// CurrentBranch returns the current branch name.
func (c *Client) CurrentBranch(ctx context.Context) (string, error) {
	if c == nil || c.runner == nil {
		return "", errors.New("git client is not configured")
	}
	out, err := c.runner.Run(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return out, nil
}

// ReflogBranchMoves returns branch names discovered in the HEAD reflog.
func (c *Client) ReflogBranchMoves(ctx context.Context) ([]string, error) {
	if c == nil || c.runner == nil {
		return nil, errors.New("git client is not configured")
	}
	out, err := c.runner.Run(ctx, "reflog", "--format=%gs")
	if err != nil {
		return nil, err
	}
	return parseReflogSubjects(out), nil
}

// BranchesByCommitDate returns local branches ordered by most recent commit date.
func (c *Client) BranchesByCommitDate(ctx context.Context) ([]string, error) {
	if c == nil || c.runner == nil {
		return nil, errors.New("git client is not configured")
	}
	out, err := c.runner.Run(ctx, "for-each-ref", "--format=%(refname:short)", "--sort=-committerdate", "refs/heads")
	if err != nil {
		return nil, err
	}
	return splitAndFilter(out), nil
}

// BranchExists reports whether the provided local branch exists.
func (c *Client) BranchExists(ctx context.Context, branch string) (bool, error) {
	if c == nil || c.runner == nil {
		return false, errors.New("git client is not configured")
	}
	if strings.TrimSpace(branch) == "" {
		return false, nil
	}
	_, err := c.runner.Run(ctx, "show-ref", "--verify", "--quiet", fmt.Sprintf("refs/heads/%s", branch))
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CheckoutBranch switches the working tree to the specified local branch.
func (c *Client) CheckoutBranch(ctx context.Context, branch string) (string, error) {
	if c == nil || c.runner == nil {
		return "", errors.New("git client is not configured")
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "", errors.New("branch name is required")
	}

	current, err := c.CurrentBranch(ctx)
	if err != nil {
		return "", err
	}
	if branch == current {
		return fmt.Sprintf("already on '%s'", branch), nil
	}

	out, err := c.runner.Run(ctx, "checkout", branch)
	if err != nil {
		return "", err
	}
	return out, nil
}

// MergeBranch merges the provided branch into the current branch.
func (c *Client) MergeBranch(ctx context.Context, branch string, opts MergeOptions) (MergeResult, error) {
	if c == nil || c.runner == nil {
		return MergeResult{}, errors.New("git client is not configured")
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return MergeResult{}, errors.New("branch name is required")
	}

	args := []string{"merge"}
	args = append(args, opts.args()...)
	args = append(args, branch)

	if combined, ok := c.runner.(CombinedRunner); ok {
		stdout, stderr, err := combined.RunWithCombinedOutput(ctx, args...)
		return MergeResult{Stdout: stdout, Stderr: stderr}, err
	}

	stdout, err := c.runner.Run(ctx, args...)
	return MergeResult{Stdout: stdout}, err
}

func parseReflogSubjects(output string) []string {
	lines := splitAndFilter(output)
	branches := make([]string, 0, len(lines))
	for _, line := range lines {
		if branch, ok := extractBranchFromSubject(line); ok {
			branches = append(branches, branch)
		}
	}
	return branches
}

func splitAndFilter(s string) []string {
	raw := strings.Split(strings.TrimSpace(s), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func extractBranchFromSubject(subject string) (string, bool) {
	if subject == "" {
		return "", false
	}

	const (
		prefixMoveFrom  = "checkout: moving from "
		prefixMoveTo    = "checkout: moving to "
		prefixSwitching = "checkout: switching to "
	)

	switch {
	case strings.HasPrefix(subject, prefixMoveFrom):
		rest := strings.TrimPrefix(subject, prefixMoveFrom)
		idx := strings.LastIndex(rest, " to ")
		if idx == -1 {
			return "", false
		}
		branch := strings.TrimSpace(rest[idx+4:])
		branch = strings.Trim(branch, "'\"")
		if branch == "" {
			return "", false
		}
		return branch, true
	case strings.HasPrefix(subject, prefixMoveTo):
		branch := strings.TrimSpace(strings.TrimPrefix(subject, prefixMoveTo))
		branch = strings.Trim(branch, "'\"")
		if branch == "" {
			return "", false
		}
		return branch, true
	case strings.HasPrefix(subject, prefixSwitching):
		branch := strings.TrimSpace(strings.TrimPrefix(subject, prefixSwitching))
		branch = strings.Trim(branch, "'\"")
		if branch == "" {
			return "", false
		}
		return branch, true
	default:
		return "", false
	}
}
