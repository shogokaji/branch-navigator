# branch-navigator
![demo](https://github.com/user-attachments/assets/6d7edaf5-8351-4ebd-844c-0201a8cb4091)

branch-navigator is an interactive Git helper written in Go 1.22+ that keeps your terminal workflow focused on the branches you touched most recently. It lists recent local branches, lets you act on them with a couple of keystrokes, and exits cleanly when you are already where you meant to be.

## Highlights
- Surface the current branch plus a deduplicated list of recent local branches sourced from `git reflog`, with a commit-date fallback when the reflog is sparse.
- Keyboard-first navigation: `j`/`k` or the arrow keys move the cursor, `Enter` confirms, `q` exits. The selected row is prefixed with `>` and rendered in green, and `(current branch)` is shown when applicable.
- Checkout, merge, and safe delete actions. Deletes prompt when a branch is not fully merged and refuse to touch the branch you are on.
- Git output is streamed directly so you can resolve merges and inspect errors without leaving the tool.
- Works anywhere you have Git installed—no extra services, databases, or shell integrations required.

## Installation

### Pre-built releases
Once a tagged release (matching `v*`) is published, GitHub Actions runs GoReleaser and uploads archives for macOS, Linux, and Windows. To install:
1. Visit the repository's **Releases** page.
2. Download the archive that matches your OS and architecture.
3. Extract the single `branch-navigator` binary and move it onto your `PATH` (for example `~/bin`).

Each archive contains a standalone binary; no runtime dependencies are required.

### Build from source
If you have the repository locally, you can build or install the binary yourself:

```sh
go build ./cmd/branch-navigator
./branch-navigator -h

# or install into $(go env GOPATH)/bin
go install ./cmd/branch-navigator
```

branch-navigator targets Go 1.22 or newer. Older toolchains are not supported.

## Usage

Run the CLI inside any Git repository. The help output is:

```
Usage: branch-navigator [-c|-m|-d] [-n N] [-h]

Options:
  -c	checkout the selected branch (default)
  -m	merge the selected branch into the current branch
  -d	delete the selected local branch
  -n	maximum number of branches to list (default 10)
      --limit N	alias for -n
  -h	show this help message
```

### Actions
- `-c` (default): check out the highlighted branch. Selecting the current branch immediately prints `already on '<branch>'` and exits with success.
- `-m`: merge the highlighted branch into the current branch. The tool streams `git merge` output to stdout/stderr so you see conflicts and progress in real time. Git's exit status is propagated.
- `-d`: delete the highlighted local branch. When the branch is not fully merged, you are prompted before retrying with `git branch -D`. Attempts to delete the current branch are blocked.
- `-n`, `--limit`: cap how many candidates appear (default `10`).
- `-h`: print the help text and exit.

### Interactive controls
- `j` / `k` or Down / Up arrows: move the selection cursor.
- `Enter`: run the selected action on the highlighted branch.
- `q`: exit without taking any action.
- `Ctrl+C`, `Ctrl+D`, `Ctrl+Z`, or EOF: abort immediately.

The UI runs in raw mode when the input is a TTY. If you select the current branch, the tool short-circuits after printing `already on '<branch>'`.

## How branches are chosen
1. Query `git reflog --format=%gs` to replay the HEAD reflog and extract branch moves.
2. Filter out empty entries, the current branch, deleted branches, and duplicates.
3. Validate each candidate with `git show-ref --verify` before adding it to the menu.
4. If the reflog does not supply enough unique branches, fall back to `git for-each-ref --sort=-committerdate refs/heads` and continue filtering until the requested limit is satisfied.

This keeps the list focused on the branches that are still available locally.

## Project layout

```
cmd/branch-navigator   Main entry point and CLI wiring
internal/git           Git command execution and parsing helpers
internal/navigator     Recent-branch discovery and filtering
internal/ui            Terminal input/output and interactive loop
```

## Development
- Install Go 1.22 or newer.
- Format code with `go fmt ./...` and `goimports ./...` before sending patches.
- Build with `go build ./...` and run `go test -cover ./...` to keep package coverage at or above 80%.
- Use `go run ./cmd/branch-navigator` inside a Git repository to test the interactive flow as you iterate.

## Continuous integration and releases
- `.github/workflows/ci.yml` runs gofmt verification, `go build`, and `go test -cover ./...` on pushes and pull requests.
- `.github/workflows/release.yml` triggers on `v*` tags or manual dispatch, runs `go test ./...`, and invokes GoReleaser. The resulting archives are attached to the GitHub release automatically.
- `.goreleaser.yaml` is configured to produce macOS, Linux, and Windows binaries with CGO disabled for static builds.

## License

branch-navigator is released under the MIT License. See `LICENSE` for details.
