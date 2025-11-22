package github

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"gopkg.in/h2non/gock.v1"
)

func TestMain(m *testing.M) {
	gock.DisableNetworking()
	os.Exit(m.Run())
}

// testClient creates a new client for testing with sensible defaults.
func testClient(t *testing.T) *Client {
	t.Helper()
	client, err := NewClient(ClientOptions{
		AuthToken:    "fake-token",
		DisableCache: true,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return client
}

// assertMocksCalled registers cleanup to disable gock and verify all mocks were called.
func assertMocksCalled(t *testing.T) {
	t.Helper()
	t.Cleanup(gock.Off)
	t.Cleanup(func() {
		if !gock.IsDone() {
			t.Errorf("not all mocks were called: %v", gock.Pending())
		}
	})
}

// mockOwnerType sets up a gock mock for the GET /users/{username} endpoint.
func mockOwnerType(username, ownerType string) {
	gock.New("https://api.github.com").
		Get("/users/" + username).
		Reply(200).
		JSON(fmt.Sprintf(`{"type": %q, "login": %q}`, ownerType, username))
}

// assertError checks if an error matches expectations and reports failure.
func assertError(t *testing.T, err error, wantErr bool, operation string) bool {
	t.Helper()
	if (err != nil) != wantErr {
		t.Errorf("%s error = %v, wantErr %v", operation, err, wantErr)
		return false
	}
	return true
}

// repoFields contains fields for building repository JSON.
type repoFields struct {
	name      string
	branch    string
	size      int
	fork      bool
	archived  bool
	mirrorURL string
}

// repoJSON creates a JSON string for a repository with the given owner and fields.
func repoJSON(owner string, fields repoFields) string {
	return fmt.Sprintf(
		`{"name": %q, "full_name": %q, "owner": {"login": %q}, "default_branch": %q, "size": %d, "fork": %t, "archived": %t, "mirror_url": %q}`,
		fields.name, owner+"/"+fields.name, owner, fields.branch, fields.size, fields.fork, fields.archived, fields.mirrorURL,
	)
}

// reposJSON creates a JSON array of repositories for the given owner.
func reposJSON(owner string, repos ...repoFields) string {
	if len(repos) == 0 {
		return "[]"
	}
	parts := make([]string, len(repos))
	for i, r := range repos {
		parts[i] = repoJSON(owner, r)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// generateRepoPage creates a JSON array of N repositories for testing pagination.
func generateRepoPage(owner string, startNum, count int) string {
	repos := make([]repoFields, count)
	for i := range count {
		repoNum := startNum + i
		repos[i] = repoFields{
			name:   fmt.Sprintf("repo%d", repoNum),
			branch: "main",
			size:   1024,
		}
	}
	return reposJSON(owner, repos...)
}

// Common test data for filter tests.
var (
	sourceRepo = repoFields{name: "source-repo", branch: "main", size: 1024}
	forkRepo   = repoFields{name: "fork-repo", branch: "main", size: 1024, fork: true}
	mirrorRepo = repoFields{name: "mirror-repo", branch: "main", size: 1024, mirrorURL: "https://example.com/repo.git"}
)

// TestNewClient tests client initialization with various options.
func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		opts    ClientOptions
		wantErr bool
	}{
		{
			name: "default options",
			opts: ClientOptions{
				AuthToken:    "fake-token",
				CacheDir:     "",
				CacheTTL:     24 * time.Hour,
				DisableCache: false,
			},
			wantErr: false,
		},
		{
			name: "cache disabled",
			opts: ClientOptions{
				AuthToken:    "fake-token",
				CacheDir:     "",
				CacheTTL:     0,
				DisableCache: true,
			},
			wantErr: false,
		},
		{
			name: "custom cache directory",
			opts: ClientOptions{
				AuthToken:    "fake-token",
				CacheDir:     "/tmp/test-cache",
				CacheTTL:     time.Hour,
				DisableCache: false,
			},
			wantErr: false,
		},
		{
			name: "custom cache TTL",
			opts: ClientOptions{
				AuthToken:    "fake-token",
				CacheDir:     "",
				CacheTTL:     30 * time.Minute,
				DisableCache: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.opts)
			if !assertError(t, err, tt.wantErr, "NewClient()") {
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client")
			}
		})
	}
}

// TestMapRepoTypes tests the internal mapRepoTypes function.
func TestMapRepoTypes(t *testing.T) {
	tests := []struct {
		name      string
		repoTypes RepoTypes
		ownerType OwnerType
		want      string
	}{
		// Sources
		{
			name:      "sources for user",
			repoTypes: RepoTypes{Sources: true},
			ownerType: OwnerTypeUser,
			want:      "owner",
		},
		{
			name:      "sources for organization",
			repoTypes: RepoTypes{Sources: true},
			ownerType: OwnerTypeOrganization,
			want:      "sources",
		},

		// Forks
		{
			name:      "forks for user (not supported)",
			repoTypes: RepoTypes{Forks: true},
			ownerType: OwnerTypeUser,
			want:      "all",
		},
		{
			name:      "forks for organization",
			repoTypes: RepoTypes{Forks: true},
			ownerType: OwnerTypeOrganization,
			want:      "forks",
		},

		// All
		{
			name:      "all for user (empty struct)",
			repoTypes: RepoTypes{},
			ownerType: OwnerTypeUser,
			want:      "all",
		},
		{
			name:      "all for organization (empty struct)",
			repoTypes: RepoTypes{},
			ownerType: OwnerTypeOrganization,
			want:      "all",
		},

		// Archives (not supported by API)
		{
			name:      "archives for user (not supported)",
			repoTypes: RepoTypes{Archives: true},
			ownerType: OwnerTypeUser,
			want:      "all",
		},
		{
			name:      "archives for organization (not supported)",
			repoTypes: RepoTypes{Archives: true},
			ownerType: OwnerTypeOrganization,
			want:      "all",
		},

		// Mirrors (not supported by API)
		{
			name:      "mirrors for user (not supported)",
			repoTypes: RepoTypes{Mirrors: true},
			ownerType: OwnerTypeUser,
			want:      "all",
		},
		{
			name:      "mirrors for organization (not supported)",
			repoTypes: RepoTypes{Mirrors: true},
			ownerType: OwnerTypeOrganization,
			want:      "all",
		},

		// Multiple types (fallback to all)
		{
			name:      "multiple types for user",
			repoTypes: RepoTypes{Sources: true, Forks: true},
			ownerType: OwnerTypeUser,
			want:      "all",
		},
		{
			name:      "multiple types for organization",
			repoTypes: RepoTypes{Sources: true, Forks: true},
			ownerType: OwnerTypeOrganization,
			want:      "all",
		},

		// Empty struct
		{
			name:      "empty repo types",
			repoTypes: RepoTypes{},
			ownerType: OwnerTypeUser,
			want:      "all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapRepoTypes(tt.repoTypes, tt.ownerType)
			if got != tt.want {
				t.Errorf("mapRepoTypes() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetOwnerType tests owner type detection.
func TestGetOwnerType(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		mockStatus int
		mockBody   string
		want       OwnerType
		wantErr    bool
	}{
		{
			name:       "user account",
			username:   "octocat",
			mockStatus: 200,
			mockBody:   `{"type": "User", "login": "octocat"}`,
			want:       OwnerTypeUser,
			wantErr:    false,
		},
		{
			name:       "organization account",
			username:   "github",
			mockStatus: 200,
			mockBody:   `{"type": "Organization", "login": "github"}`,
			want:       OwnerTypeOrganization,
			wantErr:    false,
		},
		{
			name:       "not found",
			username:   "nonexistent",
			mockStatus: 404,
			mockBody:   `{"message": "Not Found"}`,
			wantErr:    true,
		},
		{
			name:       "server error",
			username:   "error",
			mockStatus: 500,
			mockBody:   `{"message": "Internal Server Error"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMocksCalled(t)

			gock.New("https://api.github.com").
				Get("/users/" + tt.username).
				Reply(tt.mockStatus).
				JSON(tt.mockBody)

			client := testClient(t)

			got, err := client.GetOwnerType(context.Background(), tt.username)
			if !assertError(t, err, tt.wantErr, "GetOwnerType()") {
				return
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("GetOwnerType() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetOwnerType_ContextCanceled tests context cancellation.
func TestGetOwnerType_ContextCanceled(t *testing.T) {
	assertMocksCalled(t)

	gock.New("https://api.github.com").
		Get("/users/octocat").
		Reply(200).
		JSON(`{"type": "User"}`)

	client := testClient(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.GetOwnerType(ctx, "octocat")
	if err == nil {
		t.Error("expected context canceled error")
	}
}

// TestListRepos tests repository listing with pagination and filtering.
func TestListRepos(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		repoTypes     RepoTypes
		mockOwnerType string
		mockPages     []string // JSON for each page
		wantRepoCount int
		wantRepoNames []string // Optional: check specific repo names
		wantErr       bool
	}{
		{
			name:          "partial page",
			username:      "octocat",
			repoTypes:     RepoTypes{Sources: true},
			mockOwnerType: "User",
			mockPages:     []string{reposJSON("octocat", repoFields{name: "repo1", branch: "main", size: 1024})},
			wantRepoCount: 1,
		},
		{
			name:          "empty result",
			username:      "emptyuser",
			repoTypes:     RepoTypes{Sources: true},
			mockOwnerType: "User",
			mockPages:     []string{reposJSON("emptyuser")},
			wantRepoCount: 0,
		},
		{
			name:          "pagination",
			username:      "manyrepos",
			repoTypes:     RepoTypes{Sources: true},
			mockOwnerType: "User",
			mockPages: []string{
				generateRepoPage("manyrepos", 1, pageSize),
				reposJSON("manyrepos", repoFields{name: "repo101", branch: "main", size: 1024}),
			},
			wantRepoCount: pageSize + 1,
		},
		{
			name:          "filter sources only - excludes forks and mirrors",
			username:      "filtertest",
			repoTypes:     RepoTypes{Sources: true},
			mockOwnerType: "User",
			mockPages:     []string{reposJSON("filtertest", sourceRepo, forkRepo, mirrorRepo)},
			wantRepoCount: 1,
			wantRepoNames: []string{"source-repo"},
		},
		{
			name:          "filter forks only - excludes sources and mirrors",
			username:      "filtertest",
			repoTypes:     RepoTypes{Forks: true},
			mockOwnerType: "User",
			mockPages:     []string{reposJSON("filtertest", sourceRepo, forkRepo, mirrorRepo)},
			wantRepoCount: 1,
			wantRepoNames: []string{"fork-repo"},
		},
		{
			name:          "filter mirrors only - excludes sources and forks",
			username:      "filtertest",
			repoTypes:     RepoTypes{Mirrors: true},
			mockOwnerType: "User",
			mockPages:     []string{reposJSON("filtertest", sourceRepo, forkRepo, mirrorRepo)},
			wantRepoCount: 1,
			wantRepoNames: []string{"mirror-repo"},
		},
		{
			name:          "filter sources with archives - includes archived sources",
			username:      "filtertest",
			repoTypes:     RepoTypes{Sources: true, Archives: true},
			mockOwnerType: "User",
			mockPages: []string{
				reposJSON("filtertest",
					repoFields{name: "active-source", branch: "main", size: 1024},
					repoFields{name: "archived-source", branch: "main", size: 1024, archived: true},
					repoFields{name: "active-fork", branch: "main", size: 1024, fork: true},
					repoFields{name: "archived-fork", branch: "main", size: 1024, fork: true, archived: true},
				),
			},
			wantRepoCount: 2,
			wantRepoNames: []string{"active-source", "archived-source"},
		},
		{
			name:          "filter sources without archives - excludes archived sources",
			username:      "filtertest",
			repoTypes:     RepoTypes{Sources: true},
			mockOwnerType: "User",
			mockPages: []string{
				reposJSON("filtertest",
					repoFields{name: "active-source", branch: "main", size: 1024},
					repoFields{name: "archived-source", branch: "main", size: 1024, archived: true},
				),
			},
			wantRepoCount: 1,
			wantRepoNames: []string{"active-source"},
		},
		{
			name:          "filter forks with archives - includes archived forks",
			username:      "filtertest",
			repoTypes:     RepoTypes{Forks: true, Archives: true},
			mockOwnerType: "User",
			mockPages: []string{
				reposJSON("filtertest",
					repoFields{name: "active-source", branch: "main", size: 1024},
					repoFields{name: "active-fork", branch: "main", size: 1024, fork: true},
					repoFields{name: "archived-fork", branch: "main", size: 1024, fork: true, archived: true},
				),
			},
			wantRepoCount: 2,
			wantRepoNames: []string{"active-fork", "archived-fork"},
		},
		{
			name:          "filter sources and forks without archives - excludes archived repos",
			username:      "filtertest",
			repoTypes:     RepoTypes{Sources: true, Forks: true},
			mockOwnerType: "User",
			mockPages: []string{
				reposJSON("filtertest",
					repoFields{name: "active-source", branch: "main", size: 1024},
					repoFields{name: "archived-source", branch: "main", size: 1024, archived: true},
					repoFields{name: "active-fork", branch: "main", size: 1024, fork: true},
					repoFields{name: "archived-fork", branch: "main", size: 1024, fork: true, archived: true},
				),
			},
			wantRepoCount: 2,
			wantRepoNames: []string{"active-source", "active-fork"},
		},
		{
			name:          "empty repo types - filters all repos when no types selected",
			username:      "filtertest",
			repoTypes:     RepoTypes{},
			mockOwnerType: "User",
			mockPages: []string{
				reposJSON("filtertest",
					repoFields{name: "source-repo", branch: "main", size: 1024},
					repoFields{name: "fork-repo", branch: "main", size: 1024, fork: true},
				),
			},
			wantRepoCount: 0,
			wantRepoNames: nil,
		},
		{
			name:          "filter empty repositories",
			username:      "filtertest",
			repoTypes:     RepoTypes{Sources: true},
			mockOwnerType: "User",
			mockPages: []string{
				reposJSON("filtertest",
					repoFields{name: "normal-repo", branch: "main", size: 1024},
					repoFields{name: "empty-repo", branch: "main", size: 0},
				),
			},
			wantRepoCount: 1,
			wantRepoNames: []string{"normal-repo"},
		},
		{
			name:          "filter repositories without default branch",
			username:      "filtertest",
			repoTypes:     RepoTypes{Sources: true},
			mockOwnerType: "User",
			mockPages: []string{
				reposJSON("filtertest",
					repoFields{name: "normal-repo", branch: "main", size: 1024},
					repoFields{name: "no-branch-repo", branch: "", size: 1024},
				),
			},
			wantRepoCount: 1,
			wantRepoNames: []string{"normal-repo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMocksCalled(t)

			// Mock owner type check
			mockOwnerType(tt.username, tt.mockOwnerType)

			// Determine endpoint based on owner type
			var endpoint string
			if tt.mockOwnerType == "Organization" {
				endpoint = "/orgs/" + tt.username + "/repos"
			} else {
				endpoint = "/users/" + tt.username + "/repos"
			}

			// Mock paginated responses
			for i, pageBody := range tt.mockPages {
				page := i + 1
				gock.New("https://api.github.com").
					Get(endpoint).
					MatchParam("page", fmt.Sprintf("%d", page)).
					MatchParam("per_page", fmt.Sprintf("%d", pageSize)).
					Reply(200).
					JSON(pageBody)
			}

			client := testClient(t)

			repos, err := client.ListRepos(context.Background(), tt.username, tt.repoTypes)
			if !assertError(t, err, tt.wantErr, "ListRepos()") {
				return
			}

			if !tt.wantErr && len(repos) != tt.wantRepoCount {
				t.Errorf("ListRepos() returned %d repos, want %d", len(repos), tt.wantRepoCount)
			}

			// If specific repo names are provided, verify them
			if !tt.wantErr && len(tt.wantRepoNames) > 0 {
				gotNames := make([]string, len(repos))
				for i, repo := range repos {
					gotNames[i] = repo.Name
				}
				slices.Sort(gotNames)
				wantNames := slices.Clone(tt.wantRepoNames)
				slices.Sort(wantNames)
				if !slices.Equal(gotNames, wantNames) {
					t.Errorf("ListRepos() repo names = %v, want %v", gotNames, wantNames)
				}
			}
		})
	}
}

// TestGetRepo tests fetching a single repository.
func TestGetRepo(t *testing.T) {
	tests := []struct {
		name       string
		owner      string
		repo       string
		mockStatus int
		mockBody   string
		wantErr    bool
	}{
		{
			name:       "valid repository",
			owner:      "octocat",
			repo:       "Hello-World",
			mockStatus: 200,
			mockBody: `{
				"name": "Hello-World",
				"full_name": "octocat/Hello-World",
				"owner": {"login": "octocat"},
				"default_branch": "main",
				"size": 1024,
				"fork": false,
				"archived": false,
				"mirror_url": ""
			}`,
			wantErr: false,
		},
		{
			name:       "repository not found",
			owner:      "octocat",
			repo:       "nonexistent",
			mockStatus: 404,
			mockBody:   `{"message": "Not Found"}`,
			wantErr:    true,
		},
		{
			name:       "private repository forbidden",
			owner:      "octocat",
			repo:       "private",
			mockStatus: 403,
			mockBody:   `{"message": "Forbidden"}`,
			wantErr:    true,
		},
		{
			name:       "empty repository",
			owner:      "octocat",
			repo:       "empty-repo",
			mockStatus: 200,
			mockBody: `{
				"name": "empty-repo",
				"full_name": "octocat/empty-repo",
				"owner": {"login": "octocat"},
				"default_branch": "main",
				"size": 0,
				"fork": false,
				"archived": false,
				"mirror_url": ""
			}`,
			wantErr: true,
		},
		{
			name:       "repository without default branch",
			owner:      "octocat",
			repo:       "no-branch-repo",
			mockStatus: 200,
			mockBody: `{
				"name": "no-branch-repo",
				"full_name": "octocat/no-branch-repo",
				"owner": {"login": "octocat"},
				"default_branch": "",
				"size": 1024,
				"fork": false,
				"archived": false,
				"mirror_url": ""
			}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMocksCalled(t)

			gock.New("https://api.github.com").
				Get("/repos/" + tt.owner + "/" + tt.repo).
				Reply(tt.mockStatus).
				JSON(tt.mockBody)

			client := testClient(t)

			repo, err := client.GetRepo(context.Background(), tt.owner, tt.repo)
			if !assertError(t, err, tt.wantErr, "GetRepo()") {
				return
			}

			if !tt.wantErr {
				if repo.Name != tt.repo {
					t.Errorf("GetRepo() repo.Name = %v, want %v", repo.Name, tt.repo)
				}
				if repo.Owner != tt.owner {
					t.Errorf("GetRepo() repo.Owner = %v, want %v", repo.Owner, tt.owner)
				}
			}
		})
	}
}

// TestGetTree tests fetching Git trees.
func TestGetTree(t *testing.T) {
	tests := []struct {
		name          string
		repo          Repository
		mockStatus    int
		mockBody      string
		wantTruncated bool
		wantTreeSize  int
		wantErr       bool
	}{
		{
			name: "small tree",
			repo: Repository{
				Owner: "octocat",
				Name:  "Hello-World",
				Ref:   "main",
			},
			mockStatus: 200,
			mockBody: `{
				"sha": "abc123",
				"url": "https://api.github.com/repos/octocat/Hello-World/git/trees/abc123",
				"tree": [
					{"path": "README.md", "mode": "100644", "type": "blob", "sha": "def456", "size": 1234},
					{"path": "main.go", "mode": "100644", "type": "blob", "sha": "ghi789", "size": 5678}
				],
				"truncated": false
			}`,
			wantTruncated: false,
			wantTreeSize:  2,
		},
		{
			name: "truncated tree",
			repo: Repository{
				Owner: "octocat",
				Name:  "huge-repo",
				Ref:   "main",
			},
			mockStatus: 200,
			mockBody: `{
				"sha": "abc123",
				"url": "https://api.github.com/repos/octocat/huge-repo/git/trees/abc123",
				"tree": [
					{"path": "file1.txt", "mode": "100644", "type": "blob", "sha": "def456", "size": 100}
				],
				"truncated": true
			}`,
			wantTruncated: true,
			wantTreeSize:  1,
		},
		{
			name: "empty repository",
			repo: Repository{
				Owner: "octocat",
				Name:  "empty-repo",
				Ref:   "main",
			},
			mockStatus: 200,
			mockBody: `{
				"sha": "abc123",
				"url": "https://api.github.com/repos/octocat/empty-repo/git/trees/abc123",
				"tree": [],
				"truncated": false
			}`,
			wantTruncated: false,
			wantTreeSize:  0,
		},
		{
			name: "invalid branch",
			repo: Repository{
				Owner: "octocat",
				Name:  "repo",
				Ref:   "nonexistent",
			},
			mockStatus:    404,
			mockBody:      `{"message": "Not Found"}`,
			wantTruncated: false,
			wantTreeSize:  0,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMocksCalled(t)

			gock.New("https://api.github.com").
				Get("/repos/"+tt.repo.Owner+"/"+tt.repo.Name+"/git/trees/"+tt.repo.Ref).
				MatchParam("recursive", "1").
				Reply(tt.mockStatus).
				JSON(tt.mockBody)

			client := testClient(t)

			tree, err := client.GetTree(context.Background(), tt.repo)
			if !assertError(t, err, tt.wantErr, "GetTree()") {
				return
			}

			if !tt.wantErr {
				if tree.Truncated != tt.wantTruncated {
					t.Errorf("GetTree() truncated = %v, want %v", tree.Truncated, tt.wantTruncated)
				}
				if len(tree.Tree) != tt.wantTreeSize {
					t.Errorf("GetTree() tree size = %d, want %d", len(tree.Tree), tt.wantTreeSize)
				}
			}
		})
	}
}
