# Implementation Status

Last updated: 2025-12-08

## Conversation Highlights & Decisions

- **SemVer Library**: We will exclusively use `github.com/blang/semver/v4` for all version parsing, comparison, and bumping to ensure strict SemVer 2.0.0 compliance.
- **ADO Client**: A real Azure DevOps REST client is required (unless a suitable library appears). Environment variables already provide the necessary organization/project/repo/token values; the client must remain fully mockable for tests.
- **Tagging Scope**: The first iteration will focus on the automatic SemVer-driven tagging workflow (`create-tag` for `release` and `rc`). Manual tag mode can be deferred.
- **Configuration Flexibility**: Every option must ultimately support env > CLI > default precedence. Future file-based configuration should slot in easily; current structs/resolvers must be designed with that extension in mind.
- **Logging**: Use `go.uber.org/zap` with terse/verbose levels. Conflict logging format follows the spec (`config: conflict for <SETTING> …`).
- **Branch Mapping**: Provide sensible defaults today, but structure the mapping logic so injecting custom prefixes (via future config) only requires wiring new inputs rather than refactoring business logic.
- **Architecture Plan**: Three layers—business logic (pure functions), ADO client interface, CLI/adapters. Business logic is fully unit-tested with mocks for the ADO interface. Cobra will power the CLI (`pr-label`, `infer-bump`, `create-tag`).
- **Testing Expectations**: Table-driven tests for exported functions, >80% coverage target, no real HTTP calls in tests, use mocks for ADO interactions.
- **Documentation Expectations**: README must be replaced with an AAV-focused document covering overview, quick start, config reference (with precedence example), usage snippets, architecture notes, testing guidance, and contribution tips.

## Current Implementation State

| Component | Status | Notes |
|-----------|--------|-------|
| `go.mod` / dependencies | ✅ | Zap, Cobra, and blang/semver have been added; `go mod tidy` recorded transient sums (cleanup pending once packages import). |
| Logging helper (`internal/logging`) | ✅ | Zap-based factory with terse/verbose levels. Needs unit tests once broader code scaffolded. |
| Config resolver (`internal/config/resolver.go`) | ✅ | Env > CLI > default precedence helper with conflict logging. Ready to be embedded in CLI config structs. |
| Domain bump logic (`internal/domain/bump`) | ✅ | Pure helpers for parsing bumps, determining precedence, and deriving defaults. |
| Other business logic packages (branch mapping, labels, semver scanning, tag planning) | ✅ | Branch mapping + label logic done; new `tagplan` package now parses tags and computes next release/RC plans with tests. |
| Tagging service (business logic) | ✅ | `internal/services/tagging` pulls refs via the ADO client interface, feeds them into the planner, and surfaces release/RC plans with validation + tests. |
| PR labeling service (business logic) | ✅ | `internal/services/prlabel` now wires branch mapping + label resolvers through the ADO client to enforce the required PR label rules with full unit tests. |
| ADO client interface definitions | ✅ | Added `internal/ado` with the shared `Client` interface (`ListRefsWithPrefix`, `ListPRLabels`, `AddPRLabel`) plus the `Ref` model; concrete REST implementation still pending. |
| ADO client interface + REST implementation | ✅ | `internal/ado` now ships the full SDK-backed client (PAT auth, refs, PR labels, tag creation) plus helper structs and validation. |
| CLI (Cobra commands, env/flag plumbing) | ✅ | `cmd/aav` implements `pr-label`, `infer-bump`, and `create-tag` with env/flag precedence, logging, and service wiring. |
| Services for `pr-label`, `infer-bump`, `create-tag` | ✅ | `internal/services/prlabel`, `inferbump`, and `tagging` implement all workflows with coverage. |
| Tests | ✅ | Unit suites cover business logic and services (`branchmap`, `labels`, `tagplan`, `prlabel`, `inferbump`, `tagging`, etc.). Integration harness lives under `integration/` for live ADO runs. |
| README / docs overhaul | ✅ | README now documents overview, quick start, config precedence, pipeline example, architecture, testing, and contributing. |

## Next Steps

1. **Integration automation**: remaining TODOs focus on orchestration helpers for end-to-end tests (automated PR creation/merge tooling) and expanding the documented workflow for those suites.
2. **Release polish**: ensure `docs/IMPLEMENTATION_STATUS.md` stays current as integration tooling lands; consider adding CHANGELOG / versioning policy before GA.

Please update this document whenever significant decisions are made or major components land, so we have a single source of truth for project status.
