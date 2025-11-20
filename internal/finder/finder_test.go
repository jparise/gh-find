package finder

import (
	"slices"
	"testing"

	"github.com/jparise/gh-find/internal/github"
)

func treePaths(entries []github.TreeEntry) []string {
	paths := make([]string, len(entries))
	for i, e := range entries {
		paths[i] = e.Path
	}
	return paths
}

func TestFilterByType(t *testing.T) {
	tests := []struct {
		name      string
		entries   []github.TreeEntry
		types     []github.FileType
		wantPaths []string
	}{
		{
			name: "filter files only",
			entries: []github.TreeEntry{
				{Path: "main.go", Mode: "100644"},
				{Path: "src", Mode: "040000"},
				{Path: "build.sh", Mode: "100755"},
				{Path: "link", Mode: "120000"},
			},
			types:     []github.FileType{github.FileTypeFile},
			wantPaths: []string{"main.go"},
		},
		{
			name: "filter directories only",
			entries: []github.TreeEntry{
				{Path: "main.go", Mode: "100644"},
				{Path: "src", Mode: "040000"},
				{Path: "pkg", Mode: "040000"},
				{Path: "build.sh", Mode: "100755"},
			},
			types:     []github.FileType{github.FileTypeDirectory},
			wantPaths: []string{"src", "pkg"},
		},
		{
			name: "filter executables only",
			entries: []github.TreeEntry{
				{Path: "main.go", Mode: "100644"},
				{Path: "build.sh", Mode: "100755"},
				{Path: "deploy.sh", Mode: "100755"},
				{Path: "src", Mode: "040000"},
			},
			types:     []github.FileType{github.FileTypeExecutable},
			wantPaths: []string{"build.sh", "deploy.sh"},
		},
		{
			name: "filter symlinks only",
			entries: []github.TreeEntry{
				{Path: "main.go", Mode: "100644"},
				{Path: "link1", Mode: "120000"},
				{Path: "link2", Mode: "120000"},
				{Path: "src", Mode: "040000"},
			},
			types:     []github.FileType{github.FileTypeSymlink},
			wantPaths: []string{"link1", "link2"},
		},
		{
			name: "filter submodules only",
			entries: []github.TreeEntry{
				{Path: "main.go", Mode: "100644"},
				{Path: "vendor/lib", Mode: "160000"},
				{Path: "vendor/dep", Mode: "160000"},
				{Path: "src", Mode: "040000"},
			},
			types:     []github.FileType{github.FileTypeSubmodule},
			wantPaths: []string{"vendor/lib", "vendor/dep"},
		},
		{
			name: "multiple types - OR logic",
			entries: []github.TreeEntry{
				{Path: "main.go", Mode: "100644"},
				{Path: "src", Mode: "040000"},
				{Path: "build.sh", Mode: "100755"},
				{Path: "link", Mode: "120000"},
				{Path: "vendor/lib", Mode: "160000"},
			},
			types:     []github.FileType{github.FileTypeFile, github.FileTypeDirectory},
			wantPaths: []string{"main.go", "src"},
		},
		{
			name: "multiple types including executables",
			entries: []github.TreeEntry{
				{Path: "main.go", Mode: "100644"},
				{Path: "src", Mode: "040000"},
				{Path: "build.sh", Mode: "100755"},
				{Path: "link", Mode: "120000"},
			},
			types:     []github.FileType{github.FileTypeExecutable, github.FileTypeSymlink},
			wantPaths: []string{"build.sh", "link"},
		},
		{
			name: "no type filter - returns all",
			entries: []github.TreeEntry{
				{Path: "main.go", Mode: "100644"},
				{Path: "src", Mode: "040000"},
				{Path: "build.sh", Mode: "100755"},
			},
			types:     []github.FileType{},
			wantPaths: []string{"main.go", "src", "build.sh"},
		},
		{
			name: "no matches",
			entries: []github.TreeEntry{
				{Path: "main.go", Mode: "100644"},
				{Path: "data.txt", Mode: "100644"},
			},
			types:     []github.FileType{github.FileTypeDirectory},
			wantPaths: []string{},
		},
		{
			name:      "empty input slice",
			entries:   []github.TreeEntry{},
			types:     []github.FileType{github.FileTypeFile},
			wantPaths: []string{},
		},
		{
			name: "group-writable file matches file type",
			entries: []github.TreeEntry{
				{Path: "data.txt", Mode: "100664"},
				{Path: "src", Mode: "040000"},
			},
			types:     []github.FileType{github.FileTypeFile},
			wantPaths: []string{"data.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterByType(tt.entries, tt.types)

			if !slices.Equal(treePaths(got), tt.wantPaths) {
				t.Errorf("got %v, want %v", treePaths(got), tt.wantPaths)
			}
		})
	}
}

