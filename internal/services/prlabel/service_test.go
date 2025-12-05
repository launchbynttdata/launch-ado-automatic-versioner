package prlabel

import (
	"context"
	"errors"
	"testing"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/ado"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/branchmap"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/labels"
)

func TestApplyAddsLabelWhenMissing(t *testing.T) {
	t.Parallel()

	client := &fakeClient{labels: []string{"needs-review"}}
	svc := NewService(client, branchmap.NewResolver(branchmap.DefaultMapping()), labels.NewResolver(labels.Config{}))

	result, err := svc.Apply(context.Background(), Config{PRID: 42, Branch: "feature/foo"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	if !result.LabelAdded {
		t.Fatalf("expected label to be added")
	}
	if len(client.added) != 1 || client.added[0].label != "semver-minor" {
		t.Fatalf("expected semver-minor to be added, got %#v", client.added)
	}
}

func TestApplyNoopWhenLabelPresent(t *testing.T) {
	t.Parallel()

	client := &fakeClient{labels: []string{"Semver-Minor"}}
	svc := NewService(client, branchmap.NewResolver(branchmap.DefaultMapping()), labels.NewResolver(labels.Config{}))

	result, err := svc.Apply(context.Background(), Config{PRID: 1, Branch: "feature/foo"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	if result.Decision != labels.DecisionNoop {
		t.Fatalf("expected noop decision got %v", result.Decision)
	}
	if len(client.added) != 0 {
		t.Fatalf("unexpected label additions %#v", client.added)
	}
}

func TestApplyConflictDoesNotAdd(t *testing.T) {
	t.Parallel()

	client := &fakeClient{labels: []string{"semver-major"}}
	svc := NewService(client, branchmap.NewResolver(branchmap.DefaultMapping()), labels.NewResolver(labels.Config{}))

	result, err := svc.Apply(context.Background(), Config{PRID: 9, Branch: "feature/foo"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	if result.Decision != labels.DecisionConflict {
		t.Fatalf("expected conflict decision got %v", result.Decision)
	}
	if result.LabelAdded {
		t.Fatalf("expected no label addition")
	}
}

func TestApplyValidations(t *testing.T) {
	t.Parallel()

	svc := NewService(nil, branchmap.NewResolver(branchmap.DefaultMapping()), labels.NewResolver(labels.Config{}))
	if _, err := svc.Apply(context.Background(), Config{PRID: 1, Branch: "feature/foo"}); !errors.Is(err, ErrNilClient) {
		t.Fatalf("expected ErrNilClient got %v", err)
	}

	client := &fakeClient{}
	svc = NewService(client, branchmap.NewResolver(branchmap.DefaultMapping()), labels.NewResolver(labels.Config{}))
	if _, err := svc.Apply(context.Background(), Config{PRID: 0, Branch: "feature/foo"}); !errors.Is(err, ErrInvalidPR) {
		t.Fatalf("expected ErrInvalidPR got %v", err)
	}
	if _, err := svc.Apply(context.Background(), Config{PRID: 1, Branch: ""}); !errors.Is(err, ErrEmptyBranch) {
		t.Fatalf("expected ErrEmptyBranch got %v", err)
	}
}

func TestApplyClientErrors(t *testing.T) {
	t.Parallel()

	client := &fakeClient{listErr: errors.New("boom")}
	svc := NewService(client, branchmap.NewResolver(branchmap.DefaultMapping()), labels.NewResolver(labels.Config{}))
	if _, err := svc.Apply(context.Background(), Config{PRID: 1, Branch: "feature/foo"}); err == nil {
		t.Fatalf("expected error from list")
	}

	client = &fakeClient{labels: []string{"other"}, addErr: errors.New("add-fail")}
	svc = NewService(client, branchmap.NewResolver(branchmap.DefaultMapping()), labels.NewResolver(labels.Config{}))
	if _, err := svc.Apply(context.Background(), Config{PRID: 1, Branch: "feature/foo"}); err == nil {
		t.Fatalf("expected error from add")
	}
}

type fakeClient struct {
	labels  []string
	listErr error
	addErr  error
	added   []addedCall
}

type addedCall struct {
	prID  int
	label string
}

func (f *fakeClient) ListRefsWithPrefix(context.Context, string) ([]ado.Ref, error) {
	return nil, nil
}

func (f *fakeClient) ListPRLabels(context.Context, int) ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	if len(f.labels) == 0 {
		return nil, nil
	}
	out := make([]string, len(f.labels))
	copy(out, f.labels)
	return out, nil
}

func (f *fakeClient) AddPRLabel(_ context.Context, prID int, label string) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.added = append(f.added, addedCall{prID: prID, label: label})
	return nil
}

func (f *fakeClient) FindPullRequestByMergeCommit(context.Context, string) (int, error) {
	return 0, ado.ErrPullRequestNotFound
}

func (f *fakeClient) CreateAnnotatedTag(context.Context, ado.TagSpec) error {
	return nil
}
