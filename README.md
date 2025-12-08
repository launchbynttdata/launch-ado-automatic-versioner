# AAV – ADO Automatic Versioner

`aav` is a Go-based CLI that keeps Azure DevOps (ADO) repositories on semantic-versioning rails. It reads bump intent from branch names and pull-request labels, propagates that intent through merge validation, and creates annotated SemVer release or RC tags directly in ADO Git.

## Key Features

- Automatic branch → bump mapping with conflict-safe PR labeling
- PR merge inference that survives squash merges and defaults safely when strict mode is off
- Release and RC tag planning using validated SemVer math (powered by `github.com/blang/semver/v4`)
- Annotated tag creation through the official Azure DevOps SDK with tagger metadata and commit targeting
- Env > flag > default configuration precedence with structured logging (Zap)
- Pure business-logic layer with exhaustive unit tests and Azure-friendly integration tests

## Quick Start

```bash
# Build the CLI
GOOS=$(go env GOOS) GOARCH=$(go env GOARCH) go build -o ./bin/aav ./cmd/aav

# Add semver labels to a PR during validation
AAV_ORG_URL=... AAV_PROJECT=... AAV_REPO=... AAV_TOKEN=... \
AAV_PR_ID=1234 AAV_SOURCE_BRANCH=feature/awesome-fix \
./bin/aav pr-label

# Infer the bump after a squash merge (strict mode errors when no PR is found)
AAV_COMMIT_SHA=$(git rev-parse HEAD) AAV_STRICT=true ./bin/aav infer-bump

# Create a release tag for the merge commit (bump provided via infer-bump output)
AAV_BUMP=minor AAV_TAG_MODE=release AAV_COMMIT_SHA=$(git rev-parse HEAD) \
AAV_TAGGER_NAME="Build Bot" AAV_TAGGER_EMAIL=ci@example.com ./bin/aav create-tag
```

### Running in Azure Pipelines

- **PR validation**: call `aav pr-label` after checkout and before policy evaluation.
- **Main pipeline**: run `aav infer-bump --commit-sha $(Build.SourceVersion)` to capture intent, export its stdout, then invoke `aav create-tag` (release or RC mode) with that bump.

```yaml
# azure-pipelines.yml
trigger:
  branches:
    include:
      - main

pr:
  branches:
    include:
      - '*'

variables:
  AAV_ORG_URL: $(System.TeamFoundationCollectionUri)
  AAV_PROJECT: $(System.TeamProject)
  AAV_REPO: $(Build.Repository.Name)
  AAV_LABEL_PREFIX: semver-

stages:
  - stage: PRValidation
    condition: eq(variables['Build.Reason'], 'PullRequest')
    jobs:
      - job: LabelPR
        steps:
          - checkout: self
          - script: |
              go run ./cmd/aav pr-label \
                --pr-id $(System.PullRequest.PullRequestId) \
                --source-branch $(System.PullRequest.SourceBranch)
            displayName: Apply semver label

  - stage: MainRelease
    condition: and(succeeded(), eq(variables['Build.SourceBranch'], 'refs/heads/main'))
    dependsOn: PRValidation
    jobs:
      - job: TagRelease
        steps:
          - checkout: self
          - script: |
              BUMP=$(go run ./cmd/aav infer-bump \
                --commit-sha $(Build.SourceVersion))
              echo "##vso[task.setvariable variable=AAV_BUMP]$BUMP"
            displayName: Infer bump
          - script: |
              go run ./cmd/aav create-tag \
                --commit-sha $(Build.SourceVersion) \
                --tag-mode release \
                --bump $(AAV_BUMP)
            displayName: Create release tag
```

## Configuration Reference

