# AAV – ADO Automatic Versioner
Requirements Specification

## 1. Overview

`aav` (ADO Automatic Versioner) is a Go-based CLI tool for automated semantic versioning in Azure DevOps (ADO) Git repositories and pipelines.

The tool:

- Determines version-bump intent during PR validation from branch names.
- Persists that intent as ADO PR labels.
- Rehydrates that intent on main-branch CI after a squash merge.
- Computes the **next semantic version** based on existing tags and bump intent.
- Creates annotated Git tags for RC and final releases.

All versions MUST follow **Semantic Versioning 2.0.0** as defined at:
https://semver.org/

### 1.1 SemVer Library

`aav` MUST use an existing, widely used Go SemVer 2.0.0 library and MUST NOT implement its own ad-hoc SemVer parsing/comparison/bumping.

Acceptable choices include:

- `github.com/blang/semver/v4` – explicitly states full SemVer 2.0.0 spec coverage, widely used.  [oai_citation:0‡GitHub](https://github.com/blang/semver?utm_source=chatgpt.com)
- `github.com/Masterminds/semver/v3` – active, widely used, spec-compliant, handles `v` prefix.  [oai_citation:1‡Go Packages](https://pkg.go.dev/github.com/Masterminds/semver/v3?utm_source=chatgpt.com)
- `go.followtheprocess.codes/semver` – explicitly provides parse/validate/bump and claims full SemVer 2.0.0 compliance.  [oai_citation:2‡Go Packages](https://pkg.go.dev/go.followtheprocess.codes/semver?utm_source=chatgpt.com)

The implementation SHOULD prefer one of these and MUST:

- Use the library for:
  - Parsing/validating versions.
  - Comparing versions.
  - Performing major/minor/patch bumps.
- Avoid hand-written regex or custom comparison logic.

---

## 2. High-Level Features

`aav` MUST provide at least these subcommands:

1. `pr-label` – used in PR validation.
2. `infer-bump` – used in main-branch CI post-merge.
3. `create-tag` – used to compute and create RC and final SemVer tags “on rails”.

---

## 3. Architecture & Testability

### 3.1 Layering

The codebase MUST be structured with clear separation:

- **Business Logic Layer**
  - No direct HTTP, no env lookup, no CLI parsing.
  - Pure functions operating on structs and slices.
  - Responsible for:
    - Branch → bump mapping.
    - Label name derivation.
    - Label conflict decisions.
    - Env vs CLI config resolution.
    - SemVer validation and version math decisions.
    - Determining “next version” from:
      - Existing SemVer tags.
      - Bump intent.
      - Mode (`release` vs `rc`).

- **ADO Client Interface Layer**
  - Defines an interface, e.g.:

    ```go
    type AdoClient interface {
        ListPrLabels(prID int) ([]string, error)
        AddPrLabel(prID int, label string) error
        QueryPrByMergeCommit(commitSha string) (int, error)
        ListRefsWithPrefix(prefix string) ([]Ref, error) // for tags
        CreateAnnotatedTag(tag TagSpec) error
    }
    ```

  - Concrete implementation performs actual REST calls to ADO Git APIs (pull request labels, refs, annotated tags).  [oai_citation:3‡Microsoft Learn](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/refs/list?view=azure-devops-rest-7.1&utm_source=chatgpt.com)
  - Fully mockable for testing.

- **CLI / Adapter Layer**
  - Parses CLI args.
  - Reads environment variables.
  - Applies env>CLI>default precedence.
  - Sets up logging.
  - Wires inputs to business logic + `AdoClient`.
  - Converts results to exit codes and console output.

### 3.2 Testing

- All business logic MUST have unit tests with full coverage.
- No unit test may call real ADO APIs.
- The ADO interface MUST be mockable and used in higher-level tests.
- Tests MUST cover:
  - Branch → bump mapping.
  - Label derivation and conflict rules.
  - Env vs CLI precedence and conflict logging.
  - SemVer validation and version arithmetic.
  - Tag-selection logic (latest release vs RC).
  - `create-tag` decision logic for both `release` and `rc`.

---

## 4. Semantic Versioning Requirements

### 4.1 SemVer Constraints

- All versions MUST conform to **SemVer 2.0.0**.
- A tag name is valid if:
  - It is either `vMAJOR.MINOR.PATCH` or `vMAJOR.MINOR.PATCH-PRERELEASE` (where `v` is optional).
  - Removing the optional leading `v` results in a valid SemVer 2.0.0 string.

Examples of invalid tag names:

- `v1.2`
- `v1.2.3.4`
- `v1.2.3--1`
- `release-1.2.3` (unless a future extension explicitly supports this naming).

### 4.2 Pre-release Rules

- Pre-release identifiers must obey SemVer rules:
  - Only `[0-9A-Za-z-]` characters.
  - Dot-separated (e.g. `rc.1`, `alpha`, `beta.2`).
- For RC tags produced by `aav`, the canonical form MUST be:

  ```text
  vMAJOR.MINOR.PATCH-rc.N
  ```

  where `N` is a positive integer (`1, 2, 3, ...`).

### 4.3 Version Arithmetic

When computing the “next version”:

- Start from a **valid SemVer** base version.
- For bump intent:
  - `major`: `(major+1, minor=0, patch=0)`
  - `minor`: `(minor+1, patch=0)`
  - `patch`: `(patch+1)`
- Invalid base version MUST cause a semantic error in business logic (no tag creation).

The chosen SemVer library MUST be used for parsing and bumping; `aav` must not recompute fields by hand.

---

## 5. Configuration Model

### 5.1 Precedence

Every option can be set via:

1. Environment variable (highest precedence).
2. CLI flag.
3. Default.

If env and CLI differ:

- Use env.
- Log conflict in both terse and verbose modes:
  - Example:

    ```text
    config: conflict for ORG_URL: env="https://dev.azure.com/orgA" cli="https://dev.azure.com/orgB" → using env value
    ```

### 5.2 Settings

#### ADO Connection

- `AAV_ORG_URL` / `--org-url`
- `AAV_PROJECT` / `--project`
- `AAV_REPO` / `--repo`
- `AAV_TOKEN` / `--token`

#### Logging

- `AAV_LOG_LEVEL` / `--log-level`
  - `terse` (default) or `verbose`.

#### Label Prefix & Names

- `AAV_LABEL_PREFIX` / `--label-prefix`
  - Default: `semver-`.
  - Empty string is allowed → labels: `major`, `minor`, `patch`.

Label overrides:

- `AAV_LABEL_MAJOR` / `--label-major`
- `AAV_LABEL_MINOR` / `--label-minor`
- `AAV_LABEL_PATCH` / `--label-patch`

If overrides missing, derive as `<LABEL_PREFIX>major` etc.

#### Branch-to-Bump Mapping

- `AAV_BRANCH_MAJOR_PREFIXES` / `--branch-major-prefix` (repeatable).
- `AAV_BRANCH_MINOR_PREFIXES` / `--branch-minor-prefix` (repeatable).
- `AAV_BRANCH_PATCH_PREFIXES` / `--branch-patch-prefix` (repeatable).

Rules:

- Match is prefix-based and case-sensitive.
- If multiple categories match, highest-impact wins:
  - `major > minor > patch`.
- If none match, default to `patch` (with a log message).

#### Strict Mode

- `AAV_STRICT` / `--strict` (bool, default `false`).
- For `infer-bump`:
  - No PR for commit:
    - Non-strict: warn + default to `patch`.
    - Strict: error + non-zero exit.

#### Bump Source for `create-tag`

- `AAV_BUMP` / `--bump`
  - One of `major`, `minor`, `patch`.
  - Typically provided by piping `infer-bump` output into `AAV_BUMP` env or `--bump`.
  - Required for `create-tag` automatic modes.

#### Tag Mode & Base Version

- `AAV_TAG_MODE` / `--tag-mode`
  - `release` – compute next release tag.
  - `rc` – compute next RC tag.
- `AAV_BASE_VERSION` / `--base-version` (optional)
  - Used if there are no existing release tags and we need an initial base.
  - Must be valid SemVer (e.g. `0.0.0` or `1.0.0`).

---

## 6. Subcommand Requirements

---

### 6.1 `pr-label`

#### Inputs

- `AAV_PR_ID` / `--pr-id`
- `AAV_SOURCE_BRANCH` / `--source-branch`

#### Behavior

1. Determine bump category (`major|minor|patch`) from branch mapping.
2. Resolve expected label name (using prefix/overrides).
3. Fetch labels on the PR via ADO `Pull Request Labels - List`.  [oai_citation:4‡Microsoft Learn](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-request-labels/list?view=azure-devops-rest-7.1&utm_source=chatgpt.com)
4. Identify existing “semver labels” (labels that equal any of: configured major/minor/patch labels, case-insensitive).

##### Multi-run & Conflict Rules

- **Case A: expected label present**
  - Do NOT add labels.
  - Do NOT warn.
  - Exit `0`.

- **Case B: other semver label(s) present, expected one missing**
  - Example: expected `semver-patch`; PR has `semver-minor`.
  - Do NOT remove labels.
  - Do NOT add new semver labels.
  - Log a loud warning (in both modes) describing:
    - Expected label.
    - Existing semver labels.
    - Statement that no changes are made.
  - Exit `0`.

- **Case C: no semver labels present**
  - Add expected label.
  - Exit `0`.

These rules MUST be implemented in business logic and fully unit-tested.

---

### 6.2 `infer-bump`

#### Inputs

- `AAV_COMMIT_SHA` / `--commit-sha`

#### Behavior

1. Use ADO Git PR query to find PR whose merge commit equals `AAV_COMMIT_SHA`.  [oai_citation:5‡Microsoft Learn](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/?view=azure-devops-rest-7.1&utm_source=chatgpt.com)
2. If none found:
   - Non-strict: warn, default bump to `patch`.
   - Strict: log error, exit non-zero.
3. If a PR is found:
   - Fetch PR labels.
   - Determine bump from semver labels:
     - If multiple bump labels: highest-impact wins.
     - If none: default `patch` and log defaulting.
4. Output bump to stdout as a single word: `major`, `minor`, or `patch`.
5. Exit `0` on success.

---

### 6.3 `create-tag` (On-Rails SemVer Mode)

This subcommand MUST support **automatic SemVer-based tagging**, not just arbitrary tag creation.

#### Inputs

- `AAV_TAG_MODE` / `--tag-mode` (required)
  - `release` or `rc`.
- `AAV_BUMP` / `--bump` (required in automatic mode)
  - `major`, `minor`, or `patch`.
- `AAV_BASE_VERSION` / `--base-version` (optional; used when no releases exist).
- Common tag metadata:
  - `AAV_TAG_MESSAGE` / `--tag-message`
  - `AAV_TAGGER_NAME` / `--tagger-name` (default `"aav"`)
  - `AAV_TAGGER_EMAIL` / `--tagger-email` (default `"aav@example"`)

##### Optional Manual Mode

- `AAV_TAG_NAME` / `--tag-name` (explicit tag name)
  - If provided with `--manual` (or similar explicit flag), `create-tag` MAY skip version math but MUST still validate the tag name as SemVer (after stripping optional leading `v`).
  - If `--manual` is not set, then `create-tag` MUST ignore `--tag-name` and operate purely in automatic mode.

#### 6.3.1 Common Tag Discovery Logic

In automatic mode (`release` or `rc`):

1. Use ADO `Refs - List` to fetch all refs with prefix `refs/tags/`.  [oai_citation:6‡Microsoft Learn](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/refs/list?view=azure-devops-rest-7.1&utm_source=chatgpt.com)
2. For each tag ref:
   - Strip `refs/tags/`.
   - Optionally strip leading `v`.
   - Parse with the chosen SemVer library.
   - Skip invalid SemVer tags.
3. Partition parsed tags into:
   - **Release tags** – versions with no prerelease.
   - **Pre-release tags** – versions with prerelease (e.g. `rc.1`, `alpha`).

All subsequent logic MUST only use these parsed, validated SemVer versions.

#### 6.3.2 `tag-mode=release` Behavior

Goal: Compute and create the **next stable release version**, ignoring RCs when determining the “current highest release”.

1. Determine **current highest release version**:
   - Among release tags (no prerelease), choose max by SemVer comparison.
   - If no release tags exist:
     - If `AAV_BASE_VERSION` is provided:
       - Use `AAV_BASE_VERSION` as base.
     - Else:
       - Default base to `0.0.0`.
       - Log that default base was used.

2. Apply bump:
   - Use `AAV_BUMP` (`major|minor|patch`) and the SemVer library’s bump helpers.
   - Result is the **next release version** `Vnext`.

3. Construct tag name:
   - `v` + `Vnext` (e.g. `v1.2.3`).

4. Create annotated tag:
   - Use ADO `Annotated Tags - Create`.  [oai_citation:7‡Microsoft Learn](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/annotated-tags/create?view=azure-devops-rest-7.1&utm_source=chatgpt.com)
   - Target commit: supplied by pipeline (e.g. `$(Build.SourceVersion)`), passed into `create-tag` via `AAV_COMMIT_SHA` / `--commit-sha`.

5. Logging:
   - Terse: one line summarizing `mode=release`, bump, base, resulting version, and commit.
   - Verbose: details on:
     - All candidate release tags.
     - Chosen base version.
     - Bump step.
     - Tag creation response (no secrets).

6. Exit codes:
   - `0` on success.
   - Non-zero on config/semantic/ADO errors.

#### 6.3.3 `tag-mode=rc` Behavior

Goal: Compute and create the **next RC** for the next release version implied by `AAV_BUMP`, ignoring existing RC tags when determining the base release.

1. Determine **current highest release version** as in `release` mode.
2. Compute **target release version** `VreleaseNext`:
   - Apply bump intent (`AAV_BUMP`) to the current highest release (`Vbase`).
3. Among all **pre-release tags**, find those whose **base version** equals `VreleaseNext` and whose prerelease begins with `rc.` followed by a positive integer.
   - Examples:
     - `v1.2.3-rc.1`
     - `v1.2.3-rc.2`
   - Ignore any other prerelease tags (`alpha`, `beta`, etc.) for RC sequence numbering.
4. Determine next RC number `N`:
   - If no existing `rc.N` tags for this base:
     - `N = 1`.
   - Else:
     - `N = max(existing N) + 1`.
5. Construct RC version:
   - `Vrc = VreleaseNext` with prerelease `rc.N`.
   - Example: `1.2.3-rc.2`.
6. Construct tag name:
   - `v` + `Vrc` (e.g. `v1.2.3-rc.2`).
7. Create annotated tag via ADO.
8. Logging:
   - Terse: one line summarizing `mode=rc`, `VreleaseNext`, `rc.N`, and commit.
   - Verbose: details on:
     - Existing releases and RCs.
     - How `VreleaseNext` was computed.
     - How `N` was chosen.

9. Exit codes:
   - `0` on success.
   - Non-zero on config/semantic/ADO errors.

#### 6.3.4 Error Cases for `create-tag`

- If `AAV_BUMP` is missing or invalid in automatic mode → config error.
- If base version or discovered tags produce an invalid SemVer or inconsistent state → semantic error.
- If tag creation fails due to ADO (e.g. permissions, conflict) → ADO error.

---

## 7. Logging Requirements

### 7.1 Modes

- `terse`:
  - Minimal action logs.
  - Warnings and errors.
- `verbose`:
  - Fully resolved configuration.
  - Env vs CLI conflicts.
  - Tag discovery details.
  - Decision traces.

### 7.2 Conflict Logging

Env vs CLI conflicts MUST always be logged as:

```text
config: conflict for <SETTING>: env="<x>" cli="<y>" → using env value
```

### 7.3 Redaction

No tokens or secrets in logs.

---

## 8. Output Format

### 8.1 `infer-bump`

- Terse mode:
  - stdout: exactly `major`, `minor`, or `patch`.
- Verbose mode:
  - Additional logs should go to stderr or be clearly separated; final line of stdout SHOULD still be the bump.

### 8.2 `pr-label` and `create-tag`

- No strict machine-readable contract beyond exit code.
- Human-readable logs sufficient.

---

## 9. Error Handling & Exit Codes

Recommended mapping:

- `0` – success.
- `1` – configuration error.
- `2` – ADO API error.
- `3` – semantic error (e.g. invalid SemVer state).
- Additional codes allowed but must be documented in code.

Every non-zero exit MUST include:

- One-line terse summary.
- Extra detail in verbose mode.

---

## 10. Implementation Notes

- Language: Go (latest stable).
- Single binary (`aav`) if practical.
- Use the chosen SemVer library for all version parsing/comparison/bumping – no custom logic.
- ADO REST APIs:
  - Use **Git** APIs for PR labels, refs (tags), and annotated tags.  [oai_citation:8‡Microsoft Learn](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/annotated-tags/create?view=azure-devops-rest-7.1&utm_source=chatgpt.com)
  - Authenticate via PAT or `System.AccessToken`.

---

## 11. Documentation Requirements (README.md)

A `README.md` in the repo root MUST include:

### 11.1 Overview

- What `aav` is.
- How it fits into ADO PR validation and main-branch pipelines.

### 11.2 Quick Start

- Build instructions:
  - `go build ./cmd/aav`
- Minimal examples:
  - `aav pr-label` in PR validation.
  - `aav infer-bump` in main pipeline.
  - `aav create-tag` in both `release` and `rc` modes.

### 11.3 Configuration Reference

- Table of env vars and CLI flags.
- Precedence rule (env > CLI > default).
- Example showing a conflict and log output.

### 11.4 Usage Examples

- ADO pipeline YAML snippets:
  - PR pipeline calling `pr-label`.
  - Main pipeline calling `infer-bump` and then `create-tag` (`release` mode).
  - RC tag creation (`rc` mode) tied to PR validation or special RC pipeline.

### 11.5 Internal Architecture

- Short explanation of:
  - Business logic packages.
  - ADO client interface.
  - CLI entrypoint/subcommands.
- How the layered design enables mocking and unit testing.

### 11.6 Testing

- How to run tests: `go test ./...`
- Statement that:
  - Business logic is fully unit-tested.
  - No real ADO API calls are made in tests.
- Explanation of how ADO client mocks are used.

### 11.7 Contributing

- Coding style notes (if any).
- Requirement that new business logic includes tests.
- Guidance for adding new subcommands or configuration in line with existing patterns.

---

This specification defines `aav`’s behavior, architecture, SemVer constraints, “on-rails” tag creation, testing expectations, and documentation requirements. It is intended to be directly consumable by an automated code-generation agent implementing the tool in Go.
