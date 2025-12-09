package ado

import (
	"context"
	"errors"
	"fmt"
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
	if err := validateConfig(trimmed); err != nil {
		return nil, err
	}

	connection := azuredevops.NewPatConnection(trimmed.OrganizationURL, trimmed.Token)
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

func validateConfig(cfg Config) error {
	switch {
	case cfg.OrganizationURL == "":
		return errors.New("ado client: organization url is required")
	case cfg.Project == "":
		return errors.New("ado client: project is required")
	case cfg.Repository == "":
		return errors.New("ado client: repository is required")
	case cfg.Token == "":
		return errors.New("ado client: token is required")
	default:
		return nil
	}
}

func convertGitRefs(values []git.GitRef) []Ref {
	if len(values) == 0 {
		return nil
	}
	refs := make([]Ref, 0, len(values))
	for _, r := range values {
		refs = append(refs, Ref{
			Name:     strings.TrimSpace(derefString(r.Name)),
			ObjectID: strings.TrimSpace(derefString(r.ObjectId)),
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
