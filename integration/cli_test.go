//go:build integration
// +build integration

package integration_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	semver "github.com/blang/semver/v4"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/branchmap"
	azuredevops "github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
)

const (
	envOrgURL         = "AAV_ORG_URL"
	envProject        = "AAV_PROJECT"
	envRepo           = "AAV_REPO"
	envToken          = "AAV_TOKEN"
	envExpectedBump   = "AAV_EXPECTED_BUMP"
	envTargetBranch   = "AAV_TARGET_BRANCH"
	envManualMerge    = "AAV_MANUAL_MERGE"
	envGitAuthorName  = "AAV_GIT_AUTHOR_NAME"
	envGitAuthorEmail = "AAV_GIT_AUTHOR_EMAIL"
	envTaggerName     = "AAV_TAGGER_NAME"
	envTaggerEmail    = "AAV_TAGGER_EMAIL"
	envBadCommit      = "AAV_BAD_COMMIT_SHA"
	envBadPRID        = "AAV_BAD_PR_ID"
	envTagPrefix      = "AAV_TAG_PREFIX"
	envLogLevel       = "AAV_LOG_LEVEL"
	envPRID           = "AAV_PR_ID"
	envSourceBranch   = "AAV_SOURCE_BRANCH"
	envCommit         = "AAV_COMMIT_SHA"
	envStrict         = "AAV_STRICT"
	envTagMode        = "AAV_TAG_MODE"
	envBump           = "AAV_BUMP"
	envTagMessage     = "AAV_TAG_MESSAGE"
	envBranchMajor    = "AAV_BRANCH_MAJOR_PREFIXES"
	envBranchMinor    = "AAV_BRANCH_MINOR_PREFIXES"
	envBranchPatch    = "AAV_BRANCH_PATCH_PREFIXES"
	envUseFloating    = "AAV_USE_FLOATING_TAGS"
)

const (
	defaultTargetBranch    = "main"
	defaultAuthorName      = "aav-integration"
	defaultAuthorEmail     = "aav-integration@example.com"
	defaultTaggerName      = "aav-integration"
	defaultTaggerEmail     = "aav-integration@example.com"
	defaultExpectedBump    = "minor"
	defaultBadCommit       = "0000000000000000000000000000000000000000"
	defaultBadPRID         = "999999999"
	workflowTimeout        = 10 * time.Minute
	pollInterval           = 5 * time.Second
	integrationDir         = "integration-artifacts"
	gitTerminalPromptOff   = "GIT_TERMINAL_PROMPT=0"
	floatingVerifyTimeout  = 2 * time.Minute
	floatingVerifyInterval = 3 * time.Second
)

func TestIntegrationWorkflowCreatesReleaseAndRCTags(t *testing.T) {
	cfg := loadConfig(t)
	for _, bump := range orderedBumpCoverage(cfg.ExpectedBump) {
		bump := bump
		t.Run(fmt.Sprintf("%s-bump", bump), func(t *testing.T) {
			scenarioCfg := cfg
			scenarioCfg.ExpectedBump = bump
			runReleaseAndRcScenario(t, scenarioCfg)
		})
	}
}

func TestIntegrationWorkflowFailureModes(t *testing.T) {
	cfg := loadConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), workflowTimeout)
	defer cancel()

	h := newWorkflowHarness(t, ctx, cfg)
	branchName, prID := h.createBranchAndPullRequest(t)
	_ = h.runWorkflowAndMerge(t, prID, branchName)

	// invalid PR ID for pr-label
	if stdout, stderr, err := h.runCLI(t, []string{"pr-label"}, map[string]string{
		envPRID:         cfg.BadPRID,
		envSourceBranch: branchName,
	}); err == nil {
		t.Fatalf("expected pr-label to fail with invalid PR; stdout=%q stderr=%q", stdout, stderr)
	}

	// invalid commit for infer-bump strict mode
	if stdout, stderr, err := h.runCLI(t, []string{"infer-bump"}, map[string]string{
		envCommit: cfg.BadCommitSHA,
		envStrict: "true",
	}); err == nil {
		t.Fatalf("expected infer-bump to fail with invalid commit; stdout=%q stderr=%q", stdout, stderr)
	}
}

