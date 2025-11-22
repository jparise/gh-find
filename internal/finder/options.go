package finder

import (
	"time"

	"github.com/jparise/gh-find/internal/github"
)

// RepoSpec represents a parsed repository specification.
type RepoSpec struct {
	Owner string // Repository owner (user or organization)
	Repo  string // Repository name (empty means expand all repos for owner)
	Ref   string // Branch/tag/SHA (empty means use default branch from API)
}

// Options contains all search parameters.
type Options struct {
	Pattern       string
	RepoSpecs     []RepoSpec
	RepoTypes     github.RepoTypes  // Repository types to include
	FileTypes     []github.FileType // File types to include (OR matching)
	IgnoreCase    bool
	FullPath      bool
	Extensions    []string
	Excludes      []string   // Exclude patterns
	MinSize       int64      // Minimum file size in bytes (0 = no minimum)
	MaxSize       int64      // Maximum file size in bytes (0 = no maximum)
	ChangedAfter  *time.Time // Files changed after this time (nil = no filter)
	ChangedBefore *time.Time // Files changed before this time (nil = no filter)
	ClientOpts    github.ClientOptions
	Jobs          int // Maximum concurrent API requests
}
