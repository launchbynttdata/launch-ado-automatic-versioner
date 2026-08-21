package prtitle

import (
	"errors"
	"fmt"
	"strings"

	"github.com/leodido/go-conventionalcommits"
	"github.com/leodido/go-conventionalcommits/parser"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/bump"
)

// ErrInvalidConventionalCommit indicates the PR title does not conform to conventional commits.
var ErrInvalidConventionalCommit = errors.New("invalid conventional commit")

// Result captures parsed conventional commit metadata used for logging.
type Result struct {
	Title          string
	Type           string
	Scope          string
	Description    string
	BreakingChange bool
}

// Resolver parses PR titles as conventional commits and maps them to bump intent.
type Resolver struct{}

// NewResolver constructs a Resolver instance.
func NewResolver() Resolver {
	return Resolver{}
}

// Resolve parses title and returns the semver bump intent.
func (r Resolver) Resolve(title string) (bump.Bump, Result, error) {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return bump.BumpPatch, Result{}, fmt.Errorf("%w: title is empty", ErrInvalidConventionalCommit)
	}

	machine := parser.NewMachine(parser.WithTypes(conventionalcommits.TypesConventional))
	parsed, err := machine.Parse([]byte(trimmed))
	if err != nil {
		return bump.BumpPatch, Result{}, fmt.Errorf("%w: %v", ErrInvalidConventionalCommit, err)
	}
	if parsed == nil || !parsed.Ok() {
		return bump.BumpPatch, Result{}, fmt.Errorf("%w: parse returned invalid commit", ErrInvalidConventionalCommit)
	}

	commit, ok := parsed.(*conventionalcommits.ConventionalCommit)
	if !ok {
		return bump.BumpPatch, Result{}, fmt.Errorf("%w: unexpected message type", ErrInvalidConventionalCommit)
	}

	scope := ""
	if commit.Scope != nil {
		scope = strings.TrimSpace(*commit.Scope)
	}

	result := Result{
		Title:          trimmed,
		Type:           strings.TrimSpace(commit.Type),
		Scope:          scope,
		Description:    strings.TrimSpace(commit.Description),
		BreakingChange: commit.IsBreakingChange(),
	}

	intent, err := bumpFromCommit(commit)
	if err != nil {
		return bump.BumpPatch, result, err
	}

	return intent, result, nil
}

func bumpFromCommit(commit *conventionalcommits.ConventionalCommit) (bump.Bump, error) {
	if commit == nil {
		return bump.BumpPatch, fmt.Errorf("%w: nil commit", ErrInvalidConventionalCommit)
	}

	if commit.IsBreakingChange() {
		return bump.BumpMajor, nil
	}

	commitType := strings.ToLower(strings.TrimSpace(commit.Type))
	switch commitType {
	case "feat":
		return bump.BumpMinor, nil
	case "perf":
		return bump.BumpMinor, nil
	case "fix":
		return bump.BumpPatch, nil
	case "build", "ci", "chore", "docs", "refactor", "revert", "style", "test":
		return bump.BumpPatch, nil
	default:
		return bump.BumpPatch, fmt.Errorf("%w: unsupported commit type %q", ErrInvalidConventionalCommit, commit.Type)
	}
}
