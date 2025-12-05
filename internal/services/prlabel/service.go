package prlabel

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/ado"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/branchmap"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/bump"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/labels"
)

var (
	ErrNilClient   = errors.New("prlabel service: nil ado client")
	ErrInvalidPR   = errors.New("prlabel service: invalid pr id")
	ErrEmptyBranch = errors.New("prlabel service: empty branch")
)

// Config captures the inputs required to label a pull request.
type Config struct {
	PRID   int
	Branch string
}

// Result summarizes the decision applied to the pull request.
type Result struct {
	Bump           bump.Bump
	BranchMatched  bool
	MatchedPrefix  string
	Decision       labels.Decision
	ExpectedLabel  string
	ExistingSemver []string
	LabelAdded     bool
}

// Service drives the PR labeling workflow.
type Service struct {
	client   ado.Client
	branches branchmap.Resolver
	labels   labels.Resolver
}

// NewService constructs a Service instance.
func NewService(client ado.Client, branches branchmap.Resolver, labels labels.Resolver) Service {
	return Service{client: client, branches: branches, labels: labels}
}

// Apply ensures the expected semver label is present on the pull request.
func (s Service) Apply(ctx context.Context, cfg Config) (Result, error) {
	if s.client == nil {
		return Result{}, ErrNilClient
	}
	if cfg.PRID <= 0 {
		return Result{}, ErrInvalidPR
	}
	branch := strings.TrimSpace(cfg.Branch)
	if branch == "" {
		return Result{}, ErrEmptyBranch
	}

	bumpIntent, matchedPrefix, matched := s.branches.Resolve(branch)
	result := Result{Bump: bumpIntent, BranchMatched: matched, MatchedPrefix: matchedPrefix}

	existing, err := s.client.ListPRLabels(ctx, cfg.PRID)
	if err != nil {
		return result, fmt.Errorf("listing pr labels: %w", err)
	}

	decision := s.labels.Decide(existing, bumpIntent)
	result.Decision = decision.Decision
	result.ExpectedLabel = decision.ExpectedLabel
	if len(decision.Existing) > 0 {
		result.ExistingSemver = append([]string(nil), decision.Existing...)
	}

	if decision.Decision == labels.DecisionAddExpected {
		if err := s.client.AddPRLabel(ctx, cfg.PRID, decision.ExpectedLabel); err != nil {
			return result, fmt.Errorf("adding pr label: %w", err)
		}
		result.LabelAdded = true
	}

	return result, nil
}
