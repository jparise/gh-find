package github

import (
	"context"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"gopkg.in/h2non/gock.v1"
)

func TestMain(m *testing.M) {
	// Disable real HTTP requests during tests
	gock.DisableNetworking()
	os.Exit(m.Run())
}

// generateRepoPage creates a JSON array of N repositories for testing pagination.
func generateRepoPage(owner string, startNum, count int) string {
	repos := make([]string, count)
	for i := range count {
		repoNum := startNum + i
		//nolint:gocritic // JSON template requires literal quoted strings
		repos[i] = fmt.Sprintf(`{"name": "repo%d", "full_name": "%s/repo%d", "owner": {"login": "%s"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": ""}`,
			repoNum, owner, repoNum, owner)
	}
	result := "[" + repos[0]
	for i := 1; i < len(repos); i++ {
		result += "," + repos[i]
	}
	return result + "]"
}

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
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
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
			t.Cleanup(gock.Off)

			gock.New("https://api.github.com").
				Get("/users/" + tt.username).
				Reply(tt.mockStatus).
				JSON(tt.mockBody)

			client, err := NewClient(ClientOptions{
				AuthToken:    "fake-token",
				DisableCache: true,
			})
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			got, err := client.GetOwnerType(context.Background(), tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetOwnerType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("GetOwnerType() = %v, want %v", got, tt.want)
			}

			if !gock.IsDone() {
				t.Errorf("not all mocks were called: %v", gock.Pending())
			}
		})
	}
}

