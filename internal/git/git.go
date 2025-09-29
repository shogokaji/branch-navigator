package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

type CommandRunner struct{}

func (CommandRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	return cmd.CombinedOutput()
}

type Client struct {
	runner Runner
}

func NewClient(r Runner) *Client {
	if r == nil {
		r = CommandRunner{}
	}
	return &Client{runner: r}
}

func (c *Client) CurrentBranch(ctx context.Context) (string, error) {
	out, err := c.run(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(out)
	if branch == "" {
		return "", errors.New("git: empty branch name")
	}
	return branch, nil
}

func (c *Client) ReflogBranchVisits(ctx context.Context) ([]string, error) {
	out, err := c.run(ctx, "reflog", "--format=%gs")
	if err != nil {
		return nil, err
	}
	subjects := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	var visits []string
	for _, subject := range subjects {
		if subject == "" {
			continue
		}
		if branch := extractBranchFromReflogSubject(subject); branch != "" {
			visits = append(visits, branch)
		}
	}
	return visits, nil
}

func (c *Client) LocalBranches(ctx context.Context) ([]string, error) {
	out, err := c.run(ctx, "for-each-ref", "--sort=-committerdate", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	branches := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		branches = append(branches, line)
	}
	return branches, nil
}

func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	out, err := c.runner.Run(ctx, args...)
	output := string(out)
	if err != nil {
		trimmed := strings.TrimSpace(output)
		if trimmed != "" {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, trimmed)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

func extractBranchFromReflogSubject(subject string) string {
	trimmed := strings.TrimSpace(subject)
	if trimmed == "" {
		return ""
	}

	lower := strings.ToLower(trimmed)

	switch {
	case strings.HasPrefix(lower, "checkout:"):
		return branchFromSuffix(trimmed)
	case strings.HasPrefix(lower, "switch branch:"):
		return branchFromSuffix(trimmed)
	case strings.HasPrefix(lower, "switch to branch"):
		return branchFromQuoted(trimmed)
	case strings.HasPrefix(lower, "reset: moving to"):
		return branchFromSuffix(trimmed)
	}

	return ""
}

func branchFromSuffix(line string) string {
	idx := strings.LastIndex(strings.ToLower(line), " to ")
	if idx == -1 {
		return ""
	}

	candidate := strings.TrimSpace(line[idx+4:])
	candidate = strings.Trim(candidate, "'\"")

	if candidate == "" || strings.HasPrefix(candidate, "HEAD") {
		return ""
	}
	if space := strings.IndexAny(candidate, " \t"); space != -1 {
		candidate = candidate[:space]
	}
	return candidate
}

func branchFromQuoted(line string) string {
	start := strings.Index(line, "'")
	if start == -1 {
		return ""
	}
	rest := line[start+1:]
	end := strings.Index(rest, "'")
	if end == -1 {
		return ""
	}
	candidate := rest[:end]
	candidate = strings.TrimSpace(candidate)
	if candidate == "" || strings.HasPrefix(candidate, "HEAD") {
		return ""
	}
	return candidate
}
