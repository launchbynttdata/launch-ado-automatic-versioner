package ado

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"
	"time"

	azuredevops "github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
)

// Config controls how the Azure DevOps client connects to the Git API.
type Config struct {
	OrganizationURL string
	Project         string
	Repository      string
	Token           string
}

// NewClient constructs a Client backed by the official Azure DevOps Go SDK.
func NewClient(ctx context.Context, cfg Config) (Client, error) {
	if ctx == nil {
		return nil, errors.New("ado client: context is nil")
	}

	trimmed := sanitizeConfig(cfg)
	canonicalOrgURL, err := validateAndCanonicalizeConfig(trimmed)
	if err != nil {
		return nil, err
	}

	connection := azuredevops.NewPatConnection(canonicalOrgURL, trimmed.Token)
	gitClient, err := git.NewClient(ctx, connection)
	if err != nil {
		return nil, fmt.Errorf("creating git client: %w", err)
	}

	project := trimmed.Project
	repository := trimmed.Repository

	return &sdkClient{
		git:        gitClient,
		project:    &project,
		repository: &repository,
	}, nil
}

type sdkClient struct {
	git        git.Client
	project    *string
	repository *string
}

// ListRefsWithPrefix returns all refs whose names start with the provided prefix.
func (c *sdkClient) ListRefsWithPrefix(ctx context.Context, prefix string) ([]Ref, error) {
	filter := strings.TrimSpace(prefix)
	filter = strings.TrimPrefix(filter, "refs/")
	var continuation *string
	var results []Ref

	peelTags := true
	for {
		args := git.GetRefsArgs{
			Project:      c.project,
			RepositoryId: c.repository,
		}
		if filter != "" {
			args.Filter = &filter
		}
		if continuation != nil {
			args.ContinuationToken = continuation
		}
		args.PeelTags = &peelTags

		resp, err := c.git.GetRefs(ctx, args)
		if err != nil {
			return nil, fmt.Errorf("listing refs: %w", err)
		}
		if resp == nil {
			break
		}

		results = append(results, convertGitRefs(resp.Value)...)

		if resp.ContinuationToken == "" {
			break
		}
		token := resp.ContinuationToken
		continuation = &token
	}

	return results, nil
}

// DeleteRef removes a ref when the current object ID matches.
func (c *sdkClient) DeleteRef(ctx context.Context, name string, objectID string) error {
	refName := strings.TrimSpace(name)
	if refName == "" {
		return errors.New("ado client: ref name is empty")
	}
	current := strings.TrimSpace(objectID)
	if current == "" {
		return errors.New("ado client: ref object id is empty")
	}
	zero := strings.Repeat("0", 40)
	updates := []git.GitRefUpdate{
		{
			Name:        &refName,
			OldObjectId: &current,
			NewObjectId: &zero,
		},
	}
	args := git.UpdateRefsArgs{
		Project:      c.project,
		RepositoryId: c.repository,
		RefUpdates:   &updates,
	}
	results, err := c.git.UpdateRefs(ctx, args)
	if err != nil {
		return fmt.Errorf("deleting ref %s: %w", refName, err)
	}
	if err := errIfRefDeleteUpdateRejected(results, refName); err != nil {
		return err
	}
	return nil
}

// errIfRefDeleteUpdateRejected returns an error when Azure DevOps rejected the
// ref update in the response body (transport succeeded but Success is false).
func errIfRefDeleteUpdateRejected(results *[]git.GitRefUpdateResult, refName string) error {
	if results == nil || len(*results) != 1 || !derefBool((*results)[0].Success) {
		return fmt.Errorf("deleting ref %s rejected", refName)
	}
	return nil
}

// FindPullRequestByMergeCommit returns the PR ID whose merge commit equals commitSHA.
func (c *sdkClient) FindPullRequestByMergeCommit(ctx context.Context, commitSHA string) (int, error) {
	commit := strings.TrimSpace(commitSHA)
	if commit == "" {
		return 0, errors.New("ado client: commit sha is empty")
	}

	queryType := git.GitPullRequestQueryTypeValues.LastMergeCommit
	items := []string{commit}
	queryInputs := []git.GitPullRequestQueryInput{
		{
			Items: &items,
			Type:  &queryType,
		},
	}
	request := git.GitPullRequestQuery{Queries: &queryInputs}
	args := git.GetPullRequestQueryArgs{
		Project:      c.project,
		RepositoryId: c.repository,
		Queries:      &request,
	}

	resp, err := c.git.GetPullRequestQuery(ctx, args)
	if err != nil {
		return 0, fmt.Errorf("querying pull requests: %w", err)
	}

	prID, ok := pullRequestIDFromQuery(commit, resp)
	if !ok {
		return 0, ErrPullRequestNotFound
	}
	return prID, nil
}

