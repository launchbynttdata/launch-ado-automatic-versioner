## Description

<!-- Describe your changes and why they are needed -->

## Checklist

- [ ] Go 1.26+ used; `make deps` and `make ci-local` pass locally
- [ ] Go modules tidy (`go mod tidy` run; no unintended changes to `go.mod` / `go.sum`)
- [ ] Tests pass (`go test ./...`)
- [ ] Business logic kept separate from Azure SDK calls and CLI plumbing
- [ ] Unit tests added for new exported behavior; ADO client mocked in service tests
- [ ] README or other docs updated if behavior or configuration changed

## Release Notes

<!-- If this affects release behavior, note any changes to RELEASE_GUIDE.md or Make targets -->
