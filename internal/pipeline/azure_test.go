package pipeline

import (
	"bytes"
	"strings"
	"testing"
)

func TestEmitLogIssueWarning(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := EmitLogIssueWarning(&buf, FallbackWarningMessage); err != nil {
		t.Fatalf("emit: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	want := FallbackWarningPrefix + FallbackWarningMessage
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestEmitLogIssueWarningWithDetail(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	detail := FallbackWarningMessage + ": PR title invalid"
	if err := EmitLogIssueWarning(&buf, detail); err != nil {
		t.Fatalf("emit: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	want := FallbackWarningPrefix + detail
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
