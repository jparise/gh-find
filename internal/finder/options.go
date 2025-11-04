package finder

import "github.com/jparise/gh-find/internal/github"

// Options contains all search parameters.
type Options struct {
	Pattern    string
	RepoSpecs  []string          // "owner" or "owner/repo", can be multiple
	RepoTypes  []github.RepoType // Repository types to include
	IgnoreCase bool
	FullPath   bool
	Extensions []string
	Excludes   []string // Exclude patterns
	MinSize    int64    // Minimum file size in bytes (0 = no minimum)
	MaxSize    int64    // Maximum file size in bytes (0 = no maximum)
	ClientOpts github.ClientOptions
	Jobs       int // Maximum concurrent API requests
}

// RepoFilter determines which types of repos to include.
type RepoFilter struct {
	IncludeSources  bool
	IncludeForks    bool
	IncludeArchived bool
	IncludeMirrored bool
}