// GetPullRequestTitle returns the title of the specified pull request.
func (c *sdkClient) GetPullRequestTitle(ctx context.Context, prID int) (string, error) {
	if prID <= 0 {
		return "", errors.New("ado client: invalid pull request id")
	}

	args := git.GetPullRequestArgs{
		Project:       c.project,
		RepositoryId:  c.repository,
		PullRequestId: &prID,
	}

	pr, err := c.git.GetPullRequest(ctx, args)
	if err != nil {
		return "", fmt.Errorf("getting pull request: %w", err)
	}

	return pullRequestTitleFromGitPR(pr)
}

// pullRequestTitleFromGitPR extracts and validates a non-empty title from a Git pull request.
func pullRequestTitleFromGitPR(pr *git.GitPullRequest) (string, error) {
	if pr == nil || pr.Title == nil {
		return "", errors.New("ado client: pull request title is empty")
	}

	title := strings.TrimSpace(*pr.Title)
	if title == "" {
		return "", errors.New("ado client: pull request title is empty")
	}

	return title, nil
}

// ListPRLabels returns the labels currently applied to the pull request.
func (c *sdkClient) ListPRLabels(ctx context.Context, prID int) ([]string, error) {
	args := git.GetPullRequestLabelsArgs{
		Project:       c.project,
		RepositoryId:  c.repository,
		PullRequestId: &prID,
	}

	labels, err := c.git.GetPullRequestLabels(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("listing pull request labels: %w", err)
	}

	return labelNames(labels), nil
}

// AddPRLabel adds the provided label to the specified pull request.
func (c *sdkClient) AddPRLabel(ctx context.Context, prID int, label string) error {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return errors.New("ado client: label name is empty")
	}

	name := trimmed
	request := core.WebApiCreateTagRequestData{Name: &name}
	args := git.CreatePullRequestLabelArgs{
		Project:       c.project,
		RepositoryId:  c.repository,
		PullRequestId: &prID,
		Label:         &request,
	}

	if _, err := c.git.CreatePullRequestLabel(ctx, args); err != nil {
		return fmt.Errorf("creating pull request label: %w", err)
	}

	return nil
}

// CreateAnnotatedTag creates an annotated tag referencing the supplied commit.
func (c *sdkClient) CreateAnnotatedTag(ctx context.Context, spec TagSpec) error {
	tag, err := buildAnnotatedTag(spec)
	if err != nil {
		return err
	}

	args := git.CreateAnnotatedTagArgs{
		Project:      c.project,
		RepositoryId: c.repository,
		TagObject:    &tag,
	}

	if _, err := c.git.CreateAnnotatedTag(ctx, args); err != nil {
		return fmt.Errorf("creating annotated tag: %w", err)
	}

	return nil
}

func sanitizeConfig(cfg Config) Config {
	return Config{
		OrganizationURL: strings.TrimSpace(cfg.OrganizationURL),
		Project:         strings.TrimSpace(cfg.Project),
		Repository:      strings.TrimSpace(cfg.Repository),
		Token:           strings.TrimSpace(cfg.Token),
	}
}

func validateAndCanonicalizeConfig(cfg Config) (string, error) {
	switch {
	case cfg.OrganizationURL == "":
		return "", errors.New("ado client: organization url is required")
	case cfg.Project == "":
		return "", errors.New("ado client: project is required")
	case cfg.Repository == "":
		return "", errors.New("ado client: repository is required")
	case cfg.Token == "":
		return "", errors.New("ado client: token is required")
	}

	return canonicalizeOrganizationURL(cfg.OrganizationURL)
}

func canonicalizeOrganizationURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("ado client: organization url is required")
	}
	if strings.ContainsAny(trimmed, " \t\r\n\\#") {
		return "", errors.New("ado client: organization url is malformed or contains fragments")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("ado client: invalid organization url: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return "", errors.New("ado client: organization url must use https")
	}
	if parsed.Opaque != "" {
		return "", errors.New("ado client: organization url must not be opaque")
	}
	if parsed.User != nil {
		return "", errors.New("ado client: organization url must not contain userinfo")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery {
		return "", errors.New("ado client: organization url must not contain query parameters")
	}
	if parsed.Fragment != "" {
		return "", errors.New("ado client: organization url must not contain a fragment")
	}

	hostname := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(parsed.Hostname()), "."))
	if !isValidOrganizationHostname(hostname) {
		return "", errors.New("ado client: organization url has an invalid or disallowed host")
	}

	port := parsed.Port()
	if port != "" && port != "443" {
		return "", errors.New("ado client: organization url must use default https port 443")
	}

	cleanedPath, err := cleanOrganizationPath(parsed.Path)
	if err != nil {
		return "", err
	}

	canonical := url.URL{
		Scheme: "https",
		Host:   hostname,
		Path:   cleanedPath,
	}
	return canonical.String(), nil
}

