package inferbump

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/ado"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/bump"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/labels"
)

var (
	ErrNilClient   = errors.New("inferbump service: nil ado client")
	ErrEmptyCommit = errors.New("inferbump service: empty commit sha")
)

// DefaultReason explains why a default bump was chosen.
type DefaultReason string

const (
	DefaultReasonNone           DefaultReason = ""
	DefaultReasonNoPullRequest  DefaultReason = "no-pull-request"
	DefaultReasonNoSemverLabels DefaultReason = "no-semver-labels"
)

// Config captures the inputs required to infer a bump intent.
type Config struct {
	CommitSHA string
	Strict    bool
}

// Result summarizes the resolution outcome.
type Result struct {
	Bump          bump.Bump
	PRID          int
	CommitSHA     string
	Labels        []string
	SemverLabels  []string
	Defaulted     bool
	DefaultReason DefaultReason
}

// Service determines bump intent for a merge commit by inspecting PR labels.
type Service struct {
	client ado.Client
	labels labels.Resolver
}

// NewService constructs a Service instance.
func NewService(client ado.Client, labels labels.Resolver) Service {
	return Service{client: client, labels: labels}
}

// Resolve returns the bump intent for the merge commit reference.
func (s Service) Resolve(ctx context.Context, cfg Config) (Result, error) {
	if s.client == nil {
		return Result{}, ErrNilClient
	}

	commit := strings.TrimSpace(cfg.CommitSHA)
	if commit == "" {
		return Result{}, ErrEmptyCommit
	}

	result := Result{CommitSHA: commit}

	prID, err := s.client.FindPullRequestByMergeCommit(ctx, commit)
	if err != nil {
		if errors.Is(err, ado.ErrPullRequestNotFound) && !cfg.Strict {
			result.Bump = bump.Default()
			result.Defaulted = true
			result.DefaultReason = DefaultReasonNoPullRequest
			return result, nil
		}
		return Result{}, fmt.Errorf("finding pull request by merge commit: %w", err)
	}

	result.PRID = prID

	prLabels, err := s.client.ListPRLabels(ctx, prID)
	if err != nil {
		return result, fmt.Errorf("listing pull request labels: %w", err)
	}

	if len(prLabels) > 0 {
		result.Labels = append([]string(nil), prLabels...)
	}

	var bumpCandidates []bump.Bump
	for _, lbl := range prLabels {
		if b, ok := s.labels.BumpForLabel(lbl); ok {
			result.SemverLabels = append(result.SemverLabels, lbl)
			bumpCandidates = append(bumpCandidates, b)
		}
	}

	if len(bumpCandidates) == 0 {
		result.Bump = bump.Default()
		result.Defaulted = true
		result.DefaultReason = DefaultReasonNoSemverLabels
		return result, nil
	}

	result.Bump = bump.Max(bumpCandidates...)
	return result, nil
}
