package prtitle

import (
	"errors"
	"testing"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/bump"
)

func TestResolveValidTitles(t *testing.T) {
	t.Parallel()

	cases := []struct {
		title string
		want  bump.Bump
	}{
		{"chore(ci): update pipelines", bump.BumpPatch},
		{"feat(enhancement): extend to allow conventional commit to influence version", bump.BumpMinor},
		{"feat(auth): add SSO", bump.BumpMinor},
		{"fix!: remove deprecated API", bump.BumpMajor},
		{"fix(api)!: remove deprecated endpoint", bump.BumpMajor},
		{"perf(cache): reduce lookups", bump.BumpMinor},
		{"chore: bump deps", bump.BumpPatch},
		{"fix: correct typo", bump.BumpPatch},
		{"docs: update README", bump.BumpPatch},
		{"feat: add login", bump.BumpMinor},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()
			resolver := NewResolver()
			got, result, err := resolver.Resolve(tc.title)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if got != tc.want {
				t.Fatalf("bump = %s, want %s", got, tc.want)
			}
			if result.Title != tc.title {
				t.Fatalf("title = %q, want %q", result.Title, tc.title)
			}
		})
	}
}

func TestResolveScopedTitleScopeDoesNotAlterBump(t *testing.T) {
	t.Parallel()

	resolver := NewResolver()

	got, result, err := resolver.Resolve("chore(ci): update pipelines")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != bump.BumpPatch {
		t.Fatalf("bump = %s, want patch", got)
	}
	if result.Scope != "ci" {
		t.Fatalf("scope = %q, want ci", result.Scope)
	}
	if result.Type != "chore" {
		t.Fatalf("type = %q, want chore", result.Type)
	}
}

func TestResolveBreakingChangeFooter(t *testing.T) {
	t.Parallel()

	resolver := NewResolver()
	title := "feat: allow provided config object\n\nBREAKING CHANGE: config format changed"

	got, result, err := resolver.Resolve(title)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != bump.BumpMajor {
		t.Fatalf("bump = %s, want major", got)
	}
	if !result.BreakingChange {
		t.Fatalf("expected breaking change")
	}
}

func TestResolveInvalidTitles(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"WIP stuff",
		"feat",
		"feat:",
		"unknown: something",
	}

	for _, title := range cases {
		title := title
		t.Run(title, func(t *testing.T) {
			t.Parallel()
			resolver := NewResolver()
			_, _, err := resolver.Resolve(title)
			if err == nil {
				t.Fatalf("expected error for %q", title)
			}
			if !errors.Is(err, ErrInvalidConventionalCommit) {
				t.Fatalf("expected ErrInvalidConventionalCommit, got %v", err)
			}
		})
	}
}