| Purpose | Environment Variable | Flag | Default | Notes |
| --- | --- | --- | --- | --- |
| Org URL | `AAV_ORG_URL` | `--org-url` | _required_ | `https://dev.azure.com/{org}` |
| Project | `AAV_PROJECT` | `--project` | _required_ | ADO project name |
| Repository | `AAV_REPO` | `--repo` | _required_ | Git repo name |
| Token | `AAV_TOKEN` | `--token` | _required_ | PAT or `System.AccessToken` |
| Log level | `AAV_LOG_LEVEL` | `--log-level` | `terse` | `verbose` prints config + trace |
| Label prefix | `AAV_LABEL_PREFIX` | `--label-prefix` | `semver-` | Empty string allowed |
| Major label | `AAV_LABEL_MAJOR` | `--label-major` | derived | Overrides prefix value |
| Minor label | `AAV_LABEL_MINOR` | `--label-minor` | derived | Overrides prefix value |
| Patch label | `AAV_LABEL_PATCH` | `--label-patch` | derived | Overrides prefix value |
| Major branch prefixes | `AAV_BRANCH_MAJOR_PREFIXES` | `--branch-major-prefix` | `breaking/,major/` | Repeatable flag; env uses comma-separated list (e.g. `breaking/,major/`) |
| Minor branch prefixes | `AAV_BRANCH_MINOR_PREFIXES` | `--branch-minor-prefix` | `feature/,minor/` | Repeatable flag; env uses comma-separated list (e.g. `feature/,minor/`) |
| Patch branch prefixes | `AAV_BRANCH_PATCH_PREFIXES` | `--branch-patch-prefix` | `bugfix/,fix/,hotfix/,chore/,patch/` | Repeatable flag; env uses comma-separated list (e.g. `bugfix/,fix/`) |
| PR ID | `AAV_PR_ID` | `--pr-id` | _required by pr-label_ | Integer > 0 |
| Source branch | `AAV_SOURCE_BRANCH` | `--source-branch` | _required by pr-label_ | Branch that triggered PR |
| Commit SHA | `AAV_COMMIT_SHA` | `--commit-sha` | _required by infer-bump/create-tag_ | 40-char SHA |
| Strict mode | `AAV_STRICT` | `--strict` | `false` | Only applies to `infer-bump` |
| Tag mode | `AAV_TAG_MODE` | `--tag-mode` | _required by create-tag_ | `release` or `rc` |
| Bump intent | `AAV_BUMP` | `--bump` | _required by create-tag_ | `major`, `minor`, `patch` |
| Base version | `AAV_BASE_VERSION` | `--base-version` | none | Used when no stable tags exist |
| Tag message | `AAV_TAG_MESSAGE` | `--tag-message` | empty | Stored in annotated tag |
| Tagger name | `AAV_TAGGER_NAME` | `--tagger-name` | `aav` | Recorded in annotated tag |
| Tagger email | `AAV_TAGGER_EMAIL` | `--tagger-email` | `aav@example.com` | Recorded in annotated tag |
| Tag prefix | `AAV_TAG_PREFIX` | `--tag-prefix` | empty | Prepended to computed tag names (set to `v` for legacy repos) |

> **Precedence**: environment variables always win over explicit flags; conflicts are logged in both terse and verbose modes.

> **Branch prefix env format**: When using the environment variables above, provide comma-separated prefixes with no quotes (e.g. `AAV_BRANCH_MINOR_PREFIXES=feature/,minor/`). Use the repeatable CLI flags when you prefer to specify each prefix individually.

## Subcommands

| Command | When to use | Behavior |
| --- | --- | --- |
| `pr-label` | Pull-request validation | Resolves bump intent from the source branch, ensures the expected semver label exists, loudly warns on conflicts, and never removes user labels. |
| `infer-bump` | Main-branch CI after squash merge | Locates the PR by merge commit, rehydrates bump intent from labels, defaults to `patch` unless `--strict` is set. Prints `major`, `minor`, or `patch` to stdout for scripting. |
| `create-tag` | Release/RC tagging stages | Discovers existing tags, computes the next SemVer (release or RC), and creates an annotated tag on the desired commit with full trace logging. |

## Architecture

- **Business logic**: pure Go packages for branch mapping, label resolution, SemVer planning, and configuration. These packages do not perform I/O and are fully unit tested.
- **ADO client**: a thin wrapper over `github.com/microsoft/azure-devops-go-api/azuredevops/v7` that exposes the handful of Git operations we need (PR labels, ref listing, annotated tags). Substitutable via interfaces for tests.
- **CLI layer**: Cobra commands, env/flag resolution, and Zap logging that wire user intent into the business layer.

## Testing

| Scope | Command | Notes |
| --- | --- | --- |
| Unit tests | `go test ./...` | Covers business logic and service layers. Executed in CI. |
| Integration tests | `go test -tags=integration ./integration -count=1` | Hits live ADO resources using the `AAV_` environment variables described below. Not run by default or in CI. |

