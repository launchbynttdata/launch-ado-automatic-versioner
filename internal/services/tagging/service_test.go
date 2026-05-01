package tagging

import (
	"context"
	"errors"
	"testing"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/ado/adotest"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/bump"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/tagplan"
)

const (
	sampleReleaseTag      = "refs/tags/v1.2.3"
	sampleReleaseObjectID = "1111111111111111111111111111111111111111"
	sampleRCtag           = "refs/tags/v1.3.0-rc.1"
	sampleRCObjectID      = "2222222222222222222222222222222222222222"
	taggerNameDefault     = "bot"
	taggerEmailDefault    = "bot@example.com"
)

func TestPlanReleaseFromExistingTags(t *testing.T) {
	t.Parallel()

	client := adotest.NewClient()
	client.SeedAnnotatedTag(sampleReleaseTag, "release-tag-object", sampleReleaseObjectID)
	client.SeedAnnotatedTag(sampleRCtag, "rc-tag-object", sampleRCObjectID)

	svc := NewService(client, tagplan.NewPlanner("v"))

	result, err := svc.Plan(context.Background(), Config{Mode: tagplan.ModeRelease, Bump: bump.BumpPatch})
	if err != nil {
		t.Fatalf("plan release: %v", err)
	}

	if client.LastPrefix != tagRefPrefix {
		t.Fatalf("expected prefix %s got %s", tagRefPrefix, client.LastPrefix)
	}

	if result.TagName != "v1.2.4" {
		t.Fatalf("tag name: want v1.2.4 got %s", result.TagName)
	}
}

func TestPlanRCUsesPlanner(t *testing.T) {
	t.Parallel()

	client := adotest.NewClient()
	client.SeedAnnotatedTag("v1.2.3", "release-tag-object", sampleReleaseObjectID)
	client.SeedAnnotatedTag("v1.3.0-rc.1", "rc-tag-object", sampleRCObjectID)

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

	client := adotest.NewClient()
	client.SeedAnnotatedTag(sampleReleaseTag, "release-tag-object", sampleReleaseObjectID)

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

	if len(client.CreatedTags) != 1 {
		t.Fatalf("expected exactly one tag creation got %d", len(client.CreatedTags))
	}
	releaseTag := client.CreatedTags[0]
	if releaseTag.Name != result.TagName {
		t.Fatalf("expected created tag name %s got %s", result.TagName, releaseTag.Name)
	}
	if releaseTag.ObjectID != "deadbeef" {
		t.Fatalf("expected object id deadbeef got %s", releaseTag.ObjectID)
	}
	if releaseTag.Message != "release v1.3.0" {
		t.Fatalf("unexpected message %s", releaseTag.Message)
	}
	if releaseTag.TaggerName != taggerNameDefault || releaseTag.TaggerEmail != taggerEmailDefault {
		t.Fatalf("unexpected tagger info %#v", releaseTag)
	}

	ref, ok := client.Ref(result.TagName)
	if !ok {
		t.Fatalf("expected release ref %s to exist", result.TagName)
	}
	if ref.ObjectID == "deadbeef" {
		t.Fatalf("expected annotated tag ref object id to differ from peeled commit")
	}
	if ref.PeeledObjectID != "deadbeef" {
		t.Fatalf("expected release ref to peel to deadbeef got %s", ref.PeeledObjectID)
	}
}

func TestPlanAndCreateCreatesFloatingTagWhenEnabled(t *testing.T) {
	t.Parallel()

	client := adotest.NewClient()
	client.SeedAnnotatedTag(sampleReleaseTag, "release-tag-object", sampleReleaseObjectID)

	svc := NewService(client, tagplan.NewPlanner("v"))

	cfg := CreateConfig{
		Config:      Config{Mode: tagplan.ModeRelease, Bump: bump.BumpPatch, UseFloatingTags: true},
		CommitSHA:   "deadbeef",
		Message:     "release v1.2.4",
		TaggerName:  taggerNameDefault,
		TaggerEmail: taggerEmailDefault,
	}

	result, err := svc.PlanAndCreate(context.Background(), cfg)
	if err != nil {
		t.Fatalf("plan and create: %v", err)
	}

	if len(client.CreatedTags) != 2 {
		t.Fatalf("expected release and floating tag creations got %d", len(client.CreatedTags))
	}
	if client.CreatedTags[1].Name != "v1" {
		t.Fatalf("expected floating tag v1 got %s", client.CreatedTags[1].Name)
	}
	if !result.Floating.Enabled || !result.Floating.Created {
		t.Fatalf("expected floating tag metadata to signal creation: %+v", result.Floating)
	}

	ref, ok := client.Ref("v1")
	if !ok {
		t.Fatalf("expected floating ref v1 to exist")
	}
	if ref.PeeledObjectID != "deadbeef" {
		t.Fatalf("expected floating ref to peel to deadbeef got %s", ref.PeeledObjectID)
	}
}

func TestPlanAndCreateAutoDetectsFloatingTag(t *testing.T) {
	t.Parallel()

	client := adotest.NewClient()
	client.SeedAnnotatedTag(sampleReleaseTag, "release-tag-object", sampleReleaseObjectID)
	client.SeedAnnotatedTag("v1", "floating-tag-object", sampleReleaseObjectID)

	svc := NewService(client, tagplan.NewPlanner("v"))

	cfg := CreateConfig{
		Config:      Config{Mode: tagplan.ModeRelease, Bump: bump.BumpPatch},
		CommitSHA:   "deadbeef",
		Message:     "release v1.2.4",
		TaggerName:  taggerNameDefault,
		TaggerEmail: taggerEmailDefault,
	}

	result, err := svc.PlanAndCreate(context.Background(), cfg)
	if err != nil {
		t.Fatalf("plan and create: %v", err)
	}

	if !result.Floating.AutoDetected {
		t.Fatalf("expected floating tag auto detection")
	}
	if !result.Floating.Enabled || !result.Floating.Created {
		t.Fatalf("expected floating tag to be updated")
	}
	if len(client.DeletedRefs) != 1 {
		t.Fatalf("expected floating tag deletion before recreation")
	}
	if client.DeletedRefs[0].OldObjectID != "floating-tag-object" {
		t.Fatalf("expected delete to use ref object id, got %s", client.DeletedRefs[0].OldObjectID)
	}

	ref, ok := client.Ref("v1")
	if !ok {
		t.Fatalf("expected floating ref v1 to exist after update")
	}
	if ref.ObjectID == "floating-tag-object" {
		t.Fatalf("expected floating ref to be recreated with a new tag object id")
	}
	if ref.PeeledObjectID != "deadbeef" {
		t.Fatalf("expected floating ref to peel to deadbeef got %s", ref.PeeledObjectID)
	}
}

func TestPlanAndCreateValidations(t *testing.T) {
	t.Parallel()

	client := adotest.NewClient()
	client.SeedAnnotatedTag("v0.0.0", "release-tag-object", sampleReleaseObjectID)
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

	client.CreateErr = errors.New("boom")
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

	client := adotest.NewClient()
	svc = NewService(client, tagplan.NewPlanner("v"))
	if _, err := svc.Plan(context.Background(), Config{Mode: "bad"}); !errors.Is(err, ErrInvalidMode) {
		t.Fatalf("expected ErrInvalidMode got %v", err)
	}

	client.ListErr = errors.New("boom")
	if _, err := svc.Plan(context.Background(), Config{Mode: tagplan.ModeRelease}); err == nil {
		t.Fatalf("expected error for client failure")
	}
}
