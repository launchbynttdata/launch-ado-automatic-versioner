# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `pr-label --use-pr-title` to derive bump intent from conventional commit PR titles (parsed via `github.com/leodido/go-conventionalcommits`).
- `pr-label --allow-branch-name-fallback` to fall back to branch-prefix bump mapping when the PR title is not a valid conventional commit.
- Azure Pipelines `##vso[task.logissue type=warning;]` output when branch-name fallback is used.
- Structured exit codes for `pr-label`: `3` for semantic validation failures (invalid PR title without fallback); `2` for ADO API failures. Other subcommands still typically exit `1`.
- Documented PR-title scope (title only, not description), `!` as the practical breaking-change marker, and rejection of unknown commit types.

## [1.1.0] - 2025-12-16

### Added

- Floating tag management for `create-tag --tag-mode release`, including the `--use-floating-tags` / `AAV_USE_FLOATING_TAGS` switch, automatic detection of existing floating refs, and annotated tag recreation so `v<major>` pointers always track the latest release.

## [1.0.2] - 2025-12-10

### Added

- Embedded semantic version/build date metadata via ldflags, exposed through a new `aav version` command.
- Introduced a reusable `internal/cli` package plus a root-level `main.go` so the tool can be installed directly via `go install github.com/launchbynttdata/launch-ado-automatic-versioner@<version>`.

### Security

- Implemented redaction for sensitive configuration values (e.g., tokens) in logs when conflicts occur between environment variables and CLI flags.