func runReleaseAndRcScenario(t *testing.T, cfg envConfig) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), workflowTimeout)
	defer cancel()

	h := newWorkflowHarness(t, ctx, cfg)
	branchName, prID := h.createBranchAndPullRequest(t)
	mergeCommit := h.runWorkflowAndMerge(t, prID, branchName)

	releaseTag := h.createTagWithCLI(t, mergeCommit, "release", cfg.ExpectedBump)
	h.registerTag(releaseTag)

	if floating := h.assertFloatingTag(t, releaseTag, mergeCommit); floating != "" {
		h.registerTag(floating)
	}

	rcTag := h.createTagWithCLI(t, mergeCommit, "rc", cfg.ExpectedBump)
	h.registerTag(rcTag)
}

type envConfig struct {
	OrgURL         string
	Project        string
	Repo           string
	Token          string
	ExpectedBump   string
	TargetBranch   string
	ManualMerge    bool
	GitAuthorName  string
	GitAuthorEmail string
	TaggerName     string
	TaggerEmail    string
	TagPrefix      string
	BadCommitSHA   string
	BadPRID        string
	BranchPrefixes branchPrefixConfig
	UseFloatingTag bool
}

func loadConfig(t *testing.T) envConfig {
	t.Helper()
	expected := strings.ToLower(optionalEnv(envExpectedBump, defaultExpectedBump))
	if !isValidBump(expected) {
		t.Fatalf("invalid expected bump %q; must be major, minor, or patch", expected)
	}
	return envConfig{
		OrgURL:         requireEnv(t, envOrgURL),
		Project:        requireEnv(t, envProject),
		Repo:           requireEnv(t, envRepo),
		Token:          requireEnv(t, envToken),
		ExpectedBump:   expected,
		TargetBranch:   strings.TrimSpace(optionalEnv(envTargetBranch, defaultTargetBranch)),
		ManualMerge:    parseBoolEnv(envManualMerge),
		GitAuthorName:  optionalEnv(envGitAuthorName, defaultAuthorName),
		GitAuthorEmail: optionalEnv(envGitAuthorEmail, defaultAuthorEmail),
		TaggerName:     optionalEnv(envTaggerName, defaultTaggerName),
		TaggerEmail:    optionalEnv(envTaggerEmail, defaultTaggerEmail),
		TagPrefix:      optionalEnv(envTagPrefix, ""),
		BadCommitSHA:   optionalEnv(envBadCommit, defaultBadCommit),
		BadPRID:        optionalEnv(envBadPRID, defaultBadPRID),
		BranchPrefixes: loadBranchPrefixConfig(),
		UseFloatingTag: parseBoolEnv(envUseFloating),
	}
}

type workflowHarness struct {
	t      *testing.T
	ctx    context.Context
	cfg    envConfig
	gitDir string
	git    *gitWorkspace
	ado    *adoWorkflowClient
	tags   []string
}

type refState struct {
	Ref            string
	ObjectID       string
	PeeledObjectID string
}

func (r refState) commitID() string {
	if r.PeeledObjectID != "" {
		return r.PeeledObjectID
	}
	return r.ObjectID
}

func newWorkflowHarness(t *testing.T, ctx context.Context, cfg envConfig) *workflowHarness {
	gitDir := cloneRepository(t, cfg)
	adoClient, err := newADOClient(ctx, cfg)
	if err != nil {
		t.Fatalf("creating ado client: %v", err)
	}
	h := &workflowHarness{
		t:      t,
		ctx:    ctx,
		cfg:    cfg,
		gitDir: gitDir,
		git:    &gitWorkspace{dir: gitDir, cfg: cfg},
		ado:    adoClient,
	}
	t.Cleanup(func() {
		for _, tag := range h.tags {
			h.git.deleteRemoteTag(t, tag)
		}
	})
	return h
}

func (h *workflowHarness) createBranchAndPullRequest(t *testing.T) (string, int) {
	prefix := h.cfg.branchPrefixFor(h.cfg.ExpectedBump)
	if strings.TrimSpace(prefix) == "" {
		t.Fatalf("no branch prefix configured for bump %q; set %s/%s/%s", h.cfg.ExpectedBump, envBranchMajor, envBranchMinor, envBranchPatch)
	}
	branchName := fmt.Sprintf("%saav-int-%s", prefix, randomSuffix())
	commitMessage := fmt.Sprintf("integration: %s", branchName)
	commitPath := filepath.Join(integrationDir, fmt.Sprintf("%s.txt", randomSuffix()))
	h.git.ensureCleanBase(t, h.cfg.TargetBranch)
	h.git.createCommitOnBranch(t, branchName, commitPath, commitMessage)
	prID, err := h.ado.createPullRequest(h.ctx, branchName, h.cfg.TargetBranch, commitMessage)
	if err != nil {
		t.Fatalf("creating pull request: %v", err)
	}
	h.t.Logf("created PR %d for branch %s", prID, branchName)
	return branchName, prID
}

