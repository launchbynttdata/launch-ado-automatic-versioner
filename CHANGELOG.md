# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `--token-stdin` flag to read the Azure DevOps token from standard input (up to 64 KiB, trimmed) alongside `AAV_TOKEN`, with `AAV_TOKEN` taking precedence and redacted conflict logging.
- `pr-label --use-pr-title` to derive bump intent from conventional commit PR titles (parsed via `github.com/leodido/go-conventionalcommits`).
- `pr-label --allow-branch-name-fallback` to fall back to branch-prefix bump mapping when the PR title is not a valid conventional commit.
- Azure Pipelines `##vso[task.logissue type=warning;]` output when branch-name fallback is used.
- Structured exit codes for `pr-label`: `3` for semantic validation failures (invalid PR title without fallback); `2` for ADO API failures. Other subcommands still typically exit `1`.
- Documented PR-title scope (title only, not description), `!` as the practical breaking-change marker, and rejection of unknown commit types.

### Removed

- **BREAKING:** Removed the `--token` CLI flag so tokens are never exposed in process arguments (`ps` / `/proc`). Pass credentials via the `AAV_TOKEN` environment variable or pipe them over standard input with `--token-stdin`.

### Security

- Validated and canonicalized `AAV_ORG_URL` / `--org-url` (requiring HTTPS on default port 443 with no userinfo, query, fragment, IP literals, or relative segments) and enforced trusted-host binding: inside Azure Pipelines (`SYSTEM_TEAMFOUNDATIONCOLLECTIONURI` set), the organization host must match `SYSTEM_TEAMFOUNDATIONCOLLECTIONURI` (replacing default cloud hosts); outside Azure Pipelines, only official Microsoft Azure DevOps Services cloud hosts (`dev.azure.com` and `<org>.visualstudio.com`) are accepted.
- Replaced userinfo-embedded PAT clone URLs in the integration test harness (`integration/cli_test.go`) with credential-free repository URLs and child-process-scoped `GIT_CONFIG_*` `http.extraHeader` authentication so secrets are never written to `.git/config` or child-process `argv`.

## [1.1.0] - 2025-12-16

### Added

- Floating tag management for `create-tag --tag-mode release`, including the `--use-floating-tags` / `AAV_USE_FLOATING_TAGS` switch, automatic detection of existing floating refs, and annotated tag recreation so `v<major>` pointers always track the latest release.

## [1.0.2] - 2025-12-10

### Added

- Embedded semantic version/build date metadata via ldflags, exposed through a new `aav version` command.
- Introduced a reusable `internal/cli` package plus a root-level `main.go` so the tool can be installed directly via `go install github.com/launchbynttdata/launch-ado-automatic-versioner@<version>`.

### Security

- Implemented redaction for sensitive configuration values (e.g., tokens) in logs when conflicts occur between environment variables and CLI flags.
