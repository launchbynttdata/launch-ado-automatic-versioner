package version

import "testing"

func TestSummaryUsesCurrentValues(t *testing.T) {
	oldVersion := Version
	oldDate := BuildDate
	t.Cleanup(func() {
		Version = oldVersion
		BuildDate = oldDate
	})

	Version = "v1.2.3"
	BuildDate = "2025-01-02T03:04:05Z"

	summary := Summary()

	if summary != "v1.2.3 (built 2025-01-02T03:04:05Z)" {
		t.Fatalf("unexpected summary: %s", summary)
	}
}
