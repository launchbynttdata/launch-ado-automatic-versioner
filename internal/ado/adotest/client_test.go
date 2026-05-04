package adotest

import (
	"context"
	"testing"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/ado"
)

func TestDeleteRefRequiresCurrentRefObjectID(t *testing.T) {
	t.Parallel()

	client := NewClient()
	client.SeedAnnotatedTag("v1", "tag-object", "commit-object")

	if err := client.DeleteRef(context.Background(), "refs/tags/v1", "commit-object"); err == nil {
		t.Fatalf("expected delete with peeled commit id to fail")
	}

	if err := client.DeleteRef(context.Background(), "refs/tags/v1", "tag-object"); err != nil {
		t.Fatalf("delete with ref object id: %v", err)
	}
}

func TestCreateAnnotatedTagFailsWhenRefExists(t *testing.T) {
	t.Parallel()

	client := NewClient()
	spec := ado.TagSpec{Name: "v1", ObjectID: "commit-a"}

	if err := client.CreateAnnotatedTag(context.Background(), spec); err != nil {
		t.Fatalf("create annotated tag: %v", err)
	}
	if err := client.CreateAnnotatedTag(context.Background(), spec); err == nil {
		t.Fatalf("expected duplicate ref creation to fail")
	}
}
