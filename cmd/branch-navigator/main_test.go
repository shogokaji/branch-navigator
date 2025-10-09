package main

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"

	"branch-navigator/internal/ui"
)

func TestParseArgsDefaultActionCheckout(t *testing.T) {
	t.Parallel()

	usage := &bytes.Buffer{}
	opts, err := parseArgs([]string{}, usage, usage)
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	if opts.action != actionCheckout {
		t.Fatalf("expected default action %q, got %q", actionCheckout, opts.action)
	}
	if opts.limit != 10 {
		t.Fatalf("expected default limit 10, got %d", opts.limit)
	}
}

func TestParseArgsSelectsMergeAction(t *testing.T) {
	t.Parallel()

	usage := &bytes.Buffer{}
	opts, err := parseArgs([]string{"-m"}, usage, usage)
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	if opts.action != actionMerge {
		t.Fatalf("expected action %q, got %q", actionMerge, opts.action)
	}
}

func TestParseArgsRejectsMultipleActions(t *testing.T) {
	t.Parallel()

	usage := &bytes.Buffer{}
	_, err := parseArgs([]string{"-c", "-m"}, usage, usage)
	if err == nil {
		t.Fatal("expected error when multiple actions are specified")
	}
	if !strings.Contains(err.Error(), "only one of -c, -m, or -d may be specified") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseArgsLimitAlias(t *testing.T) {
	t.Parallel()

	usage := &bytes.Buffer{}
	opts, err := parseArgs([]string{"--limit", "5"}, usage, usage)
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	if opts.limit != 5 {
		t.Fatalf("expected limit 5, got %d", opts.limit)
	}
}

func TestParseArgsRejectsInvalidLimit(t *testing.T) {
	t.Parallel()

	usage := &bytes.Buffer{}
	_, err := parseArgs([]string{"-n", "0"}, usage, usage)
	if err == nil {
		t.Fatal("expected error when limit is less than 1")
	}
	if !strings.Contains(err.Error(), "limit must be greater than 0") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseArgsHelp(t *testing.T) {
	t.Parallel()

	usage := &bytes.Buffer{}
	_, err := parseArgs([]string{"-h"}, usage, usage)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}

	output := usage.String()
	if !strings.Contains(output, "Usage: branch-navigator [-c|-m|-d] [-n N] [-h]") {
		t.Fatalf("usage output missing headline: %q", output)
	}
	if !strings.Contains(output, "  -c\tcheckout the selected branch (default)") {
		t.Fatalf("usage output missing -c description: %q", output)
	}
}

func TestActionDetailsFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		action action
		want   ui.ActionDetails
	}{
		{
			name:   "checkout",
			action: actionCheckout,
			want: ui.ActionDetails{
				Name:        "Checkout branch",
				Description: "Switch to the selected branch.",
				EnterLabel:  "checkout the selected branch",
			},
		},
		{
			name:   "merge",
			action: actionMerge,
			want: ui.ActionDetails{
				Name:        "Merge branch",
				Description: "Merge the selected branch into the current branch.",
				EnterLabel:  "merge the selected branch into the current branch",
			},
		},
		{
			name:   "delete",
			action: actionDelete,
			want: ui.ActionDetails{
				Name:        "Delete branch",
				Description: "Delete the selected local branch.",
				EnterLabel:  "delete the selected branch",
			},
		},
		{
			name:   "unknown",
			action: action("unknown"),
			want:   ui.ActionDetails{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := actionDetailsFor(tt.action)
			if got != tt.want {
				t.Fatalf("actionDetailsFor(%q) = %#v, want %#v", tt.action, got, tt.want)
			}
		})
	}
}
