package inferbump

import (
	"context"
	"errors"
	"testing"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/ado"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/bump"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/labels"
)

const resolveErrFormat = "resolve: %v"

func TestResolveSelectsHighestImpactLabel(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		prID:   42,
		labels: []string{"semver-patch", "semver-major"},
	}

	svc := NewService(client, labels.NewResolver(labels.Config{}))

	result, err := svc.Resolve(context.Background(), Config{CommitSHA: "abc123"})
	if err != nil {
		t.Fatalf(resolveErrFormat, err)
	}

	if result.Bump != bump.BumpMajor {
		t.Fatalf("expected major bump got %v", result.Bump)
	}
	if result.PRID != 42 {
		t.Fatalf("unexpected pr id %d", result.PRID)
	}
	if result.Defaulted {
		t.Fatalf("expected explicit resolution, got default")
	}
	if len(result.SemverLabels) != 2 {
		t.Fatalf("expected semver labels to be captured")
	}
}

func TestResolveDefaultsWhenNoSemverLabels(t *testing.T) {
	t.Parallel()

	client := &fakeClient{prID: 71, labels: []string{"needs-review"}}
	svc := NewService(client, labels.NewResolver(labels.Config{}))

	result, err := svc.Resolve(context.Background(), Config{CommitSHA: "fff"})
	if err != nil {
		t.Fatalf(resolveErrFormat, err)
	}

	if !result.Defaulted || result.DefaultReason != DefaultReasonNoSemverLabels {
		t.Fatalf("expected default due to missing semver labels")
	}
	if result.Bump != bump.BumpPatch {
		t.Fatalf("expected patch default got %v", result.Bump)
	}
}

func TestResolveDefaultsWhenNoPullRequestNonStrict(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	svc := NewService(client, labels.NewResolver(labels.Config{}))

	result, err := svc.Resolve(context.Background(), Config{CommitSHA: "deadbeef"})
	if err != nil {
		t.Fatalf(resolveErrFormat, err)
	}

	if !result.Defaulted || result.DefaultReason != DefaultReasonNoPullRequest {
		t.Fatalf("expected default due to missing pr")
	}
	if result.Bump != bump.BumpPatch {
		t.Fatalf("expected patch default, got %v", result.Bump)
	}
}

func TestResolveStrictErrorWhenNoPullRequest(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeClient{}, labels.NewResolver(labels.Config{}))

	_, err := svc.Resolve(context.Background(), Config{CommitSHA: "deadbeef", Strict: true})
	if err == nil {
		t.Fatalf("expected error for strict mode")
	}
	if !errors.Is(err, ado.ErrPullRequestNotFound) {
		t.Fatalf("expected ErrPullRequestNotFound got %v", err)
	}
}

func TestResolveClientErrors(t *testing.T) {
	t.Parallel()

	client := &fakeClient{prID: 5, labelsErr: errors.New("boom")}
	svc := NewService(client, labels.NewResolver(labels.Config{}))

	if _, err := svc.Resolve(context.Background(), Config{CommitSHA: "abc"}); err == nil {
		t.Fatalf("expected error from label listing")
	}
}

func TestResolveValidations(t *testing.T) {
	t.Parallel()

	svc := NewService(nil, labels.NewResolver(labels.Config{}))
	if _, err := svc.Resolve(context.Background(), Config{CommitSHA: "abc"}); !errors.Is(err, ErrNilClient) {
		t.Fatalf("expected ErrNilClient got %v", err)
	}

	svc = NewService(&fakeClient{}, labels.NewResolver(labels.Config{}))
	if _, err := svc.Resolve(context.Background(), Config{CommitSHA: "   "}); !errors.Is(err, ErrEmptyCommit) {
		t.Fatalf("expected ErrEmptyCommit got %v", err)
	}
}

type fakeClient struct {
	prID      int
	prErr     error
	labels    []string
	labelsErr error
}

func (f *fakeClient) ListRefsWithPrefix(context.Context, string) ([]ado.Ref, error) {
	return nil, nil
}

func (f *fakeClient) FindPullRequestByMergeCommit(_ context.Context, _ string) (int, error) {
	if f.prErr != nil {
		return 0, f.prErr
	}
	if f.prID == 0 {
		return 0, ado.ErrPullRequestNotFound
	}
	return f.prID, nil
}

func (f *fakeClient) ListPRLabels(context.Context, int) ([]string, error) {
	if f.labelsErr != nil {
		return nil, f.labelsErr
	}
	if len(f.labels) == 0 {
		return nil, nil
	}
	out := make([]string, len(f.labels))
	copy(out, f.labels)
	return out, nil
}

func (f *fakeClient) AddPRLabel(context.Context, int, string) error {
	return nil
}

func (f *fakeClient) CreateAnnotatedTag(context.Context, ado.TagSpec) error {
	return nil
}
