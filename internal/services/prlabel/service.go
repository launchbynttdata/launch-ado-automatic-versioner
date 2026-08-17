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
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/prtitle"
)

var (
	ErrNilClient   = errors.New("prlabel service: nil ado client")
	ErrInvalidPR   = errors.New("prlabel service: invalid pr id")
	ErrEmptyBranch = errors.New("prlabel service: empty branch")
	// ErrADOAPI indicates an Azure DevOps API failure during labeling.
	ErrADOAPI = errors.New("prlabel service: ado api error")
)

// BumpSource describes how bump intent was derived.
type BumpSource string

const (
	BumpSourceBranch         BumpSource = "branch"
	BumpSourcePRTitle        BumpSource = "pr-title"
	BumpSourceBranchFallback BumpSource = "branch-fallback"
)

// Config captures the inputs required to label a pull request.
type Config struct {
	PRID                    int
	Branch                  string
	UsePRTitle              bool
	AllowBranchNameFallback bool
}

// Result summarizes the decision applied to the pull request.
type Result struct {
	Bump           bump.Bump
	BumpSource     BumpSource
	FallbackUsed   bool
	FallbackReason string
	BranchMatched  bool
	MatchedPrefix  string
	PRTitle        string
	CommitType     string
	CommitScope    string
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
	prtitles prtitle.Resolver
}

// NewService constructs a Service instance.
func NewService(client ado.Client, branches branchmap.Resolver, labels labels.Resolver) Service {
	return Service{
		client:   client,
		branches: branches,
		labels:   labels,
		prtitles: prtitle.NewResolver(),
	}
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
	if !cfg.UsePRTitle && branch == "" {
		return Result{}, ErrEmptyBranch
	}
	if cfg.UsePRTitle && cfg.AllowBranchNameFallback && branch == "" {
		return Result{}, ErrEmptyBranch
	}

	bumpIntent, result, err := s.resolveBumpIntent(ctx, cfg, branch)
	if err != nil {
		return result, err
	}

	existing, err := s.client.ListPRLabels(ctx, cfg.PRID)
	if err != nil {
		return result, fmt.Errorf("%w: listing pr labels: %w", ErrADOAPI, err)
	}

	decision := s.labels.Decide(existing, bumpIntent)
	result.Decision = decision.Decision
	result.ExpectedLabel = decision.ExpectedLabel
	if len(decision.Existing) > 0 {
		result.ExistingSemver = append([]string(nil), decision.Existing...)
	}

	if decision.Decision == labels.DecisionAddExpected {
		if err := s.client.AddPRLabel(ctx, cfg.PRID, decision.ExpectedLabel); err != nil {
			return result, fmt.Errorf("%w: adding pr label: %w", ErrADOAPI, err)
		}
		result.LabelAdded = true
	}

	return result, nil
}

func (s Service) resolveBumpIntent(ctx context.Context, cfg Config, branch string) (bump.Bump, Result, error) {
	if !cfg.UsePRTitle {
		return s.resolveFromBranch(branch)
	}

	title, err := s.client.GetPullRequestTitle(ctx, cfg.PRID)
	if err != nil {
		return bump.BumpPatch, Result{}, fmt.Errorf("%w: getting pull request title: %w", ErrADOAPI, err)
	}

	bumpIntent, parsed, err := s.prtitles.Resolve(title)
	if err == nil {
		return bumpIntent, Result{
			Bump:        bumpIntent,
			BumpSource:  BumpSourcePRTitle,
			PRTitle:     parsed.Title,
			CommitType:  parsed.Type,
			CommitScope: parsed.Scope,
		}, nil
	}

	trimmedTitle := strings.TrimSpace(title)
	if !cfg.AllowBranchNameFallback {
		return bump.BumpPatch, Result{
			BumpSource:     BumpSourcePRTitle,
			PRTitle:        trimmedTitle,
			FallbackReason: err.Error(),
		}, err
	}

	// Apply already requires a non-empty branch when fallback is enabled.
	bumpIntent, matchedPrefix, matched := s.branches.Resolve(branch)
	return bumpIntent, Result{
		Bump:           bumpIntent,
		BumpSource:     BumpSourceBranchFallback,
		FallbackUsed:   true,
		FallbackReason: err.Error(),
		BranchMatched:  matched,
		MatchedPrefix:  matchedPrefix,
		PRTitle:        trimmedTitle,
	}, nil
}

func (s Service) resolveFromBranch(branch string) (bump.Bump, Result, error) {
	trimmed := strings.TrimSpace(branch)
	if trimmed == "" {
		return bump.BumpPatch, Result{}, ErrEmptyBranch
	}

	bumpIntent, matchedPrefix, matched := s.branches.Resolve(trimmed)
	return bumpIntent, Result{
		Bump:          bumpIntent,
		BumpSource:    BumpSourceBranch,
		BranchMatched: matched,
		MatchedPrefix: matchedPrefix,
	}, nil
}
