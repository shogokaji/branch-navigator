package main

import "testing"

func TestResolveAction(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		checkout     bool
		merge        bool
		deleteBranch bool
		expected     action
		expectErr    bool
	}{
		"defaults to checkout": {
			expected: actionCheckout,
		},
		"select checkout": {
			checkout: true,
			expected: actionCheckout,
		},
		"select merge": {
			merge:    true,
			expected: actionMerge,
		},
		"select delete": {
			deleteBranch: true,
			expected:     actionDelete,
		},
		"error on multiple": {
			checkout:  true,
			merge:     true,
			expectErr: true,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			act, err := resolveAction(tc.checkout, tc.merge, tc.deleteBranch)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveAction returned error: %v", err)
			}

			if act != tc.expected {
				t.Fatalf("expected action %q, got %q", tc.expected, act)
			}
		})
	}
}