func isValidOrganizationHostname(hostname string) bool {
	if hostname == "" || hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return false
	}
	if net.ParseIP(hostname) != nil {
		return false
	}
	if !strings.Contains(hostname, ".") {
		return false
	}
	labels := strings.Split(hostname, ".")
	allDigitsLast := true
	lastLabel := labels[len(labels)-1]
	for i := 0; i < len(lastLabel); i++ {
		if lastLabel[i] < '0' || lastLabel[i] > '9' {
			allDigitsLast = false
			break
		}
	}
	if allDigitsLast {
		return false
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			ch := label[i]
			isLowerAlpha := ch >= 'a' && ch <= 'z'
			isDigit := ch >= '0' && ch <= '9'
			if !isLowerAlpha && !isDigit && ch != '-' {
				return false
			}
		}
	}
	return true
}

func cleanOrganizationPath(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" || trimmed == "/" {
		return "", nil
	}
	for _, seg := range strings.Split(trimmed, "/") {
		if seg == "." || seg == ".." {
			return "", errors.New("ado client: organization url path must not contain relative segments")
		}
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(trimmed, "/"))
	if cleaned == "/" {
		return "", nil
	}
	return strings.TrimRight(cleaned, "/"), nil
}

func convertGitRefs(values []git.GitRef) []Ref {
	if len(values) == 0 {
		return nil
	}
	refs := make([]Ref, 0, len(values))
	for _, r := range values {
		name := strings.TrimSpace(derefString(r.Name))
		objectID := strings.TrimSpace(derefString(r.ObjectId))
		peeledObjectID := strings.TrimSpace(derefString(r.PeeledObjectId))
		refs = append(refs, Ref{
			Name:           name,
			ObjectID:       objectID,
			PeeledObjectID: peeledObjectID,
		})
	}
	return refs
}

func pullRequestIDFromQuery(commit string, response *git.GitPullRequestQuery) (int, bool) {
	if response == nil || response.Results == nil {
		return 0, false
	}
	for _, result := range *response.Results {
		if commit != "" {
			if prs, ok := result[commit]; ok {
				if id, found := extractPRID(prs); found {
					return id, true
				}
			}
		}
		if id, ok := firstPRIDFromMap(result); ok {
			return id, true
		}
	}
	return 0, false
}

func firstPRIDFromMap(result map[string][]git.GitPullRequest) (int, bool) {
	if len(result) == 0 {
		return 0, false
	}
	for _, prs := range result {
		if id, ok := extractPRID(prs); ok {
			return id, true
		}
	}
	return 0, false
}

func extractPRID(prs []git.GitPullRequest) (int, bool) {
	for _, pr := range prs {
		if pr.PullRequestId != nil {
			return *pr.PullRequestId, true
		}
	}
	return 0, false
}

func labelNames(defs *[]core.WebApiTagDefinition) []string {
	if defs == nil || len(*defs) == 0 {
		return nil
	}
	names := make([]string, 0, len(*defs))
	for _, def := range *defs {
		name := strings.TrimSpace(derefString(def.Name))
		if name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func derefBool(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

func buildAnnotatedTag(spec TagSpec) (git.GitAnnotatedTag, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return git.GitAnnotatedTag{}, errors.New("ado client: tag name is empty")
	}

	objectID := strings.TrimSpace(spec.ObjectID)
	if objectID == "" {
		return git.GitAnnotatedTag{}, errors.New("ado client: tag object id is empty")
	}

	objectType, err := convertObjectType(spec.ObjectType)
	if err != nil {
		return git.GitAnnotatedTag{}, err
	}

	taggerName := strings.TrimSpace(spec.TaggerName)
	if taggerName == "" {
		return git.GitAnnotatedTag{}, errors.New("ado client: tagger name is empty")
	}

	taggerEmail := strings.TrimSpace(spec.TaggerEmail)
	if taggerEmail == "" {
		return git.GitAnnotatedTag{}, errors.New("ado client: tagger email is empty")
	}

	annotated := git.GitAnnotatedTag{}
	annotated.Name = &name
	annotated.TaggedObject = &git.GitObject{ObjectId: &objectID, ObjectType: objectType}

	if message := strings.TrimSpace(spec.Message); message != "" {
		annotated.Message = &message
	}

	stamp := azuredevops.Time{Time: time.Now().UTC()}
	annotated.TaggedBy = &git.GitUserDate{
		Name:  &taggerName,
		Email: &taggerEmail,
		Date:  &stamp,
	}

	return annotated, nil
}

func convertObjectType(objectType TagObjectType) (*git.GitObjectType, error) {
	switch objectType {
	case "", TagObjectTypeCommit:
		value := git.GitObjectTypeValues.Commit
		return &value, nil
	default:
		return nil, fmt.Errorf("ado client: unsupported tag object type %q", objectType)
	}
}
