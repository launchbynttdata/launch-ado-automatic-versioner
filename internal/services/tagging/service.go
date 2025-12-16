package tagging

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/ado"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/bump"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/tagplan"
)

const tagRefPrefix = "refs/tags/"

var (
	ErrNilClient   = errors.New("tagging service: nil ado client")
	ErrInvalidMode = errors.New("tagging service: invalid mode")
	ErrEmptyCommit = errors.New("tagging service: commit sha is empty")
	ErrEmptyTagger = errors.New("tagging service: tagger name is empty")
	ErrEmptyEmail  = errors.New("tagging service: tagger email is empty")
)

// Config captures the inputs required to compute the next tag.
type Config struct {
	Mode            tagplan.Mode
	Bump            bump.Bump
	BaseVersion     string
	UseFloatingTags bool
}

// CreateConfig extends Config with the metadata required to create the annotated tag.
type CreateConfig struct {
	Config
	CommitSHA   string
	Message     string
	TaggerName  string
	TaggerEmail string
}

// Service orchestrates fetching ADO refs and delegating to the tag planner.
type Service struct {
	client  ado.Client
	planner tagplan.Planner
}

// NewService constructs a Service instance.
func NewService(client ado.Client, planner tagplan.Planner) Service {
	return Service{client: client, planner: planner}
}

// Plan fetches refs from ADO and returns the next tag plan result.
func (s Service) Plan(ctx context.Context, cfg Config) (tagplan.Result, error) {
	if s.client == nil {
		return tagplan.Result{}, ErrNilClient
	}

	refs, err := s.client.ListRefsWithPrefix(ctx, tagRefPrefix)
	if err != nil {
		return tagplan.Result{}, fmt.Errorf("listing refs: %w", err)
	}

	tags := toPlannerTags(refs)

	switch cfg.Mode {
	case tagplan.ModeRelease:
		return s.planner.PlanRelease(tags, cfg.Bump, cfg.BaseVersion)
	case tagplan.ModeRC:
		return s.planner.PlanRC(tags, cfg.Bump, cfg.BaseVersion)
	default:
		return tagplan.Result{}, ErrInvalidMode
	}
}

// PlanAndCreate computes the next tag and creates it in ADO as an annotated tag.
func (s Service) PlanAndCreate(ctx context.Context, cfg CreateConfig) (tagplan.Result, error) {
	plan, err := s.Plan(ctx, cfg.Config)
	if err != nil {
		return tagplan.Result{}, err
	}

	commit := strings.TrimSpace(cfg.CommitSHA)
	if commit == "" {
		return tagplan.Result{}, ErrEmptyCommit
	}

	taggerName := strings.TrimSpace(cfg.TaggerName)
	if taggerName == "" {
		return tagplan.Result{}, ErrEmptyTagger
	}

	taggerEmail := strings.TrimSpace(cfg.TaggerEmail)
	if taggerEmail == "" {
		return tagplan.Result{}, ErrEmptyEmail
	}

	spec := ado.TagSpec{
		Name:        plan.TagName,
		ObjectID:    commit,
		ObjectType:  ado.TagObjectTypeCommit,
		Message:     strings.TrimSpace(cfg.Message),
		TaggerName:  taggerName,
		TaggerEmail: taggerEmail,
	}

	if err := s.client.CreateAnnotatedTag(ctx, spec); err != nil {
		return tagplan.Result{}, fmt.Errorf("creating annotated tag: %w", err)
	}

	if plan.Mode == tagplan.ModeRelease {
		if err := s.applyFloatingTag(ctx, cfg, &plan, spec); err != nil {
			return tagplan.Result{}, err
		}
	}

	return plan, nil
}

func toPlannerTags(refs []ado.Ref) []tagplan.Tag {
	if len(refs) == 0 {
		return nil
	}

	tags := make([]tagplan.Tag, 0, len(refs))
	for _, ref := range refs {
		tags = append(tags, tagplan.Tag{Name: ref.Name, ObjectID: ref.ObjectID})
	}
	return tags
}

func (s Service) applyFloatingTag(ctx context.Context, cfg CreateConfig, plan *tagplan.Result, releaseSpec ado.TagSpec) error {
	if plan == nil {
		return nil
	}

	enabled := cfg.UseFloatingTags || plan.Floating.AutoDetected
	if !enabled {
		return nil
	}

	plan.Floating.Enabled = true

	floatingName := strings.TrimSpace(plan.Floating.TagName)
	if floatingName == "" {
		floatingName = fmt.Sprintf("v%d", plan.Version.Major)
		plan.Floating.TagName = floatingName
	}

	spec := releaseSpec
	spec.Name = floatingName

	if existingName := strings.TrimSpace(plan.Floating.Existing.Name); existingName != "" {
		objectID := strings.TrimSpace(plan.Floating.Existing.ObjectID)
		if objectID == "" {
			return fmt.Errorf("floating tag %s missing object id", existingName)
		}
		if err := s.client.DeleteRef(ctx, existingName, objectID); err != nil {
			return fmt.Errorf("deleting floating tag %s: %w", existingName, err)
		}
		plan.Floating.DeletedExisting = true
	}

	if err := s.client.CreateAnnotatedTag(ctx, spec); err != nil {
		return fmt.Errorf("creating floating tag %s: %w", spec.Name, err)
	}
	plan.Floating.Created = true
	return nil
}
