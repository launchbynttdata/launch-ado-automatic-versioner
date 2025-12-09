package version

// Package version exposes build-time metadata stamped into the binary via ldflags.

const (
	defaultVersion   = "dev"
	defaultBuildDate = "unknown"
)

var (
	// Version is the semantic version associated with this build.
	Version = defaultVersion
	// BuildDate is the UTC timestamp when the binary was built.
	BuildDate = defaultBuildDate
)

// Summary returns a human-readable description of the build metadata.
func Summary() string {
	return Version + " (built " + BuildDate + ")"
}
