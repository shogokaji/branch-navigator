package git

import "testing"

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