func (h *workflowHarness) runWorkflowAndMerge(t *testing.T, prID int, branch string) string {
	if _, stderr, err := h.runCLI(t, []string{"pr-label"}, map[string]string{
		envPRID:         fmt.Sprintf("%d", prID),
		envSourceBranch: branch,
	}); err != nil {
		t.Fatalf("pr-label failed: %v\nstderr: %s", err, stderr)
	}

	if h.cfg.ManualMerge {
		t.Logf("manual merge enabled: complete PR %d at %s", prID, h.ado.pullRequestURL(prID))
	} else {
		if err := h.ado.completePullRequest(h.ctx, prID); err != nil {
			t.Fatalf("completing pull request: %v", err)
		}
	}

	mergeCommit, err := h.ado.waitForCompletion(h.ctx, prID)
	if err != nil {
		t.Fatalf("waiting for merge: %v", err)
	}
	h.t.Logf("pull request %d merged as %s", prID, mergeCommit)

	stdout, stderr, err := h.runCLI(t, []string{"infer-bump"}, map[string]string{
		envCommit: mergeCommit,
		envStrict: "true",
	})
	if err != nil {
		t.Fatalf("infer-bump failed after merge: %v\nstderr: %s", err, stderr)
	}
	if stdout != h.cfg.ExpectedBump {
		t.Fatalf("infer-bump mismatch: expected %s got %s", h.cfg.ExpectedBump, stdout)
	}

	return mergeCommit
}

func (h *workflowHarness) createTagWithCLI(t *testing.T, commit, mode, bump string) string {
	overrides := map[string]string{
		envCommit:      commit,
		envTagMode:     mode,
		envBump:        bump,
		envTaggerName:  h.cfg.TaggerName,
		envTaggerEmail: h.cfg.TaggerEmail,
		envTagMessage:  fmt.Sprintf("integration %s", mode),
	}
	if mode == "release" && h.cfg.UseFloatingTag {
		overrides[envUseFloating] = "true"
	}
	stdout, stderr, err := h.runCLI(t, []string{"create-tag"}, overrides)
	if err != nil {
		t.Fatalf("create-tag (%s) failed: %v\nstderr: %s", mode, err, stderr)
	}
	if stdout == "" {
		t.Fatalf("create-tag (%s) produced empty tag name", mode)
	}
	return stdout
}

func (h *workflowHarness) registerTag(tag string) {
	h.tags = append(h.tags, tag)
}

func (h *workflowHarness) assertFloatingTag(t *testing.T, releaseTag, releaseCommit string) string {
	t.Helper()
	if !h.cfg.UseFloatingTag {
		return ""
	}

	trimmedCommit := strings.TrimSpace(releaseCommit)
	if trimmedCommit == "" {
		t.Fatalf("release commit is empty while asserting floating tag")
	}

	releaseRef := tagRefName(releaseTag)
	releaseState := h.waitForRefState(t, releaseRef)
	releaseTarget := releaseState.commitID()
	if releaseTarget == "" {
		t.Fatalf("release tag %s missing peeled commit", releaseTag)
	}
	if releaseTarget != trimmedCommit {
		h.t.Logf("release tag %s references %s; merge commit %s", releaseTag, releaseTarget, trimmedCommit)
	}

	version := parseReleaseVersion(t, releaseTag, h.cfg.TagPrefix)
	floatingTag := fmt.Sprintf("v%d", version.Major)
	floatingRef := tagRefName(floatingTag)
	floatingState := h.waitForRefMatch(t, floatingRef, releaseTarget)
	h.t.Logf("floating tag %s matches release tag %s @ %s", floatingTag, releaseTag, floatingState.commitID())
	return floatingTag
}

func tagRefName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	return fmt.Sprintf("refs/tags/%s", trimmed)
}

func (h *workflowHarness) waitForRefState(t *testing.T, ref string) refState {
	deadline := time.Now().Add(floatingVerifyTimeout)
	var lastErr error
	for {
		state, err := h.ado.refState(h.ctx, ref)
		if err == nil && state.ObjectID != "" {
			return state
		}
		if err != nil {
			lastErr = err
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				t.Fatalf("timed out waiting for ref %s: %v", ref, lastErr)
			}
			t.Fatalf("timed out waiting for ref %s to appear", ref)
		}
		time.Sleep(floatingVerifyInterval)
	}
}

