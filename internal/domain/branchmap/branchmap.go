package branchmap

import (
	"strings"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/bump"
)

// Mapping defines the branch prefixes that imply a semantic version bump intent.
type Mapping struct {
	MajorPrefixes []string
	MinorPrefixes []string
	PatchPrefixes []string
}

var defaultMapping = Mapping{
	MajorPrefixes: []string{"breaking/", "major/"},
	MinorPrefixes: []string{"feature/", "minor/"},
	PatchPrefixes: []string{"bugfix/", "fix/", "hotfix/", "chore/", "patch/"},
}

// Resolver maps branch names to bump intents, allowing future injection of custom mappings.
type Resolver struct {
	mapping Mapping
}

// NewResolver creates a Resolver using the provided mapping or the defaults when empty.
func NewResolver(mapping Mapping) Resolver {
	resolved := mapping
	if len(resolved.MajorPrefixes) == 0 && len(resolved.MinorPrefixes) == 0 && len(resolved.PatchPrefixes) == 0 {
		resolved = defaultMapping
	}
	return Resolver{mapping: sanitize(resolved)}
}

// DefaultMapping exposes the built-in mapping so callers can extend/modify it before injection.
func DefaultMapping() Mapping {
	return sanitize(defaultMapping)
}

// Resolve determines the bump intent for the provided branch.
// It returns the bump, the matched prefix (if any), and whether a prefix match occurred.
func (r Resolver) Resolve(branch string) (bump.Bump, string, bool) {
	if matched, ok := matchPrefix(branch, r.mapping.MajorPrefixes); ok {
		return bump.BumpMajor, matched, true
	}
	if matched, ok := matchPrefix(branch, r.mapping.MinorPrefixes); ok {
		return bump.BumpMinor, matched, true
	}
	if matched, ok := matchPrefix(branch, r.mapping.PatchPrefixes); ok {
		return bump.BumpPatch, matched, true
	}
	return bump.BumpPatch, "", false
}

func sanitize(m Mapping) Mapping {
	return Mapping{
		MajorPrefixes: trimAll(m.MajorPrefixes),
		MinorPrefixes: trimAll(m.MinorPrefixes),
		PatchPrefixes: trimAll(m.PatchPrefixes),
	}
}

func trimAll(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func matchPrefix(branch string, prefixes []string) (string, bool) {
	for _, prefix := range prefixes {
		if strings.HasPrefix(branch, prefix) {
			return prefix, true
		}
	}
	return "", false
}
