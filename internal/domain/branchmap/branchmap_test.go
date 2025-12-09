package branchmap

import (
	"testing"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/bump"
)

func TestResolverResolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		mapping       Mapping
		branch        string
		expectedBump  bump.Bump
		expectedMatch string
		matched       bool
	}{
		{
			name:          "matches major prefix",
			branch:        "breaking/new-api",
			expectedBump:  bump.BumpMajor,
			expectedMatch: "breaking/",
			matched:       true,
		},
		{
			name:          "matches minor when no major",
			branch:        "feature/new-ui",
			expectedBump:  bump.BumpMinor,
			expectedMatch: "feature/",
			matched:       true,
		},
		{
			name:          "matches patch",
			branch:        "bugfix/issue",
			expectedBump:  bump.BumpPatch,
			expectedMatch: "bugfix/",
			matched:       true,
		},
		{
			name:         "defaults to patch when no match",
			branch:       "docs/readme",
			expectedBump: bump.BumpPatch,
			matched:      false,
		},
		{
			name: "custom mapping used",
			mapping: Mapping{
				MajorPrefixes: []string{"release/"},
			},
			branch:        "release/v2",
			expectedBump:  bump.BumpMajor,
			expectedMatch: "release/",
			matched:       true,
		},
		{
			name: "major preferred over minor when both match",
			mapping: Mapping{
				MajorPrefixes: []string{"rel/"},
				MinorPrefixes: []string{"rel/"},
			},
			branch:        "rel/feat",
			expectedBump:  bump.BumpMajor,
			expectedMatch: "rel/",
			matched:       true,
		},
	}

	for _, testCase := range tests {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resolver := NewResolver(tc.mapping)
			gotBump, gotPrefix, gotMatched := resolver.Resolve(tc.branch)

			if gotBump != tc.expectedBump {
				t.Fatalf("bump: expected %s got %s", tc.expectedBump, gotBump)
			}
			if gotPrefix != tc.expectedMatch {
				t.Fatalf("prefix: expected %q got %q", tc.expectedMatch, gotPrefix)
			}
			if gotMatched != tc.matched {
				t.Fatalf("matched: expected %v got %v", tc.matched, gotMatched)
			}
		})
	}
}

func TestDefaultMappingCopy(t *testing.T) {
	t.Parallel()

	m := DefaultMapping()
	if len(m.MajorPrefixes) == 0 {
		t.Fatal("expected default mapping to include major prefixes")
	}

	m.MajorPrefixes = nil
	other := DefaultMapping()
	if len(other.MajorPrefixes) == 0 {
		t.Fatal("expected second default mapping to remain unchanged")
	}
}