func (h *workflowHarness) waitForRefMatch(t *testing.T, ref string, expected string) refState {
	deadline := time.Now().Add(floatingVerifyTimeout)
	var lastObserved refState
	for {
		state, err := h.ado.refState(h.ctx, ref)
		commitID := state.commitID()
		if err == nil && commitID == expected {
			return state
		}
		if err == nil && commitID != "" {
			lastObserved = state
		}
		if err != nil {
			h.t.Logf("waiting for %s to match %s: %v", ref, expected, err)
		}
		if time.Now().After(deadline) {
			if lastObserved.commitID() == "" {
				t.Fatalf("timed out waiting for ref %s to match release object %s", ref, expected)
			}
			t.Fatalf("ref %s last observed at %s; expected %s", ref, lastObserved.commitID(), expected)
		}
		time.Sleep(floatingVerifyInterval)
	}
}

func (h *workflowHarness) runCLI(t *testing.T, args []string, overrides map[string]string) (string, string, error) {
	t.Helper()
	cmd := exec.CommandContext(h.ctx, "go", append([]string{"run", "./cmd/aav"}, args...)...)
	cmd.Dir = projectRoot(t)
	envMap := h.baseCLIEnv()
	for k, v := range overrides {
		envMap[k] = v
	}
	cmd.Env = append(os.Environ(), flattenEnv(envMap)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	h.t.Logf("running CLI: aav %s overrides=%v", strings.Join(args, " "), overrides)
	err := cmd.Run()
	stdoutStr := strings.TrimSpace(stdout.String())
	stderrStr := strings.TrimSpace(stderr.String())
	h.t.Logf("CLI result for %v err=%v stdout=%q stderr=%q", args, err, stdoutStr, stderrStr)
	return stdoutStr, stderrStr, err
}

func (h *workflowHarness) baseCLIEnv() map[string]string {
	envMap := map[string]string{
		envOrgURL:   h.cfg.OrgURL,
		envProject:  h.cfg.Project,
		envRepo:     h.cfg.Repo,
		envToken:    h.cfg.Token,
		envLogLevel: "verbose",
	}
	if h.cfg.TagPrefix != "" {
		envMap[envTagPrefix] = h.cfg.TagPrefix
	}
	if joined := joinPrefixes(h.cfg.BranchPrefixes.Major); joined != "" {
		envMap[envBranchMajor] = joined
	}
	if joined := joinPrefixes(h.cfg.BranchPrefixes.Minor); joined != "" {
		envMap[envBranchMinor] = joined
	}
	if joined := joinPrefixes(h.cfg.BranchPrefixes.Patch); joined != "" {
		envMap[envBranchPatch] = joined
	}
	return envMap
}

type gitWorkspace struct {
	dir string
	cfg envConfig
}

func cloneRepository(t *testing.T, cfg envConfig) string {
	t.Helper()
	dir := t.TempDir()
	remote := authenticatedRemoteURL(cfg)
	cmd := exec.Command("git", "clone", remote, dir)
	cmd.Env = append(os.Environ(), gitTerminalPromptOff)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone failed: %s", sanitizeOutput(output, cfg.Token))
	}
	workspace := &gitWorkspace{dir: dir, cfg: cfg}
	workspace.run(t, "config", "user.name", cfg.GitAuthorName)
	workspace.run(t, "config", "user.email", cfg.GitAuthorEmail)
	return dir
}

func (w *gitWorkspace) ensureCleanBase(t *testing.T, target string) {
	w.run(t, "checkout", target)
	w.run(t, "pull", "--ff-only")
}

func (w *gitWorkspace) createCommitOnBranch(t *testing.T, branch, relPath, message string) {
	w.run(t, "checkout", "-b", branch)
	fullPath := filepath.Join(w.dir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("creating artifact directory: %v", err)
	}
	content := []byte(time.Now().UTC().Format(time.RFC3339Nano))
	if err := os.WriteFile(fullPath, content, 0o644); err != nil {
		t.Fatalf("writing artifact: %v", err)
	}
	rel := strings.TrimPrefix(fullPath, w.dir+string(os.PathSeparator))
	w.run(t, "add", rel)
	w.run(t, "commit", "-m", message)
	w.run(t, "push", "-u", "origin", branch)
}

