package labels

import (
	"testing"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/bump"
)

func TestNewResolverDefaults(t *testing.T) {
	t.Parallel()

	r := NewResolver(Config{})
	resolved := r.Labels()

	if resolved[bump.BumpMajor] != "semver-major" {
		t.Fatalf("expected default major label, got %s", resolved[bump.BumpMajor])
	}
	if resolved[bump.BumpMinor] != "semver-minor" {
		t.Fatalf("expected default minor label, got %s", resolved[bump.BumpMinor])
	}
	if resolved[bump.BumpPatch] != "semver-patch" {
		t.Fatalf("expected default patch label, got %s", resolved[bump.BumpPatch])
	}
}

func TestDecide(t *testing.T) {
	t.Parallel()

	r := NewResolver(Config{Prefix: "release-"})

	tests := []struct {
		name      string
		existing  []string
		bump      bump.Bump
		expected  Decision
		expectedL string
	}{
		{
			name:      "noop when expected present",
			existing:  []string{"release-major"},
			bump:      bump.BumpMajor,
			expected:  DecisionNoop,
			expectedL: "release-major",
		},
		{
			name:      "conflict when other semver labels exist",
			existing:  []string{"release-minor"},
			bump:      bump.BumpPatch,
			expected:  DecisionConflict,
			expectedL: "release-patch",
		},
		{
			name:      "add when none present",
			existing:  []string{"needs-review"},
			bump:      bump.BumpMinor,
			expected:  DecisionAddExpected,
			expectedL: "release-minor",
		},
	}

	for _, testCase := range tests {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := r.Decide(tc.existing, tc.bump)
			if result.Decision != tc.expected {
				t.Fatalf("decision: expected %v got %v", tc.expected, result.Decision)
			}
			if result.ExpectedLabel != tc.expectedL {
				t.Fatalf("expected label: expected %s got %s", tc.expectedL, result.ExpectedLabel)
			}
		})
	}
}

func TestBumpForLabel(t *testing.T) {
	t.Parallel()

	r := NewResolver(Config{Prefix: "rel-"})

	if b, ok := r.BumpForLabel("REL-MAJOR"); !ok || b != bump.BumpMajor {
		t.Fatalf("expected major bump, got %v, ok=%v", b, ok)
	}

	if _, ok := r.BumpForLabel("unknown"); ok {
		t.Fatalf("expected no bump for unknown label")
	}
}
