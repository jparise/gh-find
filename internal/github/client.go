// Package github provides GitHub API client functionality for gh-find.
package github

import (
	"context"
	"fmt"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// OwnerType represents the type of account owner (User or Organization).
type OwnerType string

const (
	// OwnerTypeUser represents a user account.
	OwnerTypeUser OwnerType = "User"
	// OwnerTypeOrganization represents an organization account.
	OwnerTypeOrganization OwnerType = "Organization"

	pageSize = 100
)

// ClientOptions configures the GitHub API client.
type ClientOptions struct {
	AuthToken    string
	CacheDir     string
	CacheTTL     time.Duration
	DisableCache bool
}

// Client wraps the go-gh REST client.
type Client struct {
	rest *api.RESTClient
}

// NewClient creates a new GitHub API client with the given options.
func NewClient(opts ClientOptions) (*Client, error) {
	apiOpts := api.ClientOptions{
		AuthToken:   opts.AuthToken,
		CacheDir:    opts.CacheDir,
		CacheTTL:    opts.CacheTTL,
		EnableCache: !opts.DisableCache,
	}

	rest, err := api.NewRESTClient(apiOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	return &Client{
		rest: rest,
	}, nil
}

// GetOwnerType determines if a name is a "User" or "Organization".
func (c *Client) GetOwnerType(ctx context.Context, name string) (OwnerType, error) {
	var result struct {
		Type OwnerType `json:"type"`
	}

	endpoint := fmt.Sprintf("users/%s", name)
	err := c.rest.DoWithContext(ctx, "GET", endpoint, nil, &result)
	if err != nil {
		return "", fmt.Errorf("failed to get owner type for %s: %w", name, err)
	}

	return result.Type, nil
}

// ListRepos returns all repositories for a user or organization with pagination.
// It detects whether the name is a user or org and uses the appropriate endpoint.
func (c *Client) ListRepos(ctx context.Context, name string, types RepoTypes) ([]Repository, error) {
	// Detect if this is a user or organization
	accountType, err := c.GetOwnerType(ctx, name)
	if err != nil {
		return nil, err
	}

	var allRepos []Repository
	page := 1
	perPage := pageSize

	// Determine the base endpoint based on account type
	var baseEndpoint string
	if accountType == OwnerTypeOrganization {
		baseEndpoint = fmt.Sprintf("orgs/%s/repos", name)
	} else {
		baseEndpoint = fmt.Sprintf("users/%s/repos", name)
	}

	typeParam := mapRepoTypes(types, accountType)

	for {
		endpoint := fmt.Sprintf("%s?type=%s&per_page=%d&page=%d",
			baseEndpoint, typeParam, perPage, page)

		var repos []Repository
		err := c.rest.DoWithContext(ctx, "GET", endpoint, nil, &repos)
		if err != nil {
			return nil, fmt.Errorf("failed to list repos for %s: %w", name, err)
		}

		if len(repos) == 0 {
			break
		}

		allRepos = append(allRepos, repos...)

		// Check if there are more pages
		if len(repos) < perPage {
			break
		}
		page++
	}

	// Apply client-side filtering for repo types to cover the cases that
	// aren't natively supported by the GitHub API.
	filtered := make([]Repository, 0, len(allRepos))
	for _, repo := range allRepos {
		if repo.Size == 0 {
			continue
		}

		if repo.Archived && !types.Archives {
			continue
		}

		var shouldInclude bool
		switch {
		case repo.Fork:
			shouldInclude = types.Forks
		case repo.MirrorURL != "":
			shouldInclude = types.Mirrors
		default:
			shouldInclude = types.Sources
		}
		if shouldInclude {
			filtered = append(filtered, repo)
		}
	}

	return filtered, nil
}

// repoTypeAPIParams maps repository types to their GitHub API type parameter
// for each owner type. Missing entries default to "all" (fetch all, filter client-side).
//
// Support matrix:
//
//	Sources:  orgs="sources", users="owner"
//	Forks:    orgs="forks",   users=not supported
//	Archives: not supported (filter client-side)
//	Mirrors:  not supported (filter client-side)
var repoTypeAPIParams = map[RepoType]map[OwnerType]string{
	RepoTypeSources: {
		OwnerTypeOrganization: "sources",
		OwnerTypeUser:         "owner",
	},
	RepoTypeForks: {
		OwnerTypeOrganization: "forks",
	},
}

// mapRepoTypes returns the GitHub API type parameter for filtering repositories.
// Returns "all" if the API doesn't support filtering the requested type(s).
func mapRepoTypes(types RepoTypes, ownerType OwnerType) string {
	var selected []RepoType

	if types.Sources {
		selected = append(selected, RepoTypeSources)
	}
	if types.Forks {
		selected = append(selected, RepoTypeForks)
	}
	if types.Archives {
		selected = append(selected, RepoTypeArchives)
	}
	if types.Mirrors {
		selected = append(selected, RepoTypeMirrors)
	}

	// If only a single type is selected, attempt to map it to an API `type`
	// parameter value as a server-side filtering optimization.
	if len(selected) == 1 {
		repoType := selected[0]
		if params, ok := repoTypeAPIParams[repoType]; ok {
			if apiParam, ok := params[ownerType]; ok {
				return apiParam
			}
		}
	}

	return "all"
}

// GetRepo fetches a single repository.
func (c *Client) GetRepo(ctx context.Context, owner, repo string) (Repository, error) {
	var result Repository

	endpoint := fmt.Sprintf("repos/%s/%s", owner, repo)
	err := c.rest.DoWithContext(ctx, "GET", endpoint, nil, &result)
	if err != nil {
		return Repository{}, fmt.Errorf("failed to get repo %s/%s: %w", owner, repo, err)
	}
	if result.Size == 0 {
		return Repository{}, fmt.Errorf("repository is empty (no commits yet)")
	}

	return result, nil
}

// GetTree fetches the Git tree for a repository recursively.
func (c *Client) GetTree(ctx context.Context, repo Repository) (*TreeResponse, error) {
	var tree TreeResponse

	// Fetch the tree for the default branch with recursive flag
	endpoint := fmt.Sprintf("repos/%s/%s/git/trees/%s?recursive=1",
		repo.Owner, repo.Name, repo.DefaultBranch)

	err := c.rest.DoWithContext(ctx, "GET", endpoint, nil, &tree)
	if err != nil {
		return nil, fmt.Errorf("failed to get tree for %s: %w", repo.FullName, err)
	}

	return &tree, nil
}
