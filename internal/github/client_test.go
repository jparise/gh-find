package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

const fakeToken = "fake-token"

type mockResponse struct {
	method      string
	path        string
	requestBody string
	status      int
	body        string
}

type mockTransport struct {
	t         *testing.T
	responses []mockResponse
	next      int
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.t.Helper()
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	if m.next == len(m.responses) {
		m.t.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		return nil, fmt.Errorf("unexpected request")
	}

	mock := m.responses[m.next]
	m.next++
	if req.Method != mock.method || req.URL.RequestURI() != mock.path {
		m.t.Errorf("request = %s %s, want %s %s", req.Method, req.URL.RequestURI(), mock.method, mock.path)
	}
	if mock.requestBody != "" {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			m.t.Errorf("read request body: %v", err)
		} else if string(body) != mock.requestBody {
			m.t.Errorf("request body = %s, want %s", body, mock.requestBody)
		}
	}

	return &http.Response{
		StatusCode: mock.status,
		Status:     fmt.Sprintf("%d %s", mock.status, http.StatusText(mock.status)),
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(mock.body)),
		Request:    req,
	}, nil
}

func getResponse(path string, status int, body string) mockResponse {
	return mockResponse{method: http.MethodGet, path: path, status: status, body: body}
}

func graphqlResponse(query string, status int, body string) mockResponse {
	return mockResponse{
		method:      http.MethodPost,
		path:        "/graphql",
		requestBody: fmt.Sprintf(`{"query":%q,"variables":null}`, query),
		status:      status,
		body:        body,
	}
}

func testClient(t *testing.T, responses ...mockResponse) *Client {
	t.Helper()
	transport := &mockTransport{t: t, responses: responses}
	t.Cleanup(func() {
		if transport.next != len(responses) {
			t.Errorf("used %d of %d mock responses", transport.next, len(responses))
		}
	})

	opts := api.ClientOptions{
		AuthToken:    fakeToken,
		Host:         "github.com",
		Transport:    transport,
		LogIgnoreEnv: true,
	}
	rest, err := api.NewRESTClient(opts)
	if err != nil {
		t.Fatalf("failed to create REST client: %v", err)
	}
	graphql, err := api.NewGraphQLClient(opts)
	if err != nil {
		t.Fatalf("failed to create GraphQL client: %v", err)
	}
	return &Client{rest: rest, graphql: graphql}
}

func ownerTypeResponse(username, ownerType string) mockResponse {
	return getResponse("/users/"+username, 200, fmt.Sprintf(`{"type": %q, "login": %q}`, ownerType, username))
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

func TestNewClient(t *testing.T) {
	client, err := NewClient(ClientOptions{AuthToken: fakeToken})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
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
			client := testClient(t, getResponse("/users/"+tt.username, tt.mockStatus, tt.mockBody))

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
			responses := []mockResponse{ownerTypeResponse(tt.username, tt.mockOwnerType)}
			endpoint := "/users/" + tt.username + "/repos"
			ownerType := OwnerTypeUser
			if tt.mockOwnerType == "Organization" {
				endpoint = "/orgs/" + tt.username + "/repos"
				ownerType = OwnerTypeOrganization
			}
			for i, pageBody := range tt.mockPages {
				path := fmt.Sprintf("%s?type=%s&per_page=%d&page=%d", endpoint, mapRepoTypes(tt.repoTypes, ownerType), pageSize, i+1)
				responses = append(responses, getResponse(path, 200, pageBody))
			}

			client := testClient(t, responses...)
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
			path := "/repos/" + tt.owner + "/" + tt.repo
			client := testClient(t, getResponse(path, tt.mockStatus, tt.mockBody))

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
			path := "/repos/" + tt.repo.Owner + "/" + tt.repo.Name + "/git/trees/" + tt.repo.Ref + "?recursive=1"
			client := testClient(t, getResponse(path, tt.mockStatus, tt.mockBody))

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