func TestFilterBySize(t *testing.T) {
	tests := []struct {
		name      string
		entries   []github.TreeEntry
		minSize   int64
		maxSize   int64
		wantPaths []string
	}{
		{
			name: "min size only - filters smaller files",
			entries: []github.TreeEntry{
				{Path: "small.txt", Size: 512},
				{Path: "exact.txt", Size: 1024},
				{Path: "large.txt", Size: 2048},
			},
			minSize:   1024,
			maxSize:   0,
			wantPaths: []string{"exact.txt", "large.txt"},
		},
		{
			name: "max size only - filters larger files",
			entries: []github.TreeEntry{
				{Path: "small.txt", Size: 512},
				{Path: "exact.txt", Size: 1024},
				{Path: "large.txt", Size: 2048},
			},
			minSize:   0,
			maxSize:   1024,
			wantPaths: []string{"small.txt", "exact.txt"},
		},
		{
			name: "both min and max - range filter",
			entries: []github.TreeEntry{
				{Path: "tiny.txt", Size: 100},
				{Path: "min.txt", Size: 500},
				{Path: "mid.txt", Size: 750},
				{Path: "max.txt", Size: 1000},
				{Path: "huge.txt", Size: 2000},
			},
			minSize:   500,
			maxSize:   1000,
			wantPaths: []string{"min.txt", "mid.txt", "max.txt"},
		},
		{
			name: "no filter - returns all",
			entries: []github.TreeEntry{
				{Path: "a.txt", Size: 100},
				{Path: "b.txt", Size: 200},
			},
			minSize:   0,
			maxSize:   0,
			wantPaths: []string{"a.txt", "b.txt"},
		},
		{
			name: "zero-size file with min filter - excluded",
			entries: []github.TreeEntry{
				{Path: "empty.txt", Size: 0},
				{Path: "small.txt", Size: 100},
			},
			minSize:   1,
			maxSize:   0,
			wantPaths: []string{"small.txt"},
		},
		{
			name: "zero-size file with max filter - included",
			entries: []github.TreeEntry{
				{Path: "empty.txt", Size: 0},
				{Path: "small.txt", Size: 100},
			},
			minSize:   0,
			maxSize:   100,
			wantPaths: []string{"empty.txt", "small.txt"},
		},
		{
			name: "boundary: size equals min",
			entries: []github.TreeEntry{
				{Path: "below.txt", Size: 999},
				{Path: "exact.txt", Size: 1000},
				{Path: "above.txt", Size: 1001},
			},
			minSize:   1000,
			maxSize:   0,
			wantPaths: []string{"exact.txt", "above.txt"},
		},
		{
			name: "boundary: size equals max",
			entries: []github.TreeEntry{
				{Path: "below.txt", Size: 999},
				{Path: "exact.txt", Size: 1000},
				{Path: "above.txt", Size: 1001},
			},
			minSize:   0,
			maxSize:   1000,
			wantPaths: []string{"below.txt", "exact.txt"},
		},
		{
			name: "impossible range - min > max returns nothing",
			entries: []github.TreeEntry{
				{Path: "file.txt", Size: 500},
			},
			minSize:   1000,
			maxSize:   100,
			wantPaths: []string{},
		},
		{
			name:      "empty input slice",
			entries:   []github.TreeEntry{},
			minSize:   100,
			maxSize:   1000,
			wantPaths: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterBySize(tt.entries, tt.minSize, tt.maxSize)

			if !slices.Equal(treePaths(got), tt.wantPaths) {
				t.Errorf("got %v, want %v", treePaths(got), tt.wantPaths)
			}
		})
	}
}

func TestFilterByPattern(t *testing.T) {
	entries := []github.TreeEntry{
		{Path: "main.go"},
		{Path: "cmd/root.go"},
		{Path: "internal/foo/bar.go"},
		{Path: "README.md"},
		{Path: "Test.GO"},
	}

	tests := []struct {
		name       string
		pattern    string
		fullPath   bool
		ignoreCase bool
		wantPaths  []string
	}{
		{
			name:      "simple wildcard basename",
			pattern:   "*.go",
			fullPath:  false,
			wantPaths: []string{"main.go", "cmd/root.go", "internal/foo/bar.go"},
		},
		{
			name:      "glob pattern with fullpath",
			pattern:   "**/*.go",
			fullPath:  true,
			wantPaths: []string{"main.go", "cmd/root.go", "internal/foo/bar.go"},
		},
		{
			name:       "case insensitive",
			pattern:    "*.go",
			fullPath:   false,
			ignoreCase: true,
			wantPaths:  []string{"main.go", "cmd/root.go", "internal/foo/bar.go", "Test.GO"},
		},
		{
			name:      "specific filename",
			pattern:   "README.md",
			fullPath:  false,
			wantPaths: []string{"README.md"},
		},
		{
			name:      "no matches",
			pattern:   "*.py",
			fullPath:  false,
			wantPaths: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filterByPattern(entries, tt.pattern, tt.fullPath, tt.ignoreCase)
			if err != nil {
				t.Fatalf("filterByPattern() error = %v", err)
			}

			if !slices.Equal(treePaths(got), tt.wantPaths) {
				t.Errorf("got %v, want %v", treePaths(got), tt.wantPaths)
			}
		})
	}
}

