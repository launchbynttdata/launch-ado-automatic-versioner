package tagplan

import (
	"testing"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/bump"
)

const (
	errPlanRelease = "plan release: %v"
	errPlanRC      = "plan rc: %v"
)

func TestPlanReleaseUsesHighestRelease(t *testing.T) {
	t.Parallel()

	planner := NewPlanner("v")
	tags := []Tag{
		{Name: "refs/tags/v1.2.3"},
		{Name: "refs/tags/v2.0.0"},
		{Name: "refs/tags/v2.0.1"},
		{Name: "refs/tags/v2.1.0-rc.1"},
	}

	result, err := planner.PlanRelease(tags, bump.BumpMinor, "")
	if err != nil {
		t.Fatalf(errPlanRelease, err)
	}

	if result.TagName != "v2.1.0" {
		t.Fatalf("tag name: want v2.1.0 got %s", result.TagName)
	}
	if result.BaseSource != BaseSourceExisting {
		t.Fatalf("base source: want existing got %s", result.BaseSource)
	}
	if result.ReleaseBase.String() != "2.0.1" {
		t.Fatalf("base version: want 2.0.1 got %s", result.ReleaseBase.String())
	}
}

func TestPlanReleaseRespectsBaseOverride(t *testing.T) {
	t.Parallel()

	planner := NewPlanner("v")
	tags := []Tag{
		{Name: "refs/tags/v0.9.0-rc.1"},
		{Name: "invalid"},
	}

	result, err := planner.PlanRelease(tags, bump.BumpPatch, "v0.9.0")
	if err != nil {
		t.Fatalf(errPlanRelease, err)
	}

	if result.TagName != "v0.9.1" {
		t.Fatalf("tag name: want v0.9.1 got %s", result.TagName)
	}
	if result.BaseSource != BaseSourceConfigured {
		t.Fatalf("base source: want configured got %s", result.BaseSource)
	}
	if result.ReleaseBase.String() != "0.9.0" {
		t.Fatalf("base version: want 0.9.0 got %s", result.ReleaseBase.String())
	}
}

func TestPlanReleaseDefaultsToZeroBase(t *testing.T) {
	t.Parallel()

	planner := NewPlanner("v")

	result, err := planner.PlanRelease(nil, bump.BumpPatch, "")
	if err != nil {
		t.Fatalf(errPlanRelease, err)
	}

	if result.TagName != "v0.0.1" {
		t.Fatalf("tag name: want v0.0.1 got %s", result.TagName)
	}
	if result.BaseSource != BaseSourceZero {
		t.Fatalf("base source: want zero got %s", result.BaseSource)
	}
	if result.ReleaseBase.String() != "0.0.0" {
		t.Fatalf("base version: want 0.0.0 got %s", result.ReleaseBase.String())
	}
}

func TestPlanRCAllocatesNextSequence(t *testing.T) {
	t.Parallel()

	planner := NewPlanner("v")
	tags := []Tag{
		{Name: "refs/tags/v1.0.0"},
		{Name: "refs/tags/v2.0.0"},
		{Name: "refs/tags/v2.1.0-rc.1"},
		{Name: "refs/tags/v2.1.0-rc.3"},
		{Name: "refs/tags/v2.1.0-beta.1"},
		{Name: "refs/tags/garbage"},
	}

	result, err := planner.PlanRC(tags, bump.BumpMinor, "")
	if err != nil {
		t.Fatalf(errPlanRC, err)
	}

	if result.TagName != "v2.1.0-rc.4" {
		t.Fatalf("tag name: want v2.1.0-rc.4 got %s", result.TagName)
	}
	if result.RCNumber != 4 {
		t.Fatalf("rc number: want 4 got %d", result.RCNumber)
	}
	if result.TargetRelease.String() != "2.1.0" {
		t.Fatalf("target release: want 2.1.0 got %s", result.TargetRelease.String())
	}
}

