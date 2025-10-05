package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

const clearScreen = "\033[2J\033[H"
const highlightColor = "\033[32m"
const resetColor = "\033[0m"
const lineBreak = "\r\n"

// Branch represents a branch candidate with metadata required by the UI.
type Branch struct {
	Name    string
	Current bool
}

// Result captures the outcome of the branch selection loop.
type Result struct {
	Branch    string
	Quit      bool
	AlreadyOn bool
}

// UI drives the interactive terminal selection flow.
type UI struct {
	in  io.Reader
	out io.Writer
}

// New constructs a UI bound to the given input and output streams.
func New(input io.Reader, output io.Writer) *UI {
	return &UI{in: input, out: output}
}

// Select renders the branch list and processes key events until completion.
func (u *UI) Select(branches []Branch) (Result, error) {
	if u == nil {
		return Result{}, fmt.Errorf("ui is nil")
	}
	if u.in == nil || u.out == nil {
		return Result{}, fmt.Errorf("ui input and output must be configured")
	}

	restore, err := u.enterRawMode()
	if err != nil {
		return Result{}, err
	}
	if restore != nil {
		defer restore()
	}

	reader := bufio.NewReader(u.in)
	index := 0
	maxIndex := len(branches) - 1
	if err := u.render(branches, index); err != nil {
		return Result{}, err
	}

	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				return Result{Quit: true}, nil
			}
			return Result{}, err
		}

		switch b {
		case 0x03, 0x04, 0x1a: // Ctrl+C, Ctrl+D, Ctrl+Z
			return Result{Quit: true}, nil
		case 'j':
			if index < maxIndex {
				index++
				if err := u.render(branches, index); err != nil {
					return Result{}, err
				}
			}
		case 'k':
			if index > 0 {
				index--
				if err := u.render(branches, index); err != nil {
					return Result{}, err
				}
			}
		case 'q', 'Q':
			return Result{Quit: true}, nil
		case '\r', '\n':
			if len(branches) == 0 {
				return Result{Quit: true}, nil
			}
			selected := branches[index]
			if selected.Current {
				if _, err := fmt.Fprintf(u.out, "already on '%s'%s", selected.Name, lineBreak); err != nil {
					return Result{}, err
				}
				return Result{Branch: selected.Name, AlreadyOn: true}, nil
			}
			return Result{Branch: selected.Name}, nil
		case 0x1b: // escape sequence
			if err := u.handleEscape(reader, &index, maxIndex, branches); err != nil {
				return Result{}, err
			}
		default:
			// ignore other keys
		}
	}
}

func (u *UI) handleEscape(reader *bufio.Reader, index *int, maxIndex int, branches []Branch) error {
	next, err := reader.ReadByte()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	if next != '[' {
		return nil
	}

	dir, err := reader.ReadByte()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}

	updated := false
	switch dir {
	case 'A':
		if *index > 0 {
			*index = *index - 1
			updated = true
		}
	case 'B':
		if maxIndex >= 0 && *index < maxIndex {
			*index = *index + 1
			updated = true
		}
	default:
		return nil
	}

	if !updated {
		return nil
	}
	return u.render(branches, *index)
}

func (u *UI) render(branches []Branch, selected int) error {
	if _, err := fmt.Fprint(u.out, clearScreen); err != nil {
		return err
	}
	if _, err := fmt.Fprint(u.out, "Select a branch:"+lineBreak); err != nil {
		return err
	}
	for i, branch := range branches {
		line := branch.Name
		if branch.Current {
			line += " (current branch)"
		}
		if i == selected {
			if _, err := fmt.Fprintf(u.out, "> %s%s%s%s", highlightColor, line, resetColor, lineBreak); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(u.out, "  %s%s", line, lineBreak); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprint(u.out, lineBreak); err != nil {
		return err
	}
	if _, err := fmt.Fprint(u.out, "j/k or ↑/↓ to move, Enter to select, q to exit"+lineBreak); err != nil {
		return err
	}
	return nil
}

func (u *UI) enterRawMode() (func(), error) {
	file, ok := u.in.(*os.File)
	if !ok {
		return nil, nil
	}

	fd := int(file.Fd())
	if !term.IsTerminal(fd) {
		return nil, nil
	}

	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("failed to configure terminal for interactive input: %w", err)
	}

	return func() {
		_ = term.Restore(fd, state)
	}, nil
}
