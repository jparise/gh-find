package github

import (
	"strings"
)

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
	Size int64  `json:"size"`
}

// TreeResponse represents the GitHub API tree response.
type TreeResponse struct {
	Tree      []TreeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

// RepoType represents a GitHub repository classification.
type RepoType string

const (
	// RepoTypeSources represents source repositories (non-forks).
	RepoTypeSources RepoType = "sources"
	// RepoTypeForks represents forked repositories.
	RepoTypeForks RepoType = "forks"
	// RepoTypeArchives represents archived repositories.
	RepoTypeArchives RepoType = "archives"
	// RepoTypeMirrors represents mirrored repositories.
	RepoTypeMirrors RepoType = "mirrors"
)

// ValidRepoTypes is the list of valid repository type values.
var ValidRepoTypes = []string{
	string(RepoTypeSources),
	string(RepoTypeForks),
	string(RepoTypeArchives),
	string(RepoTypeMirrors),
}

// RepoTypes represents a set of repository types to include.
type RepoTypes struct {
	Sources  bool
	Forks    bool
	Archives bool
	Mirrors  bool
}

// All returns a RepoTypes with all types enabled.
func (r RepoTypes) All() RepoTypes {
	return RepoTypes{
		Sources:  true,
		Forks:    true,
		Archives: true,
		Mirrors:  true,
	}
}

func (r RepoTypes) String() string {
	if r.Sources && r.Forks && r.Archives && r.Mirrors {
		return "all"
	}

	var types []string
	if r.Sources {
		types = append(types, string(RepoTypeSources))
	}
	if r.Forks {
		types = append(types, string(RepoTypeForks))
	}
	if r.Archives {
		types = append(types, string(RepoTypeArchives))
	}
	if r.Mirrors {
		types = append(types, string(RepoTypeMirrors))
	}

	return strings.Join(types, ",")
}

// FileType represents a file type classification.
type FileType string

const (
	// FileTypeFile represents a regular file.
	FileTypeFile FileType = "file"
	// FileTypeDirectory represents a directory.
	FileTypeDirectory FileType = "directory"
	// FileTypeSymlink represents a symbolic link.
	FileTypeSymlink FileType = "symlink"
	// FileTypeExecutable represents an executable file.
	FileTypeExecutable FileType = "executable"
	// FileTypeSubmodule represents a Git submodule.
	FileTypeSubmodule FileType = "submodule"
)

// ParseFileType returns the file type based on Git mode.
func ParseFileType(mode string) FileType {
	// Simply switch on the string values rather than converting them to their
	// numeric representation of mode flags. The GitHub API only returns valid
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
