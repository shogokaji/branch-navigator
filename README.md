# branch-navigator
![demo](https://github.com/user-attachments/assets/6d7edaf5-8351-4ebd-844c-0201a8cb4091)

branch-navigator is an interactive Git helper written in Go 1.22+ that keeps your terminal workflow focused on the branches you touched most recently. It lists the current branch, nearby history, and lets you act on a selection without leaving the keyboard.

## Features
- Shows the current branch plus a deduplicated list of recent local branches pulled from `git reflog`, with a commit-date fallback when the reflog runs dry.
- Keyboard-first navigation: `j`/`k` or arrow keys move, `Enter` triggers the action, `q`/`Ctrl+C` exits. The highlighted row is prefixed with `>` and rendered in green, and `(current branch)` marks the branch you are already on.
- One binary, three actions: checkout (default), merge, or safe delete. Unmerged deletes prompt before retrying with force, and git exit codes propagate untouched.
- Thin wrapper around your local git: no daemons, no shell hooks, just standard output so you can read git's messages directly.

## Installation

### Prebuilt binaries
Tagged releases (`v*`) are packaged by GoReleaser for macOS, Linux, and Windows. To install:
1. Open the repository's **Releases** page.
2. Download the archive that matches your OS and architecture.
3. Extract the `branch-navigator` binary and place it on your `PATH` (for example `~/bin`).

Each archive contains a single statically linked binary; no extra runtime is required besides git.

### Build from source
```sh
go build ./cmd/branch-navigator
./branch-navigator -h

# or install into $(go env GOPATH)/bin
go install ./cmd/branch-navigator
```

Go 1.22 or newer is required to build the project.

## Usage
Run the tool inside any Git repository.

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

Action flags choose what happens when you press `Enter`:
- `-c` (default) checks out the highlighted branch. Picking the current branch prints `already on '<branch>'` and exits successfully.
- `-m` merges the highlighted branch into the current branch. Git's stdout/stderr and exit code are passed through so you can resolve conflicts immediately.
- `-d` deletes the highlighted local branch. If the branch is not fully merged, you'll be prompted before retrying with `git branch -D`. Attempts to delete the current branch are rejected.
- `-n` / `--limit` controls how many branches are listed (default `10`).
- `-h` prints help and exits.

The UI runs in raw mode when connected to a TTY so single keystrokes take effect instantly. Arrow keys, `j`, and `k` move the selection; `q`, `Ctrl+C`, `Ctrl+D`, `Ctrl+Z`, or EOF exit without changes.

### How branches are chosen
1. Read the HEAD reflog (`git reflog --format=%gs`) to collect branch switch entries.
2. Filter out empty lines, duplicates, the current branch, deleted branches, and anything that no longer exists locally.
3. When the reflog does not fill the requested limit, fall back to `git for-each-ref --sort=-committerdate refs/heads` and continue filtering.

## Development
- Install Go 1.22+ and ensure `git` is available on your `PATH`.
- Format with `go fmt ./...` and `goimports ./...`.
- Build with `go build ./...`; test with `go test -cover ./...` (target ≥80% coverage).
- Use `go run ./cmd/branch-navigator` inside a Git repository to try the interactive flow.

## License
MIT. See `LICENSE` for the full text.
