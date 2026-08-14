package pipeline

import (
	"fmt"
	"io"
	"strings"
)

// FallbackWarningPrefix is the Azure Pipelines logissue prefix for branch-name fallback warnings.
const FallbackWarningPrefix = "##vso[task.logissue type=warning;]"

// FallbackWarningMessage is the default human-readable fallback warning text.
const FallbackWarningMessage = "Semver calculation fallback to branch name"

// EmitLogIssueWarning writes an Azure Pipelines warning line to w.
func EmitLogIssueWarning(w io.Writer, message string) error {
	if w == nil {
		return fmt.Errorf("pipeline: writer is nil")
	}
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		trimmed = FallbackWarningMessage
	}
	if _, err := fmt.Fprintf(w, "%s%s\n", FallbackWarningPrefix, trimmed); err != nil {
		return fmt.Errorf("writing pipeline warning: %w", err)
	}
	return nil
}
