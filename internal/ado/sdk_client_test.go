package ado

import (
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
)

func TestConvertGitRefs(t *testing.T) {
	t.Parallel()

	t.Run("annotated tag preserves ref and peeled object ids", func(t *testing.T) {
		t.Parallel()
		name := "refs/tags/v1"
		tagObject := "tag-object"
		commit := "commit-object"
		refs := convertGitRefs([]git.GitRef{{
			Name:           &name,
			ObjectId:       &tagObject,
			PeeledObjectId: &commit,
		}})
		if len(refs) != 1 {
			t.Fatalf("expected 1 ref, got %d", len(refs))
		}
		if refs[0].Name != name || refs[0].ObjectID != tagObject || refs[0].PeeledObjectID != commit {
			t.Fatalf("unexpected ref conversion: %+v", refs[0])
		}
	})

	t.Run("lightweight tag leaves peeled object id empty", func(t *testing.T) {
		t.Parallel()
		name := "refs/tags/v1-lw"
		commit := "commit-only"
		refs := convertGitRefs([]git.GitRef{{
			Name:     &name,
			ObjectId: &commit,
		}})
		if len(refs) != 1 {
			t.Fatalf("expected 1 ref, got %d", len(refs))
		}
		if refs[0].Name != name || refs[0].ObjectID != commit || refs[0].PeeledObjectID != "" {
			t.Fatalf("unexpected ref conversion: %+v", refs[0])
		}
	})
}

func TestErrIfRefDeleteUpdateRejected(t *testing.T) {
	t.Parallel()
	const ref = "refs/tags/floating"

	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name    string
		results *[]git.GitRefUpdateResult
		wantErr bool
	}{
		{
			name:    "nil results",
			results: nil,
			wantErr: true,
		},
		{
			name:    "empty slice",
			results: &[]git.GitRefUpdateResult{},
			wantErr: true,
		},
		{
			name: "two results",
			results: &[]git.GitRefUpdateResult{
				{Success: boolPtr(true)},
				{Success: boolPtr(true)},
			},
			wantErr: true,
		},
		{
			name: "success false stale old object id",
			results: &[]git.GitRefUpdateResult{
				{Success: boolPtr(false)},
			},
			wantErr: true,
		},
		{
			name: "success nil",
			results: &[]git.GitRefUpdateResult{
				{Success: nil},
			},
			wantErr: true,
		},
		{
			name: "single success true",
			results: &[]git.GitRefUpdateResult{
				{Success: boolPtr(true)},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := errIfRefDeleteUpdateRejected(tt.results, ref)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				const wantSub = "deleting ref refs/tags/floating rejected"
				if err.Error() != wantSub {
					t.Fatalf("error %q, want %q", err.Error(), wantSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
