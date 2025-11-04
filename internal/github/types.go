package github

// Repository represents a GitHub repository.
type Repository struct {
	Owner         string
	Name          string
	FullName      string // owner/name
	DefaultBranch string
	Fork          bool
	Archived      bool
	MirrorURL     string
}

// TreeEntry represents a file or directory in a Git tree.
type TreeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"` // blob, tree
	SHA  string `json:"sha"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

// TreeResponse represents the GitHub API tree response.
type TreeResponse struct {
	SHA       string      `json:"sha"`
	URL       string      `json:"url"`
	Tree      []TreeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

// RepoType represents a GitHub repository classification.
type RepoType string

const (
	RepoTypeSources  RepoType = "sources"
	RepoTypeForks    RepoType = "forks"
	RepoTypeArchives RepoType = "archives"
	RepoTypeMirrors  RepoType = "mirrors"
	RepoTypeAll      RepoType = "all"
)
