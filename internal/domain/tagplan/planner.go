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
	Name     string
	ObjectID string
}

// FloatingPlan captures detection and execution details for floating tags.
type FloatingPlan struct {
	TagName           string
	Existing          Tag
	AutoDetected      bool
	AutoDetectedMajor uint64
	Enabled           bool
	DeletedExisting   bool
	Created           bool
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
	Floating      FloatingPlan
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
		Floating:      planFloating(catalog, next),
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
	releases    []releaseEntry
	prereleases []semver.Version
	floating    []floatingEntry
}

type releaseEntry struct {
	version semver.Version
	tag     Tag
}

type floatingEntry struct {
	major uint64
	tag   Tag
}

func buildCatalog(tags []Tag) catalog {
	var c catalog
	for _, tag := range tags {
		version, ok := parseSemverTag(tag.Name)
		if !ok {
			if major, isFloating := parseFloatingTag(tag.Name); isFloating {
				c.floating = append(c.floating, floatingEntry{major: major, tag: tag})
			}
			continue
		}
		if len(version.Pre) == 0 {
			c.releases = append(c.releases, releaseEntry{version: version, tag: tag})
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

func chooseBaseRelease(releases []releaseEntry, baseOverride string) (semver.Version, BaseSource, error) {
	if len(releases) > 0 {
		highest := releases[0].version
		for _, candidate := range releases[1:] {
			if candidate.version.GT(highest) {
				highest = candidate.version
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

func planFloating(c catalog, target semver.Version) FloatingPlan {
	plan := FloatingPlan{TagName: floatingTagName(target.Major)}
	if existing, ok := c.floatingTagForMajor(target.Major); ok {
		plan.Existing = existing
	}
	if highest, ok := c.highestRelease(); ok {
		plan.AutoDetectedMajor = highest.version.Major
		plan.AutoDetected = c.hasValidFloatingForMajor(highest.version.Major)
	}
	return plan
}

func floatingTagName(major uint64) string {
	return fmt.Sprintf("v%d", major)
}

func (c catalog) floatingTagForMajor(major uint64) (Tag, bool) {
	for _, entry := range c.floating {
		if entry.major == major {
			return entry.tag, true
		}
	}
	return Tag{}, false
}

func (c catalog) highestRelease() (releaseEntry, bool) {
	if len(c.releases) == 0 {
		return releaseEntry{}, false
	}
	highest := c.releases[0]
	for _, candidate := range c.releases[1:] {
		if candidate.version.GT(highest.version) {
			highest = candidate
		}
	}
	return highest, true
}

func (c catalog) hasValidFloatingForMajor(major uint64) bool {
	for _, entry := range c.floating {
		if entry.major != major {
			continue
		}
		if entry.tag.ObjectID == "" {
			continue
		}
		for _, release := range c.releases {
			if release.version.Major != major {
				continue
			}
			if release.tag.ObjectID != "" && release.tag.ObjectID == entry.tag.ObjectID {
				return true
			}
		}
	}
	return false
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

func parseFloatingTag(name string) (uint64, bool) {
	trimmed := strings.TrimSpace(name)
	trimmed = strings.TrimPrefix(trimmed, "refs/tags/")
	if len(trimmed) <= 1 {
		return 0, false
	}
	if trimmed[0] != 'v' && trimmed[0] != 'V' {
		return 0, false
	}
	digits := trimmed[1:]
	if digits == "" {
		return 0, false
	}
	for _, ch := range digits {
		if ch < '0' || ch > '9' {
			return 0, false
		}
	}
	value, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}
