package labels

import (
	"strings"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/bump"
)

// Resolver centralizes derivation of semver labels and conflict decisions.
type Resolver struct {
	labels map[bump.Bump]string
	lower  map[string]bump.Bump
}

// Config controls how labels are constructed.
type Config struct {
	Prefix     string
	MajorLabel string
	MinorLabel string
	PatchLabel string
}

// NewResolver builds a Resolver using the provided config. Prefix defaults to "semver-".
func NewResolver(cfg Config) Resolver {
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "semver-"
	}

	labels := map[bump.Bump]string{
		bump.BumpMajor: chooseLabel(cfg.MajorLabel, prefix+"major"),
		bump.BumpMinor: chooseLabel(cfg.MinorLabel, prefix+"minor"),
		bump.BumpPatch: chooseLabel(cfg.PatchLabel, prefix+"patch"),
	}

	lower := make(map[string]bump.Bump, len(labels))
	for b, lbl := range labels {
		lower[strings.ToLower(lbl)] = b
	}

	return Resolver{labels: labels, lower: lower}
}

// Labels exposes the resolved label names.
func (r Resolver) Labels() map[bump.Bump]string {
	cpy := make(map[bump.Bump]string, len(r.labels))
	for k, v := range r.labels {
		cpy[k] = v
	}
	return cpy
}

// LabelFor returns the configured label for the bump. Falls back to patch label when unknown.
func (r Resolver) LabelFor(b bump.Bump) string {
	if lbl, ok := r.labels[b]; ok {
		return lbl
	}
	return r.labels[bump.BumpPatch]
}

// Decision represents how to handle semver labels on a PR.
type Decision int

const (
	DecisionNoop Decision = iota
	DecisionAddExpected
	DecisionConflict
)

// DecisionResult encapsulates the outcome.
type DecisionResult struct {
	Decision      Decision
	ExpectedLabel string
	Existing      []string
}

// Decide determines whether to add the expected label, leave as-is, or warn about conflicts.
func (r Resolver) Decide(existing []string, desired bump.Bump) DecisionResult {
	expected := r.LabelFor(desired)
	semverLabels := r.semverLabels(existing)

	for _, lbl := range semverLabels {
		if strings.EqualFold(lbl, expected) {
			return DecisionResult{Decision: DecisionNoop, ExpectedLabel: expected, Existing: semverLabels}
		}
	}

	if len(semverLabels) > 0 {
		return DecisionResult{Decision: DecisionConflict, ExpectedLabel: expected, Existing: semverLabels}
	}

	return DecisionResult{Decision: DecisionAddExpected, ExpectedLabel: expected}
}

func (r Resolver) semverLabels(existing []string) []string {
	results := make([]string, 0, len(existing))
	for _, lbl := range existing {
		if lbl == "" {
			continue
		}
		if _, ok := r.lower[strings.ToLower(lbl)]; ok {
			results = append(results, lbl)
		}
	}
	return results
}

// BumpForLabel reports the bump intent associated with the provided label, if any.
func (r Resolver) BumpForLabel(label string) (bump.Bump, bool) {
	if label == "" {
		return "", false
	}
	b, ok := r.lower[strings.ToLower(label)]
	return b, ok
}

func chooseLabel(override string, fallback string) string {
	trimmed := strings.TrimSpace(override)
	if trimmed != "" {
		return trimmed
	}
	return fallback
}