func TestFilterByExcludes(t *testing.T) {
	entries := []github.TreeEntry{
		{Path: "main.go"},
		{Path: "main_test.go"},
		{Path: "cmd/root.go"},
		{Path: "cmd/root_test.go"},
		{Path: "README.md"},
		{Path: "UPPER_test.go"},
	}

	tests := []struct {
		name       string
		excludes   []string
		fullPath   bool
		ignoreCase bool
		wantPaths  []string
	}{
		{
			name:      "exclude test files basename",
			excludes:  []string{"*_test.go"},
			fullPath:  false,
			wantPaths: []string{"main.go", "cmd/root.go", "README.md"},
		},
		{
			name:      "multiple excludes",
			excludes:  []string{"*_test.go", "README.*"},
			fullPath:  false,
			wantPaths: []string{"main.go", "cmd/root.go"},
		},
		{
			name:      "no excludes",
			excludes:  []string{},
			fullPath:  false,
			wantPaths: []string{"main.go", "main_test.go", "cmd/root.go", "cmd/root_test.go", "README.md", "UPPER_test.go"},
		},
		{
			name:      "exclude with fullpath",
			excludes:  []string{"cmd/*"},
			fullPath:  true,
			wantPaths: []string{"main.go", "main_test.go", "README.md", "UPPER_test.go"},
		},
		{
			name:       "case insensitive exclude - single pattern",
			excludes:   []string{"*_TEST.go"},
			fullPath:   false,
			ignoreCase: true,
			wantPaths:  []string{"main.go", "cmd/root.go", "README.md"},
		},
		{
			name:       "case insensitive exclude - multiple patterns",
			excludes:   []string{"*_TEST.go", "readme.*"},
			fullPath:   false,
			ignoreCase: true,
			wantPaths:  []string{"main.go", "cmd/root.go"},
		},
		{
			name:       "case insensitive with fullpath",
			excludes:   []string{"CMD/*"},
			fullPath:   true,
			ignoreCase: true,
			wantPaths:  []string{"main.go", "main_test.go", "README.md", "UPPER_test.go"},
		},
		{
			name:       "case sensitive - should not match different case",
			excludes:   []string{"*_TEST.go"},
			fullPath:   false,
			ignoreCase: false,
			wantPaths:  []string{"main.go", "main_test.go", "cmd/root.go", "cmd/root_test.go", "README.md", "UPPER_test.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filterByExcludes(entries, tt.excludes, tt.fullPath, tt.ignoreCase)
			if err != nil {
				t.Fatalf("filterByExcludes() error = %v", err)
			}

			if !slices.Equal(treePaths(got), tt.wantPaths) {
				t.Errorf("got %v, want %v", treePaths(got), tt.wantPaths)
			}
		})
	}
}

func TestFilterByExtension(t *testing.T) {
	entries := []github.TreeEntry{
		{Path: "main.go"},
		{Path: "README.md"},
		{Path: "config.yaml"},
		{Path: "Test.GO"},
		{Path: "noext"},
	}

	tests := []struct {
		name       string
		extensions []string
		ignoreCase bool
		wantPaths  []string
	}{
		{
			name:       "single extension",
			extensions: []string{".go"},
			wantPaths:  []string{"main.go"},
		},
		{
			name:       "multiple extensions",
			extensions: []string{".go", ".md"},
			wantPaths:  []string{"main.go", "README.md"},
		},
		{
			name:       "case insensitive",
			extensions: []string{".go"},
			ignoreCase: true,
			wantPaths:  []string{"main.go", "Test.GO"},
		},
		{
			name:       "no match",
			extensions: []string{".py"},
			wantPaths:  []string{},
		},
		{
			name:       "empty extensions list",
			extensions: []string{},
			wantPaths:  []string{"main.go", "README.md", "config.yaml", "Test.GO", "noext"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterByExtension(entries, tt.extensions, tt.ignoreCase)

			if !slices.Equal(treePaths(got), tt.wantPaths) {
				t.Errorf("got %v, want %v", treePaths(got), tt.wantPaths)
			}
		})
	}
}
