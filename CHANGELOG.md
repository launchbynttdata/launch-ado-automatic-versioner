# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Embedded semantic version/build date metadata via ldflags, exposed through a new `aav version` command.
- Introduced a reusable `internal/cli` package plus a root-level `main.go` so the tool can be installed directly via `go install github.com/launchbynttdata/launch-ado-automatic-versioner@<version>`.

### Security

- Implemented redaction for sensitive configuration values (e.g., tokens) in logs when conflicts occur between environment variables and CLI flags.