func (w *gitWorkspace) deleteRemoteTag(t *testing.T, tag string) {
	if tag == "" {
		return
	}
	if err := w.runOptional("tag", "-d", tag); err != nil {
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "unknown") {
			t.Logf("tag %s not present locally; skipping local delete", tag)
		} else {
			t.Fatalf("git tag -d %s failed: %v", tag, err)
		}
	}
	w.run(t, "push", "origin", fmt.Sprintf(":refs/tags/%s", tag))
}

func (w *gitWorkspace) run(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = w.dir
	cmd.Env = append(os.Environ(), gitTerminalPromptOff)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %s", strings.Join(args, " "), sanitizeOutput(output, w.cfg.Token))
	}
}

func (w *gitWorkspace) runOptional(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = w.dir
	cmd.Env = append(os.Environ(), gitTerminalPromptOff)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s failed: %s", strings.Join(args, " "), sanitizeOutput(output, w.cfg.Token))
	}
	return nil
}

type adoWorkflowClient struct {
	git     git.Client
	project string
	repo    string
	orgURL  string
}

func newADOClient(ctx context.Context, cfg envConfig) (*adoWorkflowClient, error) {
	connection := azuredevops.NewPatConnection(cfg.OrgURL, cfg.Token)
	client, err := git.NewClient(ctx, connection)
	if err != nil {
		return nil, err
	}
	return &adoWorkflowClient{git: client, project: cfg.Project, repo: cfg.Repo, orgURL: cfg.OrgURL}, nil
}

func (c *adoWorkflowClient) createPullRequest(ctx context.Context, sourceBranch, targetBranch, title string) (int, error) {
	source := fmt.Sprintf("refs/heads/%s", sourceBranch)
	target := fmt.Sprintf("refs/heads/%s", targetBranch)
	req := git.GitPullRequest{
		Title:         &title,
		SourceRefName: &source,
		TargetRefName: &target,
	}
	args := git.CreatePullRequestArgs{
		GitPullRequestToCreate: &req,
		Project:                &c.project,
		RepositoryId:           &c.repo,
	}
	resp, err := c.git.CreatePullRequest(ctx, args)
	if err != nil {
		return 0, err
	}
	if resp.PullRequestId == nil {
		return 0, errors.New("pull request id missing")
	}
	return *resp.PullRequestId, nil
}

func (c *adoWorkflowClient) completePullRequest(ctx context.Context, prID int) error {
	pr, err := c.getPullRequest(ctx, prID)
	if err != nil {
		return err
	}
	status := git.PullRequestStatusValues.Completed
	squash := true
	deleteSource := true
	strategy := git.GitPullRequestMergeStrategyValues.Squash
	options := git.GitPullRequestCompletionOptions{
		DeleteSourceBranch: &deleteSource,
		SquashMerge:        &squash,
		MergeStrategy:      &strategy,
	}
	req := git.GitPullRequest{
		Status:                &status,
		LastMergeSourceCommit: pr.LastMergeSourceCommit,
		CompletionOptions:     &options,
	}
	args := git.UpdatePullRequestArgs{
		GitPullRequestToUpdate: &req,
		Project:                &c.project,
		RepositoryId:           &c.repo,
		PullRequestId:          &prID,
	}
	_, err = c.git.UpdatePullRequest(ctx, args)
	return err
}

