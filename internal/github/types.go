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

// FileType represents a file type classification.
type FileType string

const (
	FileTypeFile       FileType = "file"
	FileTypeDirectory  FileType = "directory"
	FileTypeSymlink    FileType = "symlink"
	FileTypeExecutable FileType = "executable"
	FileTypeSubmodule  FileType = "submodule"
)

// ParseFileType returns the file type based on Git mode.
func ParseFileType(mode string) FileType {
	// Simply switch on the string values rather than converting them to their
	// numeric represenation of mode flags. The GitHub API only returns valid
	// mode strings so this should be quick and reliable.
	switch mode {
	case "040000":
		return FileTypeDirectory
	case "120000":
		return FileTypeSymlink
	case "160000":
		return FileTypeSubmodule
	case "100755":
		return FileTypeExecutable
	case "100644", "100664":
		return FileTypeFile
	default: // unknown or unhandled
		return FileTypeFile
	}
}
