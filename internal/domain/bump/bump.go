package bump

import "fmt"

// Bump represents the semantic version increment intent.
type Bump string

const (
	BumpMajor Bump = "major"
	BumpMinor Bump = "minor"
	BumpPatch Bump = "patch"
)

// Default returns the default bump intent (patch).
func Default() Bump {
	return BumpPatch
}

// Parse converts a string into a Bump value.
func Parse(value string) (Bump, error) {
	switch Bump(value) {
	case BumpMajor, BumpMinor, BumpPatch:
		return Bump(value), nil
	default:
		return "", fmt.Errorf("invalid bump %q", value)
	}
}

// HigherImpactThan reports whether the bump is higher impact (larger) than another.
func (b Bump) HigherImpactThan(other Bump) bool {
	return weight(b) > weight(other)
}

// Max returns the highest-impact bump in the slice. Defaults to patch when empty.
func Max(values ...Bump) Bump {
	max := Default()
	for _, v := range values {
		if v.HigherImpactThan(max) {
			max = v
		}
	}
	return max
}

// String returns the textual representation. Defaults to "patch" for unknown values.
func (b Bump) String() string {
	switch b {
	case BumpMajor, BumpMinor, BumpPatch:
		return string(b)
	default:
		return string(BumpPatch)
	}
}

func weight(b Bump) int {
	switch b {
	case BumpMajor:
		return 3
	case BumpMinor:
		return 2
	case BumpPatch:
		return 1
	default:
		return 0
	}
}