func (c *adoWorkflowClient) waitForCompletion(ctx context.Context, prID int) (string, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		pr, err := c.getPullRequest(ctx, prID)
		if err != nil {
			return "", err
		}
		if pr.Status != nil && *pr.Status == git.PullRequestStatusValues.Completed {
			if pr.LastMergeCommit != nil && pr.LastMergeCommit.CommitId != nil {
				return *pr.LastMergeCommit.CommitId, nil
			}
			return "", errors.New("merge commit missing after completion")
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("waiting for PR completion: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (c *adoWorkflowClient) pullRequestURL(prID int) string {
	base := strings.TrimSuffix(c.orgURL, "/")
	proj := url.PathEscape(c.project)
	repo := url.PathEscape(c.repo)
	return fmt.Sprintf("%s/%s/_git/%s/pullrequest/%d", base, proj, repo, prID)
}

func (c *adoWorkflowClient) getPullRequest(ctx context.Context, prID int) (*git.GitPullRequest, error) {
	args := git.GetPullRequestArgs{
		Project:       &c.project,
		RepositoryId:  &c.repo,
		PullRequestId: &prID,
	}
	pr, err := c.git.GetPullRequest(ctx, args)
	if err != nil {
		return nil, err
	}
	return pr, nil
}

func (c *adoWorkflowClient) refState(ctx context.Context, ref string) (refState, error) {
	desired := strings.TrimSpace(ref)
	if desired == "" {
		return refState{}, nil
	}
	filter := strings.TrimPrefix(desired, "refs/")
	top := 100
	args := git.GetRefsArgs{
		Project:      &c.project,
		RepositoryId: &c.repo,
		Filter:       &filter,
		Top:          &top,
	}
	peelTags := true
	args.PeelTags = &peelTags
	refs, err := c.git.GetRefs(ctx, args)
	if err != nil {
		return refState{}, err
	}
	for _, refInfo := range refs.Value {
		if refInfo.Name == nil || *refInfo.Name != desired {
			continue
		}
		state := refState{
			Ref:            desired,
			ObjectID:       strings.TrimSpace(stringValue(refInfo.ObjectId)),
			PeeledObjectID: strings.TrimSpace(stringValue(refInfo.PeeledObjectId)),
		}
		return state, nil
	}
	return refState{}, nil
}

func isValidBump(value string) bool {
	switch value {
	case "major", "minor", "patch":
		return true
	default:
		return false
	}
}

func (c envConfig) branchPrefixFor(value string) string {
	return c.BranchPrefixes.prefixFor(value)
}

type branchPrefixConfig struct {
	Major []string
	Minor []string
	Patch []string
}

func loadBranchPrefixConfig() branchPrefixConfig {
	defaults := branchmap.DefaultMapping()
	cfg := branchPrefixConfig{
		Major: copyStrings(defaults.MajorPrefixes),
		Minor: copyStrings(defaults.MinorPrefixes),
		Patch: copyStrings(defaults.PatchPrefixes),
	}
	if values, ok := envPrefixList(envBranchMajor); ok {
		cfg.Major = values
	}
	if values, ok := envPrefixList(envBranchMinor); ok {
		cfg.Minor = values
	}
	if values, ok := envPrefixList(envBranchPatch); ok {
		cfg.Patch = values
	}
	return cfg
}

func (p branchPrefixConfig) prefixFor(value string) string {
	var prefixes []string
	switch value {
	case "major":
		prefixes = p.Major
	case "minor":
		prefixes = p.Minor
	default:
		prefixes = p.Patch
	}
	for _, prefix := range prefixes {
		trimmed := strings.TrimSpace(prefix)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func envPrefixList(key string) ([]string, bool) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return nil, false
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, true
	}
	return splitAndCleanList(trimmed), true
}

func splitAndCleanList(raw string) []string {
	parts := strings.Split(raw, ",")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func joinPrefixes(values []string) string {
	if len(values) == 0 {
		return ""
	}
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	return strings.Join(cleaned, ",")
}

func copyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func orderedBumpCoverage(primary string) []string {
	ordered := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)
	add := func(candidate string) {
		if !isValidBump(candidate) {
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		ordered = append(ordered, candidate)
	}
	if primary != "" {
		add(primary)
	}
	for _, candidate := range []string{"major", "minor", "patch"} {
		add(candidate)
	}
	return ordered
}

func parseReleaseVersion(t *testing.T, tagName, prefix string) semver.Version {
	t.Helper()
	value := strings.TrimSpace(tagName)
	if value == "" {
		t.Fatalf("release tag name is empty")
	}
	prefix = strings.TrimSpace(prefix)
	if prefix != "" && strings.HasPrefix(value, prefix) {
		value = strings.TrimPrefix(value, prefix)
	}
	version, err := semver.Parse(strings.TrimSpace(value))
	if err != nil {
		t.Fatalf("parsing release tag %s (prefix %q): %v", tagName, prefix, err)
	}
	return version
}

func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("unable to locate go.mod from %s", dir)
		}
		dir = parent
	}
}

func authenticatedRemoteURL(cfg envConfig) string {
	remote := fmt.Sprintf("%s/%s/_git/%s", strings.TrimSuffix(cfg.OrgURL, "/"), url.PathEscape(cfg.Project), url.PathEscape(cfg.Repo))
	u, err := url.Parse(remote)
	if err != nil {
		panic(err)
	}
	u.User = url.UserPassword("aav", cfg.Token)
	return u.String()
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func randomSuffix() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		t.Skipf("integration env %s is not set; skipping", key)
	}
	return value
}

func optionalEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func parseBoolEnv(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func flattenEnv(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for k, v := range values {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

func sanitizeOutput(output []byte, secret string) string {
	if secret == "" {
		return string(output)
	}
	return strings.ReplaceAll(string(output), secret, "***")
}
