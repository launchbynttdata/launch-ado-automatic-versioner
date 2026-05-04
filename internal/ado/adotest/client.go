package adotest

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/ado"
)

const tagRefPrefix = "refs/tags/"

// DeleteCall records a ref deletion request made through the fake client.
type DeleteCall struct {
	Name        string
	OldObjectID string
}

// Client is an in-memory ado.Client that preserves Azure DevOps tag ref semantics.
type Client struct {
	refs       map[string]ado.Ref
	nextObject int

	ListErr   error
	CreateErr error
	DeleteErr error

	LastPrefix  string
	CreatedTags []ado.TagSpec
	DeletedRefs []DeleteCall
}

// NewClient creates an empty ADO-shaped fake repository.
func NewClient() *Client {
	return &Client{
		refs:       make(map[string]ado.Ref),
		nextObject: 1,
	}
}

// SeedAnnotatedTag inserts an annotated tag ref whose ref object differs from its peeled commit.
func (c *Client) SeedAnnotatedTag(name, refObjectID, targetObjectID string) {
	c.ensureRefs()
	refName := normalizeTagRef(name)
	c.refs[refName] = ado.Ref{
		Name:           refName,
		ObjectID:       strings.TrimSpace(refObjectID),
		PeeledObjectID: strings.TrimSpace(targetObjectID),
	}
}

// Ref returns the current ref state for a tag name or full ref name.
func (c *Client) Ref(name string) (ado.Ref, bool) {
	c.ensureRefs()
	ref, ok := c.refs[normalizeTagRef(name)]
	return ref, ok
}

// ListRefsWithPrefix returns refs whose names start with the requested prefix.
func (c *Client) ListRefsWithPrefix(_ context.Context, prefix string) ([]ado.Ref, error) {
	if c.ListErr != nil {
		return nil, c.ListErr
	}
	c.ensureRefs()
	c.LastPrefix = prefix

	names := make([]string, 0, len(c.refs))
	for name := range c.refs {
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	refs := make([]ado.Ref, 0, len(names))
	for _, name := range names {
		refs = append(refs, c.refs[name])
	}
	return refs, nil
}

// DeleteRef removes a ref only when oldObjectID matches the current ref object ID.
func (c *Client) DeleteRef(_ context.Context, name string, oldObjectID string) error {
	if c.DeleteErr != nil {
		return c.DeleteErr
	}
	c.ensureRefs()

	refName := normalizeTagRef(name)
	ref, ok := c.refs[refName]
	if !ok {
		return fmt.Errorf("adotest: ref %s does not exist", refName)
	}

	current := strings.TrimSpace(ref.ObjectID)
	if current == "" {
		return fmt.Errorf("adotest: ref %s has empty object id", refName)
	}
	if current != strings.TrimSpace(oldObjectID) {
		return fmt.Errorf("adotest: deleting %s: old object id %s does not match current ref object id %s", refName, oldObjectID, current)
	}

	delete(c.refs, refName)
	c.DeletedRefs = append(c.DeletedRefs, DeleteCall{Name: refName, OldObjectID: oldObjectID})
	return nil
}

// CreateAnnotatedTag creates a new annotated tag ref and fails if the ref already exists.
func (c *Client) CreateAnnotatedTag(_ context.Context, spec ado.TagSpec) error {
	if c.CreateErr != nil {
		return c.CreateErr
	}
	c.ensureRefs()

	refName := normalizeTagRef(spec.Name)
	if refName == tagRefPrefix {
		return errors.New("adotest: tag name is empty")
	}
	if _, exists := c.refs[refName]; exists {
		return fmt.Errorf("adotest: ref %s already exists", refName)
	}

	target := strings.TrimSpace(spec.ObjectID)
	if target == "" {
		return errors.New("adotest: tag object id is empty")
	}

	refObjectID := c.nextTagObjectID()
	c.refs[refName] = ado.Ref{
		Name:           refName,
		ObjectID:       refObjectID,
		PeeledObjectID: target,
	}
	c.CreatedTags = append(c.CreatedTags, spec)
	return nil
}

// FindPullRequestByMergeCommit is not implemented for tag workflow tests.
func (c *Client) FindPullRequestByMergeCommit(context.Context, string) (int, error) {
	return 0, errors.New("adotest: pull request queries are not implemented")
}

// ListPRLabels is not implemented for tag workflow tests.
func (c *Client) ListPRLabels(context.Context, int) ([]string, error) {
	return nil, errors.New("adotest: pull request labels are not implemented")
}

// AddPRLabel is not implemented for tag workflow tests.
func (c *Client) AddPRLabel(context.Context, int, string) error {
	return errors.New("adotest: pull request labels are not implemented")
}

func (c *Client) ensureRefs() {
	if c.refs == nil {
		c.refs = make(map[string]ado.Ref)
	}
	if c.nextObject == 0 {
		c.nextObject = 1
	}
}

func (c *Client) nextTagObjectID() string {
	value := fmt.Sprintf("%040x", c.nextObject)
	c.nextObject++
	return value
}

func normalizeTagRef(name string) string {
	trimmed := strings.TrimSpace(name)
	if strings.HasPrefix(trimmed, tagRefPrefix) {
		return trimmed
	}
	return tagRefPrefix + trimmed
}
