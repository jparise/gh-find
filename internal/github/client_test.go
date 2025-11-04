package github

import (
	"context"
	"fmt"
	"os"
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
		repos[i] = fmt.Sprintf(`{"name": "repo%d", "full_name": "%s/repo%d", "owner": {"login": "%s"}, "default_branch": "main", "fork": false, "archived": false, "mirror_url": ""}`,
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
				DisableCache: false,
				CacheDir:     "",
				CacheTTL:     24 * time.Hour,
			},
			wantErr: false,
		},
		{
			name: "cache disabled",
			opts: ClientOptions{
				DisableCache: true,
				CacheDir:     "",
				CacheTTL:     0,
			},
			wantErr: false,
		},
		{
			name: "custom cache directory",
			opts: ClientOptions{
				DisableCache: false,
				CacheDir:     "/tmp/test-cache",
				CacheTTL:     time.Hour,
			},
			wantErr: false,
		},
		{
			name: "custom cache TTL",
			opts: ClientOptions{
				DisableCache: false,
				CacheDir:     "",
				CacheTTL:     30 * time.Minute,
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
		repoTypes []RepoType
		ownerType OwnerType
		want      string
	}{
		// Sources
		{
			name:      "sources for user",
			repoTypes: []RepoType{RepoTypeSources},
			ownerType: OwnerTypeUser,
			want:      "owner",
		},
		{
			name:      "sources for organization",
			repoTypes: []RepoType{RepoTypeSources},
			ownerType: OwnerTypeOrganization,
			want:      "sources",
		},

		// Forks
		{
			name:      "forks for user (not supported)",
			repoTypes: []RepoType{RepoTypeForks},
			ownerType: OwnerTypeUser,
			want:      "all",
		},
		{
			name:      "forks for organization",
			repoTypes: []RepoType{RepoTypeForks},
			ownerType: OwnerTypeOrganization,
			want:      "forks",
		},

		// All
		{
			name:      "all for user",
			repoTypes: []RepoType{RepoTypeAll},
			ownerType: OwnerTypeUser,
			want:      "all",
		},
		{
			name:      "all for organization",
			repoTypes: []RepoType{RepoTypeAll},
			ownerType: OwnerTypeOrganization,
			want:      "all",
		},

		// Archives (not supported by API)
		{
			name:      "archives for user (not supported)",
			repoTypes: []RepoType{RepoTypeArchives},
			ownerType: OwnerTypeUser,
			want:      "all",
		},
		{
			name:      "archives for organization (not supported)",
			repoTypes: []RepoType{RepoTypeArchives},
			ownerType: OwnerTypeOrganization,
			want:      "all",
		},

		// Mirrors (not supported by API)
		{
			name:      "mirrors for user (not supported)",
			repoTypes: []RepoType{RepoTypeMirrors},
			ownerType: OwnerTypeUser,
			want:      "all",
		},
		{
			name:      "mirrors for organization (not supported)",
			repoTypes: []RepoType{RepoTypeMirrors},
			ownerType: OwnerTypeOrganization,
			want:      "all",
		},

		// Multiple types (fallback to all)
		{
			name:      "multiple types for user",
			repoTypes: []RepoType{RepoTypeSources, RepoTypeForks},
			ownerType: OwnerTypeUser,
			want:      "all",
		},
		{
			name:      "multiple types for organization",
			repoTypes: []RepoType{RepoTypeSources, RepoTypeForks},
			ownerType: OwnerTypeOrganization,
			want:      "all",
		},

		// Empty slice
		{
			name:      "empty repo types",
			repoTypes: []RepoType{},
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

			client, err := NewClient(ClientOptions{DisableCache: true})
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

	client, err := NewClient(ClientOptions{DisableCache: true})
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

// TestListRepos tests repository listing with pagination.
func TestListRepos(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		repoTypes     []RepoType
		mockOwnerType string
		mockPages     []string // JSON for each page
		wantRepoCount int
		wantErr       bool
	}{
		{
			name:          "user with partial page",
			username:      "octocat",
			repoTypes:     []RepoType{RepoTypeAll},
			mockOwnerType: "User",
			mockPages: []string{
				// Only 1 repo (less than 100) - pagination stops after this
				`[{"name": "repo1", "full_name": "octocat/repo1", "owner": {"login": "octocat"}, "default_branch": "main", "fork": false, "archived": false, "mirror_url": ""}]`,
			},
			wantRepoCount: 1,
			wantErr:       false,
		},
		{
			name:          "organization with partial page",
			username:      "github",
			repoTypes:     []RepoType{RepoTypeAll},
			mockOwnerType: "Organization",
			mockPages: []string{
				// Only 1 repo (less than 100) - pagination stops after this
				`[{"name": "repo1", "full_name": "github/repo1", "owner": {"login": "github"}, "default_branch": "main", "fork": false, "archived": false, "mirror_url": ""}]`,
			},
			wantRepoCount: 1,
			wantErr:       false,
		},
		{
			name:          "empty result",
			username:      "emptyuser",
			repoTypes:     []RepoType{RepoTypeAll},
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
			repoTypes:     []RepoType{RepoTypeAll},
			mockOwnerType: "User",
			mockPages: []string{
				// First page: exactly pageSize repos (full page)
				generateRepoPage("manyrepos", 1, pageSize),
				// Second page: partial (triggers page++ and then stops)
				`[{"name": "repo101", "full_name": "manyrepos/repo101", "owner": {"login": "manyrepos"}, "default_branch": "main", "fork": false, "archived": false, "mirror_url": ""}]`,
			},
			wantRepoCount: pageSize + 1,
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

			client, err := NewClient(ClientOptions{DisableCache: true})
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(gock.Off)

			gock.New("https://api.github.com").
				Get("/repos/" + tt.owner + "/" + tt.repo).
				Reply(tt.mockStatus).
				JSON(tt.mockBody)

			client, err := NewClient(ClientOptions{DisableCache: true})
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
		repo          *Repository
		mockStatus    int
		mockBody      string
		wantTruncated bool
		wantTreeSize  int
		wantErr       bool
	}{
		{
			name: "small tree",
			repo: &Repository{
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
			repo: &Repository{
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
			repo: &Repository{
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
			repo: &Repository{
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

			client, err := NewClient(ClientOptions{DisableCache: true})
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
