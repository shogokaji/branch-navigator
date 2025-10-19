package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

const clearScreen = "\033[2J\033[H"
const lineBreak = "\r\n"
const resetColor = "\033[0m"

// Theme captures the ANSI sequences applied to various UI elements.
type Theme struct {
	ActionLabel       string
	ActionDescription string
	Branch            string
	Selected          string
	SelectedBadge     string
	Badge             string
	Help              string
}

// ThemeNord implements the Nord-inspired palette.
var ThemeNord = Theme{
	ActionLabel:       "\033[1;38;5;116m",
	ActionDescription: "\033[38;5;255m",
	Branch:            "\033[38;5;249m",
	Selected:          "\033[1;38;5;255;48;5;67m",
	SelectedBadge:     "\033[1;38;5;108;48;5;67m",
	Badge:             "\033[1;38;5;108m",
	Help:              "\033[38;5;244m",
}

// ThemeCatppuccin implements the Catppuccin Mocha palette.
var ThemeCatppuccin = Theme{
	ActionLabel:       "\033[1;38;5;111m",
	ActionDescription: "\033[38;5;189m",
	Branch:            "\033[38;5;188m",
	Selected:          "\033[1;38;5;234;48;5;111m",
	SelectedBadge:     "\033[1;38;5;151;48;5;111m",
	Badge:             "\033[1;38;5;151m",
	Help:              "\033[38;5;246m",
}

// ThemeClassic provides an ANSI-friendly palette with broad terminal support.
var ThemeClassic = Theme{
	ActionLabel:       "\033[1;36m",
	ActionDescription: "\033[37m",
	Branch:            "\033[37m",
	Selected:          "\033[1;97;44m",
	SelectedBadge:     "\033[1;32;44m",
	Badge:             "\033[1;32m",
	Help:              "\033[90m",
}

// ThemeSolarized provides a Solarized Dark-inspired palette.
var ThemeSolarized = Theme{
	ActionLabel:       "\033[1;38;5;33m",
	ActionDescription: "\033[38;5;230m",
	Branch:            "\033[38;5;244m",
	Selected:          "\033[1;38;5;230;48;5;23m",
	SelectedBadge:     "\033[1;38;5;109;48;5;23m",
	Badge:             "\033[1;38;5;109m",
	Help:              "\033[38;5;243m",
}

// ThemeGruvbox provides a Gruvbox-inspired warm palette.
var ThemeGruvbox = Theme{
	ActionLabel:       "\033[1;38;5;208m",
	ActionDescription: "\033[38;5;223m",
	Branch:            "\033[38;5;250m",
	Selected:          "\033[1;38;5;235;48;5;172m",
	SelectedBadge:     "\033[1;38;5;114;48;5;172m",
	Badge:             "\033[1;38;5;114m",
	Help:              "\033[38;5;244m",
}

// ThemeOneDark provides a One Dark-inspired palette.
var ThemeOneDark = Theme{
	ActionLabel:       "\033[1;38;5;75m",
	ActionDescription: "\033[38;5;253m",
	Branch:            "\033[38;5;250m",
	Selected:          "\033[1;38;5;233;48;5;68m",
	SelectedBadge:     "\033[1;38;5;114;48;5;68m",
	Badge:             "\033[1;38;5;114m",
	Help:              "\033[38;5;246m",
}

// DefaultTheme holds the palette used when no explicit selection is provided.
var DefaultTheme = ThemeCatppuccin

var themeNames = []string{"catppuccin", "nord", "classic", "solarized", "gruvbox", "onedark"}

// AvailableThemeNames returns the canonical list of supported themes.
func AvailableThemeNames() []string {
	names := make([]string, len(themeNames))
	copy(names, themeNames)
	return names
}

// ThemeByName resolves a theme by its human-readable name.
func ThemeByName(name string) (Theme, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "":
		return DefaultTheme, true
	case "nord":
		return ThemeNord, true
	case "catppuccin", "catppuccin-mocha", "mocha":
		return ThemeCatppuccin, true
	case "classic", "ansi":
		return ThemeClassic, true
	case "solarized", "solarized-dark":
		return ThemeSolarized, true
	case "gruvbox":
		return ThemeGruvbox, true
	case "onedark", "one-dark":
		return ThemeOneDark, true
	default:
		return Theme{}, false
	}
}

// ActionDetails captures the labels describing the currently configured operation.
type ActionDetails struct {
	Name        string
	Description string
	EnterLabel  string
}

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
	in     io.Reader
	out    io.Writer
	action ActionDetails
	theme  Theme
}

// New constructs a UI bound to the given input and output streams.
func New(input io.Reader, output io.Writer, action ActionDetails) *UI {
	return NewWithTheme(input, output, action, DefaultTheme)
}

// NewWithTheme constructs a UI configured with the provided theme.
func NewWithTheme(input io.Reader, output io.Writer, action ActionDetails, theme Theme) *UI {
	if theme == (Theme{}) {
		theme = DefaultTheme
	}
	return &UI{in: input, out: output, action: action, theme: theme}
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

	theme := u.theme
	if theme == (Theme{}) {
		theme = DefaultTheme
	}

	headerPrinted := false
	if name := strings.TrimSpace(u.action.Name); name != "" {
		if _, err := fmt.Fprintf(u.out, "%sAction: %s%s%s", theme.ActionLabel, name, resetColor, lineBreak); err != nil {
			return err
		}
		headerPrinted = true
	}
	if description := strings.TrimSpace(u.action.Description); description != "" {
		if _, err := fmt.Fprintf(u.out, "%s%s%s%s", theme.ActionDescription, description, resetColor, lineBreak); err != nil {
			return err
		}
		headerPrinted = true
	}
	if headerPrinted {
		if _, err := fmt.Fprint(u.out, lineBreak); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(u.out, "%sSelect a branch:%s%s", theme.Branch, resetColor, lineBreak); err != nil {
		return err
	}
	for i, branch := range branches {
		if i == selected {
			if branch.Current {
				if _, err := fmt.Fprintf(u.out, "%s> %s %s(current branch)%s%s", theme.Selected, branch.Name, theme.SelectedBadge, resetColor, lineBreak); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(u.out, "%s> %s%s%s", theme.Selected, branch.Name, resetColor, lineBreak); err != nil {
				return err
			}
			continue
		}

		if branch.Current {
			if _, err := fmt.Fprintf(u.out, "  %s%s%s %s(current branch)%s%s", theme.Branch, branch.Name, resetColor, theme.Badge, resetColor, lineBreak); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(u.out, "  %s%s%s%s", theme.Branch, branch.Name, resetColor, lineBreak); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(u.out, lineBreak); err != nil {
		return err
	}
	enterLabel := strings.TrimSpace(u.action.EnterLabel)
	if enterLabel == "" {
		enterLabel = "select"
	}
	if _, err := fmt.Fprintf(u.out, "%sj/k or ↑/↓ to move, Enter to %s, q to exit%s%s", theme.Help, enterLabel, resetColor, lineBreak); err != nil {
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
