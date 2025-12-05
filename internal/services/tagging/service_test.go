package tagging

import (
	"context"
	"errors"
	"testing"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/ado"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/bump"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/tagplan"
)

const (
	sampleReleaseTag   = "refs/tags/v1.2.3"
	sampleRCtag        = "refs/tags/v1.3.0-rc.1"
	taggerNameDefault  = "bot"
	taggerEmailDefault = "bot@example.com"
)

func TestPlanReleaseFromExistingTags(t *testing.T) {
	t.Parallel()

	client := &fakeRefClient{
		refs: []ado.Ref{{Name: sampleReleaseTag}, {Name: sampleRCtag}},
	}

	svc := NewService(client, tagplan.NewPlanner("v"))

	result, err := svc.Plan(context.Background(), Config{Mode: tagplan.ModeRelease, Bump: bump.BumpPatch})
	if err != nil {
		t.Fatalf("plan release: %v", err)
	}

	if client.lastPrefix != tagRefPrefix {
		t.Fatalf("expected prefix %s got %s", tagRefPrefix, client.lastPrefix)
	}

	if result.TagName != "v1.2.4" {
		t.Fatalf("tag name: want v1.2.4 got %s", result.TagName)
	}
}

func TestPlanRCUsesPlanner(t *testing.T) {
	t.Parallel()

	client := &fakeRefClient{
		refs: []ado.Ref{{Name: "refs/tags/v1.2.3"}, {Name: "refs/tags/v1.3.0-rc.1"}},
	}

	svc := NewService(client, tagplan.NewPlanner("v"))

	result, err := svc.Plan(context.Background(), Config{Mode: tagplan.ModeRC, Bump: bump.BumpMinor})
	if err != nil {
		t.Fatalf("plan rc: %v", err)
	}

	if result.TagName != "v1.3.0-rc.2" {
		t.Fatalf("tag name: want v1.3.0-rc.2 got %s", result.TagName)
	}
}

func TestPlanAndCreateCreatesTag(t *testing.T) {
	t.Parallel()

	client := &fakeRefClient{
		refs: []ado.Ref{{Name: sampleReleaseTag}},
	}

	svc := NewService(client, tagplan.NewPlanner("v"))

	cfg := CreateConfig{
		Config:      Config{Mode: tagplan.ModeRelease, Bump: bump.BumpMinor},
		CommitSHA:   "deadbeef",
		Message:     "release v1.3.0",
		TaggerName:  taggerNameDefault,
		TaggerEmail: taggerEmailDefault,
	}

	result, err := svc.PlanAndCreate(context.Background(), cfg)
	if err != nil {
		t.Fatalf("plan and create: %v", err)
	}

	if result.TagName != "v1.3.0" {
		t.Fatalf("expected tag name v1.3.0 got %s", result.TagName)
	}

	if client.createdTag == nil {
		t.Fatalf("expected annotated tag to be created")
	}

	if client.createdTag.Name != result.TagName {
		t.Fatalf("expected created tag name %s got %s", result.TagName, client.createdTag.Name)
	}
	if client.createdTag.ObjectID != "deadbeef" {
		t.Fatalf("expected object id deadbeef got %s", client.createdTag.ObjectID)
	}
	if client.createdTag.Message != "release v1.3.0" {
		t.Fatalf("unexpected message %s", client.createdTag.Message)
	}
	if client.createdTag.TaggerName != taggerNameDefault || client.createdTag.TaggerEmail != taggerEmailDefault {
		t.Fatalf("unexpected tagger info %#v", client.createdTag)
	}
}

func TestPlanAndCreateValidations(t *testing.T) {
	t.Parallel()

	client := &fakeRefClient{refs: []ado.Ref{{Name: "refs/tags/v0.0.0"}}}
	svc := NewService(client, tagplan.NewPlanner("v"))

	baseCfg := CreateConfig{Config: Config{Mode: tagplan.ModeRelease}, TaggerName: taggerNameDefault, TaggerEmail: taggerEmailDefault}

	if _, err := svc.PlanAndCreate(context.Background(), baseCfg); !errors.Is(err, ErrEmptyCommit) {
		t.Fatalf("expected ErrEmptyCommit got %v", err)
	}

	if _, err := svc.PlanAndCreate(context.Background(), CreateConfig{Config: Config{Mode: tagplan.ModeRelease}, CommitSHA: "abc", TaggerEmail: taggerEmailDefault}); !errors.Is(err, ErrEmptyTagger) {
		t.Fatalf("expected ErrEmptyTagger got %v", err)
	}

	if _, err := svc.PlanAndCreate(context.Background(), CreateConfig{Config: Config{Mode: tagplan.ModeRelease}, CommitSHA: "abc", TaggerName: "bot"}); !errors.Is(err, ErrEmptyEmail) {
		t.Fatalf("expected ErrEmptyEmail got %v", err)
	}

	client.createErr = errors.New("boom")
	_, err := svc.PlanAndCreate(context.Background(), CreateConfig{
		Config:      Config{Mode: tagplan.ModeRelease},
		CommitSHA:   "abc",
		TaggerName:  taggerNameDefault,
		TaggerEmail: taggerEmailDefault,
	})
	if err == nil {
		t.Fatalf("expected error when tag creation fails")
	}
}

func TestPlanValidations(t *testing.T) {
	t.Parallel()

	svc := NewService(nil, tagplan.NewPlanner("v"))
	if _, err := svc.Plan(context.Background(), Config{Mode: tagplan.ModeRelease}); !errors.Is(err, ErrNilClient) {
		t.Fatalf("expected ErrNilClient got %v", err)
	}

	client := &fakeRefClient{}
	svc = NewService(client, tagplan.NewPlanner("v"))
	if _, err := svc.Plan(context.Background(), Config{Mode: "bad"}); !errors.Is(err, ErrInvalidMode) {
		t.Fatalf("expected ErrInvalidMode got %v", err)
	}

	client.err = errors.New("boom")
	if _, err := svc.Plan(context.Background(), Config{Mode: tagplan.ModeRelease}); err == nil {
		t.Fatalf("expected error for client failure")
	}
}

type fakeRefClient struct {
	refs       []ado.Ref
	err        error
	lastPrefix string
	createdTag *ado.TagSpec
	createErr  error
}

func (f *fakeRefClient) ListRefsWithPrefix(_ context.Context, prefix string) ([]ado.Ref, error) {
	f.lastPrefix = prefix
	return f.refs, f.err
}

func (f *fakeRefClient) ListPRLabels(context.Context, int) ([]string, error) {
	return nil, nil
}

func (f *fakeRefClient) AddPRLabel(context.Context, int, string) error {
	return nil
}

func (f *fakeRefClient) FindPullRequestByMergeCommit(context.Context, string) (int, error) {
	return 0, ado.ErrPullRequestNotFound
}

func (f *fakeRefClient) CreateAnnotatedTag(_ context.Context, spec ado.TagSpec) error {
	if f.createErr != nil {
		return f.createErr
	}
	copy := spec
	f.createdTag = &copy
	return nil
}
