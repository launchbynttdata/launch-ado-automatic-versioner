package tagplan

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	semver "github.com/blang/semver/v4"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/bump"
)

// Mode represents the tag creation mode supported by the planner.
type Mode string

const (
	// ModeRelease computes the next stable release tag.
	ModeRelease Mode = "release"
	// ModeRC computes the next release-candidate tag.
	ModeRC Mode = "rc"
)

// BaseSource describes where the base version originated.
type BaseSource string

const (
	// BaseSourceExisting indicates the base came from the highest existing release.
	BaseSourceExisting BaseSource = "existing-release"
	// BaseSourceConfigured indicates the base used the provided base-version input.
	BaseSourceConfigured BaseSource = "configured-base"
	// BaseSourceZero indicates the planner fell back to 0.0.0.
	BaseSourceZero BaseSource = "default-zero"
)

// Tag represents a Git tag reference.
type Tag struct {
	Name string
}

// Planner computes release and RC tagging plans from a set of tags.
type Planner struct {
	tagPrefix string
}

// NewPlanner creates a Planner instance with the provided prefix (trimmed) applied to tag names.
func NewPlanner(prefix string) Planner {
	return Planner{tagPrefix: strings.TrimSpace(prefix)}
}

// Result captures the outcome of planning a tag creation operation.
type Result struct {
	Mode          Mode
	TagName       string
	Version       semver.Version
	ReleaseBase   semver.Version
	BaseSource    BaseSource
	TargetRelease semver.Version
	RCNumber      int
}

// PlanRelease determines the next release tag using the provided bump intent.
func (p Planner) PlanRelease(tags []Tag, intent bump.Bump, baseOverride string) (Result, error) {
	catalog := buildCatalog(tags)

	base, source, err := chooseBaseRelease(catalog.releases, baseOverride)
	if err != nil {
		return Result{}, err
	}

	next, err := bumpVersion(base, intent)
	if err != nil {
		return Result{}, fmt.Errorf("computing release bump: %w", err)
	}

	return Result{
		Mode:          ModeRelease,
		TagName:       p.formatTagName(next),
		Version:       next,
		ReleaseBase:   base,
		BaseSource:    source,
		TargetRelease: next,
	}, nil
}

// PlanRC determines the next RC tag for the upcoming release implied by the bump intent.
func (p Planner) PlanRC(tags []Tag, intent bump.Bump, baseOverride string) (Result, error) {
	catalog := buildCatalog(tags)

	base, source, err := chooseBaseRelease(catalog.releases, baseOverride)
	if err != nil {
		return Result{}, err
	}

	target, err := bumpVersion(base, intent)
	if err != nil {
		return Result{}, fmt.Errorf("computing release bump: %w", err)
	}

	rcNumber := nextRCNumber(target, catalog.prereleases)

	rcVersion, err := attachRC(target, rcNumber)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Mode:          ModeRC,
		TagName:       p.formatTagName(rcVersion),
		Version:       rcVersion,
		ReleaseBase:   base,
		BaseSource:    source,
		TargetRelease: target,
		RCNumber:      rcNumber,
	}, nil
}

type catalog struct {
	releases    []semver.Version
	prereleases []semver.Version
}

func buildCatalog(tags []Tag) catalog {
	var c catalog
	for _, tag := range tags {
		version, ok := parseSemverTag(tag.Name)
		if !ok {
			continue
		}
		if len(version.Pre) == 0 {
			c.releases = append(c.releases, version)
			continue
		}
		c.prereleases = append(c.prereleases, version)
	}
	return c
}

func parseSemverTag(name string) (semver.Version, bool) {
	normalized := strings.TrimSpace(name)
	normalized = strings.TrimPrefix(normalized, "refs/tags/")
	if normalized == "" {
		return semver.Version{}, false
	}

	if version, err := semver.Parse(normalized); err == nil {
		return version, true
	}

	if len(normalized) > 1 && (normalized[0] == 'v' || normalized[0] == 'V') {
		if version, err := semver.Parse(normalized[1:]); err == nil {
			return version, true
		}
	}

	return semver.Version{}, false
}

func parseVersionString(input string) (semver.Version, error) {
	trimmed := strings.TrimSpace(input)
	trimmed = strings.TrimPrefix(trimmed, "refs/tags/")
	if trimmed == "" {
		return semver.Version{}, fmt.Errorf("base version is empty")
	}

	if version, err := semver.Parse(trimmed); err == nil {
		return version, nil
	}

	if len(trimmed) > 1 && (trimmed[0] == 'v' || trimmed[0] == 'V') {
		return semver.Parse(trimmed[1:])
	}

	return semver.Version{}, fmt.Errorf("invalid semver %q", input)
}

func chooseBaseRelease(releases []semver.Version, baseOverride string) (semver.Version, BaseSource, error) {
	if len(releases) > 0 {
		highest := releases[0]
		for _, candidate := range releases[1:] {
			if candidate.GT(highest) {
				highest = candidate
			}
		}
		return highest, BaseSourceExisting, nil
	}

	if strings.TrimSpace(baseOverride) != "" {
		version, err := parseVersionString(baseOverride)
		if err != nil {
			return semver.Version{}, "", fmt.Errorf("invalid base version: %w", err)
		}
		return version, BaseSourceConfigured, nil
	}

	zero, _ := semver.Parse("0.0.0")
	return zero, BaseSourceZero, nil
}

func bumpVersion(base semver.Version, intent bump.Bump) (semver.Version, error) {
	next := base
	var err error
	switch intent {
	case bump.BumpMajor:
		err = next.IncrementMajor()
	case bump.BumpMinor:
		err = next.IncrementMinor()
	default:
		err = next.IncrementPatch()
	}
	if err != nil {
		return semver.Version{}, err
	}
	next.Pre = nil
	next.Build = nil
	return next, nil
}

func (p Planner) formatTagName(version semver.Version) string {
	prefix := strings.TrimSpace(p.tagPrefix)
	return prefix + version.String()
}

func nextRCNumber(target semver.Version, prereleases []semver.Version) int {
	max := 0
	for _, version := range prereleases {
		if !sameBase(version, target) {
			continue
		}
		number, ok := rcNumber(version)
		if !ok {
			continue
		}
		if number > max {
			max = number
		}
	}
	return max + 1
}

func sameBase(left, right semver.Version) bool {
	return left.Major == right.Major && left.Minor == right.Minor && left.Patch == right.Patch
}

func rcNumber(version semver.Version) (int, bool) {
	if len(version.Pre) != 2 {
		return 0, false
	}

	first := version.Pre[0]
	second := version.Pre[1]

	if first.IsNum {
		return 0, false
	}
	if !strings.EqualFold(first.VersionStr, "rc") {
		return 0, false
	}
	if !second.IsNum {
		return 0, false
	}
	if second.VersionNum == 0 {
		return 0, false
	}
	if second.VersionNum > math.MaxInt64 {
		return 0, false
	}

	return int(second.VersionNum), true
}

func attachRC(target semver.Version, rc int) (semver.Version, error) {
	if rc <= 0 {
		return semver.Version{}, fmt.Errorf("invalid rc number %d", rc)
	}

	base := target

	rcLabel, err := semver.NewPRVersion("rc")
	if err != nil {
		return semver.Version{}, fmt.Errorf("building rc label: %w", err)
	}

	numberLabel, err := semver.NewPRVersion(strconv.Itoa(rc))
	if err != nil {
		return semver.Version{}, fmt.Errorf("building rc number: %w", err)
	}

	base.Pre = []semver.PRVersion{rcLabel, numberLabel}
	base.Build = nil

	return base, nil
}
