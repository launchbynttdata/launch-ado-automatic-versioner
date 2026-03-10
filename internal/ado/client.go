package ado

import (
	"context"
	"errors"
)

// ErrPullRequestNotFound indicates no pull request matched the query.
var ErrPullRequestNotFound = errors.New("ado: pull request not found")

// Ref represents a Git ref returned by Azure DevOps.
type Ref struct {
	Name     string
	ObjectID string
}

// TagObjectType enumerates the Git object kinds supported when creating annotated tags.
type TagObjectType string

const (
	// TagObjectTypeCommit represents a Git commit object.
	TagObjectTypeCommit TagObjectType = "commit"
)

// TagSpec captures the information required to create an annotated tag in ADO Git.
type TagSpec struct {
	Name        string
	ObjectID    string
	ObjectType  TagObjectType
	Message     string
	TaggerName  string
	TaggerEmail string
}

// Client describes the Azure DevOps Git operations required by the business logic layer.
type Client interface {
	// ListRefsWithPrefix returns refs whose names start with the provided prefix
	// (e.g. "refs/tags/"). The concrete client encapsulates organization/project/repo details.
	ListRefsWithPrefix(ctx context.Context, prefix string) ([]Ref, error)

	// DeleteRef removes the specified ref when the current object ID matches.
	DeleteRef(ctx context.Context, name string, objectID string) error

	// FindPullRequestByMergeCommit returns the pull request ID whose merge commit equals commitSHA.
	FindPullRequestByMergeCommit(ctx context.Context, commitSHA string) (int, error)

	// ListPRLabels returns the labels currently applied to the specified pull request.
	ListPRLabels(ctx context.Context, prID int) ([]string, error)

	// AddPRLabel adds the provided label to the specified pull request.
	AddPRLabel(ctx context.Context, prID int, label string) error

	// CreateAnnotatedTag creates an annotated Git tag in the configured repository.
	CreateAnnotatedTag(ctx context.Context, spec TagSpec) error
}