### Integration Test Environment

Set these variables to point at a safe ADO test repository before running with the `integration` build tag:

| Variable | Purpose |
| --- | --- |
| `AAV_ORG_URL` | Azure DevOps organization URL that hosts the repo under test |
| `AAV_PROJECT` | Project name containing that repository |
| `AAV_REPO` | Repository name used for temporary branches/tags |
| `AAV_TOKEN` | PAT or `System.AccessToken` with PR + tag permissions |
| `AAV_EXPECTED_BUMP` (optional) | Override the bump asserted by the workflow (`major`, `minor`, or `patch`); defaults to `minor` by using a `feature/` branch |
| `AAV_TARGET_BRANCH` (optional) | Base branch to branch from; defaults to `main` |
| `AAV_MANUAL_MERGE` (optional) | Set to `true` to pause so you can merge the PR manually |
| `AAV_GIT_AUTHOR_NAME` / `AAV_GIT_AUTHOR_EMAIL` (optional) | Commit identity for the temporary feature branch |
| `AAV_TAGGER_NAME` / `AAV_TAGGER_EMAIL` (optional) | Annotated tag identity overrides |
| `AAV_TAG_PREFIX` (optional) | Prepends text to created tags (set to `v` when matching legacy repos) |
| `AAV_BAD_COMMIT_SHA` (optional) | Commit that should not map to a PR; defaults to all zeros |
| `AAV_BAD_PR_ID` (optional) | Nonexistent PR ID for negative testing |
| `AAV_BRANCH_MAJOR_PREFIXES` / `AAV_BRANCH_MINOR_PREFIXES` / `AAV_BRANCH_PATCH_PREFIXES` (optional) | Override the branch-to-bump mapping the tests and CLI use; each env expects a comma-separated list (mirrors the CLI defaults) |

The tests call `go run ./cmd/aav ...` so they verify the built binary end-to-end. When the required `AAV_` variables are absent the tests automatically skip.

## Contributing

- Go 1.23+ is required; keep Go modules tidy and run `go test ./...` before submitting changes.
- Follow the layered architecture: keep business logic separate from Azure SDK calls and CLI plumbing.
- Add unit tests alongside any new exported behavior. Mock the ADO client for service tests.
- Use `golangci-lint run --timeout=10m` and `make ci-local` to mirror CI locally.
- For release engineering, see `RELEASE_GUIDE.md` and `RELEASE_*` Make targets.

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.

- Build date
- Built by (goreleaser)

## Configuration Files

- `.goreleaser.yml` - GoReleaser configuration for builds and releases
- `.github/workflows/ci.yml` - CI pipeline (tests, linting, security)
- `.github/workflows/release.yml` - Release pipeline
- `.golangci.yml` - Linter configuration
- `.github/dependabot.yml` - Dependency update configuration
- `Dockerfile` - Multi-stage Docker build configuration
- `docker-compose.yml` - Local development environment
- `.pre-commit-config.yaml` - Pre-commit hooks configuration
- `.vscode/settings.json` - VS Code Go development settings
- `.vscode/extensions.json` - Recommended VS Code extensions
- `.env.example` - Environment variables template

## Development

This project includes several CI checks:

- **Tests**: Unit tests with race detection and coverage
- **Linting**: golangci-lint with multiple linters enabled
- **Security**: Gosec security scanner and govulncheck
- **Builds**: Cross-platform build verification
- **Dependencies**: Go mod tidy verification

All checks must pass before merging to main.

### Project Structure

```text
ai-code-template-go/
├── cmd/                    # Application entry points
│   └── server/            # HTTP server application
├── internal/               # Private application code
│   ├── config/            # Configuration management
│   ├── handlers/          # HTTP request handlers
│   └── models/            # Data models
├── pkg/                    # Public libraries
├── api/                    # API definitions
├── docs/                   # Documentation
├── scripts/                # Build and deployment scripts
├── examples/               # Usage examples
└── .env.example           # Environment variables template
```

### Docker Development

```bash
# Build and run with Docker
make docker-build
make docker-run

# Or use Docker Compose for local development
make docker-compose-up
make docker-compose-down
```

### Pre-commit Hooks

This project includes pre-commit hooks for code quality:

```bash
# Install pre-commit hooks
pre-commit install

# Run manually
pre-commit run --all-files

```