// TestGetOwnerType_ContextCanceled tests context cancellation.
func TestGetOwnerType_ContextCanceled(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Get("/users/octocat").
		Reply(200).
		JSON(`{"type": "User"}`)

	client, err := NewClient(ClientOptions{
		AuthToken:    "fake-token",
		DisableCache: true,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = client.GetOwnerType(ctx, "octocat")
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
			name:          "user with partial page",
			username:      "octocat",
			repoTypes:     RepoTypes{Sources: true}, // Default: sources only
			mockOwnerType: "User",
			mockPages: []string{
				// Only 1 repo (less than 100) - pagination stops after this
				`[{"name": "repo1", "full_name": "octocat/repo1", "owner": {"login": "octocat"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": ""}]`,
			},
			wantRepoCount: 1,
			wantErr:       false,
		},
		{
			name:          "organization with partial page",
			username:      "github",
			repoTypes:     RepoTypes{Sources: true}, // Default: sources only
			mockOwnerType: "Organization",
			mockPages: []string{
				// Only 1 repo (less than 100) - pagination stops after this
				`[{"name": "repo1", "full_name": "github/repo1", "owner": {"login": "github"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": ""}]`,
			},
			wantRepoCount: 1,
			wantErr:       false,
		},
		{
			name:          "empty result",
			username:      "emptyuser",
			repoTypes:     RepoTypes{Sources: true}, // Default: sources only
			mockOwnerType: "User",
			mockPages: []string{
				`[]`,
			},
			wantRepoCount: 0,
			wantErr:       false,
		},
		{
			name:          "full page triggers pagination",
			username:      "manyrepos",
			repoTypes:     RepoTypes{Sources: true}, // Default: sources only
			mockOwnerType: "User",
			mockPages: []string{
				// First page: exactly pageSize repos (full page)
				generateRepoPage("manyrepos", 1, pageSize),
				// Second page: partial (triggers page++ and then stops)
				`[{"name": "repo101", "full_name": "manyrepos/repo101", "owner": {"login": "manyrepos"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": ""}]`,
			},
			wantRepoCount: pageSize + 1,
			wantErr:       false,
		},
		{
			name:          "filter sources only - excludes forks and mirrors",
			username:      "filtertest",
			repoTypes:     RepoTypes{Sources: true},
			mockOwnerType: "User",
			mockPages: []string{
				`[
					{"name": "source-repo", "full_name": "filtertest/source-repo", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": ""},
					{"name": "fork-repo", "full_name": "filtertest/fork-repo", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": true, "archived": false, "mirror_url": ""},
					{"name": "mirror-repo", "full_name": "filtertest/mirror-repo", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": "https://example.com/repo.git"}
				]`,
			},
			wantRepoCount: 1,
			wantRepoNames: []string{"source-repo"},
			wantErr:       false,
		},
		{
			name:          "filter forks only - excludes sources and mirrors",
			username:      "filtertest",
			repoTypes:     RepoTypes{Forks: true},
			mockOwnerType: "User",
			mockPages: []string{
				`[
					{"name": "source-repo", "full_name": "filtertest/source-repo", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": ""},
					{"name": "fork-repo", "full_name": "filtertest/fork-repo", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": true, "archived": false, "mirror_url": ""},
					{"name": "mirror-repo", "full_name": "filtertest/mirror-repo", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": "https://example.com/repo.git"}
				]`,
			},
			wantRepoCount: 1,
			wantRepoNames: []string{"fork-repo"},
			wantErr:       false,
		},
		{
			name:          "filter mirrors only - excludes sources and forks",
			username:      "filtertest",
			repoTypes:     RepoTypes{Mirrors: true},
			mockOwnerType: "User",
			mockPages: []string{
				`[
					{"name": "source-repo", "full_name": "filtertest/source-repo", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": ""},
					{"name": "fork-repo", "full_name": "filtertest/fork-repo", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": true, "archived": false, "mirror_url": ""},
					{"name": "mirror-repo", "full_name": "filtertest/mirror-repo", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": "https://example.com/repo.git"}
				]`,
			},
			wantRepoCount: 1,
			wantRepoNames: []string{"mirror-repo"},
			wantErr:       false,
		},
		{
			name:          "filter sources with archives - includes archived sources",
			username:      "filtertest",
			repoTypes:     RepoTypes{Sources: true, Archives: true},
			mockOwnerType: "User",
			mockPages: []string{
				`[
					{"name": "active-source", "full_name": "filtertest/active-source", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": ""},
					{"name": "archived-source", "full_name": "filtertest/archived-source", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": true, "mirror_url": ""},
					{"name": "active-fork", "full_name": "filtertest/active-fork", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": true, "archived": false, "mirror_url": ""},
					{"name": "archived-fork", "full_name": "filtertest/archived-fork", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": true, "archived": true, "mirror_url": ""}
				]`,
			},
			wantRepoCount: 2,
			wantRepoNames: []string{"active-source", "archived-source"},
			wantErr:       false,
		},
		{
			name:          "filter sources without archives - excludes archived sources",
			username:      "filtertest",
			repoTypes:     RepoTypes{Sources: true},
			mockOwnerType: "User",
			mockPages: []string{
				`[
					{"name": "active-source", "full_name": "filtertest/active-source", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": ""},
					{"name": "archived-source", "full_name": "filtertest/archived-source", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": true, "mirror_url": ""}
				]`,
			},
			wantRepoCount: 1,
			wantRepoNames: []string{"active-source"},
			wantErr:       false,
		},
		{
			name:          "filter forks with archives - includes archived forks",
			username:      "filtertest",
			repoTypes:     RepoTypes{Forks: true, Archives: true},
			mockOwnerType: "User",
			mockPages: []string{
				`[
					{"name": "active-source", "full_name": "filtertest/active-source", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": ""},
					{"name": "active-fork", "full_name": "filtertest/active-fork", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": true, "archived": false, "mirror_url": ""},
					{"name": "archived-fork", "full_name": "filtertest/archived-fork", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": true, "archived": true, "mirror_url": ""}
				]`,
			},
			wantRepoCount: 2,
			wantRepoNames: []string{"active-fork", "archived-fork"},
			wantErr:       false,
		},
		{
			name:          "filter sources and forks without archives - excludes archived repos",
			username:      "filtertest",
			repoTypes:     RepoTypes{Sources: true, Forks: true},
			mockOwnerType: "User",
			mockPages: []string{
				`[
					{"name": "active-source", "full_name": "filtertest/active-source", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": ""},
					{"name": "archived-source", "full_name": "filtertest/archived-source", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": true, "mirror_url": ""},
					{"name": "active-fork", "full_name": "filtertest/active-fork", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": true, "archived": false, "mirror_url": ""},
					{"name": "archived-fork", "full_name": "filtertest/archived-fork", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": true, "archived": true, "mirror_url": ""}
				]`,
			},
			wantRepoCount: 2,
			wantRepoNames: []string{"active-source", "active-fork"},
			wantErr:       false,
		},
		{
			name:          "empty repo types - returns nothing",
			username:      "filtertest",
			repoTypes:     RepoTypes{},
			mockOwnerType: "User",
			mockPages: []string{
				`[
					{"name": "source-repo", "full_name": "filtertest/source-repo", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": ""},
					{"name": "fork-repo", "full_name": "filtertest/fork-repo", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": true, "archived": false, "mirror_url": ""}
				]`,
			},
			wantRepoCount: 0,
			wantRepoNames: nil,
			wantErr:       false,
		},
		{
			name:          "filter empty repositories",
			username:      "filtertest",
			repoTypes:     RepoTypes{Sources: true},
			mockOwnerType: "User",
			mockPages: []string{
				`[
					{"name": "normal-repo", "full_name": "filtertest/normal-repo", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": ""},
					{"name": "empty-repo", "full_name": "filtertest/empty-repo", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 0, "fork": false, "archived": false, "mirror_url": ""}
				]`,
			},
			wantRepoCount: 1,
			wantRepoNames: []string{"normal-repo"},
			wantErr:       false,
		},
		{
			name:          "filter repositories without default branch",
			username:      "filtertest",
			repoTypes:     RepoTypes{Sources: true},
			mockOwnerType: "User",
			mockPages: []string{
				`[
					{"name": "normal-repo", "full_name": "filtertest/normal-repo", "owner": {"login": "filtertest"}, "default_branch": "main", "size": 1024, "fork": false, "archived": false, "mirror_url": ""},
					{"name": "no-branch-repo", "full_name": "filtertest/no-branch-repo", "owner": {"login": "filtertest"}, "default_branch": "", "size": 1024, "fork": false, "archived": false, "mirror_url": ""}
				]`,
			},
			wantRepoCount: 1,
			wantRepoNames: []string{"normal-repo"},
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(gock.Off)

			// Mock owner type check
			gock.New("https://api.github.com").
				Get("/users/" + tt.username).
				Reply(200).
				JSON(`{"type": "` + tt.mockOwnerType + `", "login": "` + tt.username + `"}`)

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

			client, err := NewClient(ClientOptions{
				AuthToken:    "fake-token",
				DisableCache: true,
			})
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			repos, err := client.ListRepos(context.Background(), tt.username, tt.repoTypes)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListRepos() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(repos) != tt.wantRepoCount {
				t.Errorf("ListRepos() returned %d repos, want %d", len(repos), tt.wantRepoCount)
			}

			// If specific repo names are provided, verify them
			if !tt.wantErr && len(tt.wantRepoNames) > 0 {
				gotNames := make(map[string]bool)
				for _, repo := range repos {
					gotNames[repo.Name] = true
				}

				for _, wantName := range tt.wantRepoNames {
					if !gotNames[wantName] {
						t.Errorf("ListRepos() missing expected repo: %s", wantName)
					}
				}

				for gotName := range gotNames {
					if !slices.Contains(tt.wantRepoNames, gotName) {
						t.Errorf("ListRepos() returned unexpected repo: %s", gotName)
					}
				}
			}

			if !gock.IsDone() {
				t.Errorf("not all mocks were called: %v", gock.Pending())
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
			t.Cleanup(gock.Off)

			gock.New("https://api.github.com").
				Get("/repos/" + tt.owner + "/" + tt.repo).
				Reply(tt.mockStatus).
				JSON(tt.mockBody)

			client, err := NewClient(ClientOptions{
				AuthToken:    "fake-token",
				DisableCache: true,
			})
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			repo, err := client.GetRepo(context.Background(), tt.owner, tt.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRepo() error = %v, wantErr %v", err, tt.wantErr)
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

			if !gock.IsDone() {
				t.Errorf("not all mocks were called: %v", gock.Pending())
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
				Owner:         "octocat",
				Name:          "Hello-World",
				DefaultBranch: "main",
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
			wantErr:       false,
		},
		{
			name: "truncated tree",
			repo: Repository{
				Owner:         "octocat",
				Name:          "huge-repo",
				DefaultBranch: "main",
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
			wantErr:       false,
		},
		{
			name: "empty repository",
			repo: Repository{
				Owner:         "octocat",
				Name:          "empty-repo",
				DefaultBranch: "main",
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
			wantErr:       false,
		},
		{
			name: "invalid branch",
			repo: Repository{
				Owner:         "octocat",
				Name:          "repo",
				DefaultBranch: "nonexistent",
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
			t.Cleanup(gock.Off)

			gock.New("https://api.github.com").
				Get("/repos/"+tt.repo.Owner+"/"+tt.repo.Name+"/git/trees/"+tt.repo.DefaultBranch).
				MatchParam("recursive", "1").
				Reply(tt.mockStatus).
				JSON(tt.mockBody)

			client, err := NewClient(ClientOptions{
				AuthToken:    "fake-token",
				DisableCache: true,
			})
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			tree, err := client.GetTree(context.Background(), tt.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTree() error = %v, wantErr %v", err, tt.wantErr)
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

			if !gock.IsDone() {
				t.Errorf("not all mocks were called: %v", gock.Pending())
			}
		})
	}
}
