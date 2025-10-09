package ui

import (
	"bytes"
	"strings"
	"testing"
)

const clearSequence = "\033[2J\033[H"

var checkoutAction = ActionDetails{
	Name:        "Checkout branch",
	Description: "Switch to the selected branch.",
	EnterLabel:  "checkout the selected branch",
}

func framesFromOutput(t *testing.T, output string) []string {
	t.Helper()
	frames := strings.Split(output, clearSequence)
	// remove possible leading empty chunk if output starts with clearSequence
	if len(frames) > 0 && frames[0] == "" {
		frames = frames[1:]
	}
	if len(frames) == 0 {
		t.Fatalf("no frames found in output: %q", output)
	}
	return frames
}

func TestSelectMovesWithJAndEnter(t *testing.T) {
	t.Parallel()

	input := bytes.NewBufferString("j\r")
	output := &bytes.Buffer{}

	branches := []Branch{
		{Name: "main", Current: true},
		{Name: "feature/awesome", Current: false},
	}

	ui := New(input, output, checkoutAction)
	result, err := ui.Select(branches)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	if result.Branch != "feature/awesome" {
		t.Fatalf("unexpected branch selected: got %q", result.Branch)
	}
	if result.Quit {
		t.Fatal("expected selection, but got quit signal")
	}
	if result.AlreadyOn {
		t.Fatal("expected selection, but result indicates already on branch")
	}

	frames := framesFromOutput(t, output.String())
	first := frames[0]
	if !strings.Contains(first, "Action: Checkout branch") {
		t.Fatalf("header missing action name. frame=%q", first)
	}
	if !strings.Contains(first, "Switch to the selected branch.") {
		t.Fatalf("header missing action description. frame=%q", first)
	}
	last := frames[len(frames)-1]
	if !strings.Contains(last, "> \033[32mfeature/awesome\033[0m") {
		t.Fatalf("highlighted selection missing or incorrect. frame=%q", last)
	}
	if !strings.Contains(last, "  main (current branch)") {
		t.Fatalf("current branch marker missing. frame=%q", last)
	}
	if !strings.Contains(output.String(), "j/k or ↑/↓ to move, Enter to checkout the selected branch, q to exit") {
		t.Fatalf("help message missing from output: %q", output.String())
	}
}

func TestSelectHandlesArrowKeys(t *testing.T) {
	t.Parallel()

	input := bytes.NewBufferString("\x1b[B\x1b[B\x1b[A\r")
	output := &bytes.Buffer{}

	branches := []Branch{
		{Name: "main", Current: true},
		{Name: "feature/alpha", Current: false},
		{Name: "feature/beta", Current: false},
	}

	ui := New(input, output, checkoutAction)
	result, err := ui.Select(branches)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	if result.Branch != "feature/alpha" {
		t.Fatalf("unexpected branch selected: got %q", result.Branch)
	}
}

func TestSelectQuit(t *testing.T) {
	t.Parallel()

	input := bytes.NewBufferString("q")
	output := &bytes.Buffer{}

	branches := []Branch{
		{Name: "main", Current: true},
		{Name: "feature/alpha", Current: false},
	}

	ui := New(input, output, checkoutAction)
	result, err := ui.Select(branches)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	if !result.Quit {
		t.Fatal("expected quit result")
	}
	if result.Branch != "" {
		t.Fatalf("expected no branch on quit, got %q", result.Branch)
	}
}

func TestSelectCurrentBranch(t *testing.T) {
	t.Parallel()

	input := bytes.NewBufferString("\r")
	output := &bytes.Buffer{}

	branches := []Branch{
		{Name: "main", Current: true},
		{Name: "feature/alpha", Current: false},
	}

	ui := New(input, output, checkoutAction)
	result, err := ui.Select(branches)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	if !result.AlreadyOn {
		t.Fatal("expected AlreadyOn flag when selecting current branch")
	}
	if result.Branch != "main" {
		t.Fatalf("expected current branch name, got %q", result.Branch)
	}
	if !strings.Contains(output.String(), "already on 'main'") {
		t.Fatalf("expected already on message in output: %q", output.String())
	}
}

func TestSelectHandlesControlKeys(t *testing.T) {
	t.Parallel()

	input := bytes.NewBuffer([]byte{0x03})
	output := &bytes.Buffer{}

	branches := []Branch{
		{Name: "main", Current: true},
		{Name: "feature/alpha", Current: false},
	}

	ui := New(input, output, checkoutAction)
	result, err := ui.Select(branches)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	if !result.Quit {
		t.Fatal("expected quit result for Ctrl+C")
	}
	if result.Branch != "" {
		t.Fatalf("expected no branch selection, got %q", result.Branch)
	}
	if result.AlreadyOn {
		t.Fatal("expected AlreadyOn to be false when quitting")
	}
}