func TestPlanReleaseFloatingDetection(t *testing.T) {
	t.Parallel()

	planner := NewPlanner("v")
	tags := []Tag{
		{Name: "refs/tags/v1.2.3", ObjectID: "abc"},
		{Name: "refs/tags/v1", ObjectID: "abc"},
	}

	result, err := planner.PlanRelease(tags, bump.BumpPatch, "")
	if err != nil {
		t.Fatalf(errPlanRelease, err)
	}

	if !result.Floating.AutoDetected {
		t.Fatalf("expected floating tag auto detection")
	}
	if result.Floating.TagName != "v1" {
		t.Fatalf("expected target floating tag v1 got %s", result.Floating.TagName)
	}
	if result.Floating.Existing.Name != "refs/tags/v1" {
		t.Fatalf("expected existing floating tag to be captured")
	}
}

func TestPlanReleaseFloatingDetectionIgnoresMismatchedRefs(t *testing.T) {
	t.Parallel()

	planner := NewPlanner("v")
	tags := []Tag{
		{Name: "refs/tags/v1.2.3", ObjectID: "abc"},
		{Name: "refs/tags/v1", ObjectID: "def"},
	}

	result, err := planner.PlanRelease(tags, bump.BumpPatch, "")
	if err != nil {
		t.Fatalf(errPlanRelease, err)
	}

	if result.Floating.AutoDetected {
		t.Fatalf("did not expect auto detection when commits differ")
	}
	if result.Floating.Existing.Name != "refs/tags/v1" {
		t.Fatalf("expected floating tag reference to be recorded")
	}
}

func TestPlanReleaseFloatingTagNameFollowsNextMajor(t *testing.T) {
	t.Parallel()

	planner := NewPlanner("v")
	tags := []Tag{{Name: "refs/tags/v1.2.3", ObjectID: "abc"}}

	result, err := planner.PlanRelease(tags, bump.BumpMajor, "")
	if err != nil {
		t.Fatalf(errPlanRelease, err)
	}

	if result.Floating.TagName != "v2" {
		t.Fatalf("expected floating tag name v2 got %s", result.Floating.TagName)
	}
	if result.Floating.Existing.Name != "" {
		t.Fatalf("did not expect existing floating tag for new major")
	}
}

func TestPlanRCRespectsBaseOverride(t *testing.T) {
	t.Parallel()

	planner := NewPlanner("v")

	result, err := planner.PlanRC(nil, bump.BumpMinor, "0.9.0")
	if err != nil {
		t.Fatalf(errPlanRC, err)
	}

	if result.TagName != "v0.10.0-rc.1" {
		t.Fatalf("tag name: want v0.10.0-rc.1 got %s", result.TagName)
	}
	if result.RCNumber != 1 {
		t.Fatalf("rc number: want 1 got %d", result.RCNumber)
	}
	if result.BaseSource != BaseSourceConfigured {
		t.Fatalf("base source: want configured got %s", result.BaseSource)
	}
}

func TestPlanReleaseInvalidBaseError(t *testing.T) {
	t.Parallel()

	planner := NewPlanner("v")

	if _, err := planner.PlanRelease(nil, bump.BumpPatch, "not-a-version"); err == nil {
		t.Fatalf("expected error for invalid base version")
	}

	if _, err := planner.PlanRC(nil, bump.BumpPatch, "not-a-version"); err == nil {
		t.Fatalf("expected error for invalid base version in rc mode")
	}
}

func TestPlanReleaseWithoutPrefix(t *testing.T) {
	t.Parallel()

	planner := NewPlanner("")
	tags := []Tag{{Name: "refs/tags/v1.0.0"}}

	result, err := planner.PlanRelease(tags, bump.BumpMinor, "")
	if err != nil {
		t.Fatalf(errPlanRelease, err)
	}

	if result.TagName != "1.1.0" {
		t.Fatalf("tag name: want 1.1.0 got %s", result.TagName)
	}
}
