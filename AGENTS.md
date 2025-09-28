# Repository Guidelines

## Overview
- `branch-navigator` is an interactive Go CLI that lists recently visited Git branches and performs checkout (default), merge into the current branch, or local delete on the selected branch.
- The list shows up to `-n` branches (default 10), sourced from `git reflog` with a fallback to `git for-each-ref --sort=-committerdate`. Exclude the current branch, remote tracking refs, duplicates, and deleted branches.
- Target Go 1.22 or newer and ship a single binary (macOS first). Optionally read settings from `~/.config/branch-navigator/config.yaml`.

## CLI & UI (MVP)
- Invocation: `branch-navigator [-c|-m|-d] [-n N] [-h]`. Missing action flags default to checkout; `-h` prints help text.
- Actions: `-c` switches branches, `-m` merges the selected branch into the current branch, `-d` deletes the selected local branch safely. Unmerged deletes should confirm or exit non-zero, and git errors propagate via non-zero exits.
- Interactive controls: `j/k` or the Down/Up arrows move, `Enter` confirms, `q` exits. Highlight the current row with `>` and show `(current branch)` when applicable. Selecting the current branch exits immediately with `already on '<branch>'`.

## Architecture & Testing
- Entry point: `cmd/branch-navigator/main.go`. Keep shared logic under `internal/` (`internal/git` for git execution and parsing, `internal/navigator` for history and selection, `internal/ui` for terminal I/O). Place configuration adapters under `internal/platform/` when needed.
- Implement the CLI with the standard `flag` package and run git via `os/exec`. Consider `spf13/cobra` and `goreleaser` in later iterations.
- Write table-driven tests alongside the code, interface the git layer for mocking, and keep package coverage at or above 80%. Store fixtures under `testdata/`.

## Development Workflow
- Run `go mod init branch-navigator` once, then rely on `go build ./...`, `go test ./...`, `go run ./cmd/branch-navigator`, `go fmt ./...`, and `goimports ./...` during development.
- Follow idiomatic Go naming: mixedCaps for exported identifiers, lowerCamelCase for unexported ones, and ALL_CAPS only for constants driven by the environment. Keep packages small and lower-case.
- Practice TDD (Red→Green→Refactor) with focused commits. Name branches `codex/{issue-number}-{descriptive-branch-name}` using lower-case words and hyphens.
- Write commit messages in Japanese with prefixes: `<prefix>: <summary in Japanese>`. Pull requests describe intent, list changes, note test runs (for example `go test ./...`), and wait for CI before merging.
