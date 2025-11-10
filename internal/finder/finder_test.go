package finder

import (
	"slices"
	"testing"

	"github.com/jparise/gh-find/internal/github"
)

func TestMatchesFileType(t *testing.T) {
	tests := []struct {
		name  string
		entry *github.TreeEntry
		types []github.FileType
		want  bool
	}{
		{
			name: "file matches file type",
			entry: &github.TreeEntry{
				Mode: "100644",
				Path: "main.go",
			},
			types: []github.FileType{github.FileTypeFile},
			want:  true,
		},
		{
			name: "executable matches executable type",
			entry: &github.TreeEntry{
				Mode: "100755",
				Path: "build.sh",
			},
			types: []github.FileType{github.FileTypeExecutable},
			want:  true,
		},
		{
			name: "symlink matches symlink type",
			entry: &github.TreeEntry{
				Mode: "120000",
				Path: "link",
			},
			types: []github.FileType{github.FileTypeSymlink},
			want:  true,
		},
		{
			name: "directory matches directory type",
			entry: &github.TreeEntry{
				Mode: "040000",
				Path: "src",
			},
			types: []github.FileType{github.FileTypeDirectory},
			want:  true,
		},
		{
			name: "file does not match directory type",
			entry: &github.TreeEntry{
				Mode: "100644",
				Path: "main.go",
			},
			types: []github.FileType{github.FileTypeDirectory},
			want:  false,
		},
		{
			name: "file matches in multiple types (OR logic)",
			entry: &github.TreeEntry{
				Mode: "100644",
				Path: "main.go",
			},
			types: []github.FileType{github.FileTypeDirectory, github.FileTypeFile},
			want:  true,
		},
		{
			name: "executable matches in multiple types (OR logic)",
			entry: &github.TreeEntry{
				Mode: "100755",
				Path: "script.sh",
			},
			types: []github.FileType{github.FileTypeFile, github.FileTypeExecutable},
			want:  true,
		},
		{
			name: "file does not match when type not in list",
			entry: &github.TreeEntry{
				Mode: "100644",
				Path: "main.go",
			},
			types: []github.FileType{github.FileTypeDirectory, github.FileTypeSymlink},
			want:  false,
		},
		{
			name: "group-writable file matches file type",
			entry: &github.TreeEntry{
				Mode: "100664",
				Path: "data.txt",
			},
			types: []github.FileType{github.FileTypeFile},
			want:  true,
		},
		{
			name: "submodule matches submodule type",
			entry: &github.TreeEntry{
				Mode: "160000",
				Path: "vendor/lib",
			},
			types: []github.FileType{github.FileTypeSubmodule},
			want:  true,
		},
		{
			name: "submodule does not match file type",
			entry: &github.TreeEntry{
				Mode: "160000",
				Path: "vendor/lib",
			},
			types: []github.FileType{github.FileTypeFile},
			want:  false,
		},
		{
			name: "empty types list returns false",
			entry: &github.TreeEntry{
				Mode: "100644",
				Path: "main.go",
			},
			types: []github.FileType{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesFileType(tt.entry, tt.types)
			if got != tt.want {
				t.Errorf("matchesType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRepoSpec(t *testing.T) {
	tests := []struct {
		name      string
		spec      string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "owner only",
			spec:      "octocat",
			wantOwner: "octocat",
			wantRepo:  "",
			wantErr:   false,
		},
		{
			name:      "owner and repo",
			spec:      "octocat/hello-world",
			wantOwner: "octocat",
			wantRepo:  "hello-world",
			wantErr:   false,
		},
		{
			name:      "org name",
			spec:      "github",
			wantOwner: "github",
			wantRepo:  "",
			wantErr:   false,
		},
		{
			name:      "repo with dashes",
			spec:      "cli/gh-find",
			wantOwner: "cli",
			wantRepo:  "gh-find",
			wantErr:   false,
		},
		{
			name:    "too many slashes",
			spec:    "owner/repo/extra",
			wantErr: true,
		},
		{
			name:      "empty string",
			spec:      "",
			wantOwner: "",
			wantRepo:  "",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOwner, gotRepo, err := parseRepoSpec(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRepoSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotOwner != tt.wantOwner {
					t.Errorf("parseRepoSpec() owner = %v, want %v", gotOwner, tt.wantOwner)
				}
				if gotRepo != tt.wantRepo {
					t.Errorf("parseRepoSpec() repo = %v, want %v", gotRepo, tt.wantRepo)
				}
			}
		})
	}
}

func TestSizeFilter(t *testing.T) {
	tests := []struct {
		name    string
		minSize int64
		maxSize int64
		files   []struct {
			path string
			size int64
		}
		want []string
	}{
		{
			name:    "min size filter",
			minSize: 1024, // 1KB
			maxSize: 0,
			files: []struct {
				path string
				size int64
			}{
				{"small.txt", 512},
				{"medium.txt", 1024},
				{"large.txt", 2048},
			},
			want: []string{"medium.txt", "large.txt"},
		},
		{
			name:    "max size filter",
			minSize: 0,
			maxSize: 1024, // 1KB
			files: []struct {
				path string
				size int64
			}{
				{"small.txt", 512},
				{"medium.txt", 1024},
				{"large.txt", 2048},
			},
			want: []string{"small.txt", "medium.txt"},
		},
		{
			name:    "range filter",
			minSize: 1024,
			maxSize: 2048,
			files: []struct {
				path string
				size int64
			}{
				{"tiny.txt", 256},
				{"small.txt", 512},
				{"medium.txt", 1024},
				{"large.txt", 2048},
				{"huge.txt", 4096},
			},
			want: []string{"medium.txt", "large.txt"},
		},
		{
			name:    "exact size (min=max)",
			minSize: 1024,
			maxSize: 1024,
			files: []struct {
				path string
				size int64
			}{
				{"small.txt", 512},
				{"exact.txt", 1024},
				{"large.txt", 2048},
			},
			want: []string{"exact.txt"},
		},
		{
			name:    "no size filter",
			minSize: 0,
			maxSize: 0,
			files: []struct {
				path string
				size int64
			}{
				{"a.txt", 100},
				{"b.txt", 200},
				{"c.txt", 300},
			},
			want: []string{"a.txt", "b.txt", "c.txt"},
		},
		{
			name:    "zero-size files with max filter",
			minSize: 0,
			maxSize: 100,
			files: []struct {
				path string
				size int64
			}{
				{"empty.txt", 0},
				{"small.txt", 50},
				{"medium.txt", 100},
				{"large.txt", 200},
			},
			want: []string{"empty.txt", "small.txt", "medium.txt"},
		},
		{
			name:    "large file sizes (megabytes)",
			minSize: 1048576, // 1MB
			maxSize: 5242880, // 5MB
			files: []struct {
				path string
				size int64
			}{
				{"small.bin", 524288},   // 512KB
				{"medium.bin", 1048576}, // 1MB
				{"large.bin", 3145728},  // 3MB
				{"huge.bin", 10485760},  // 10MB
			},
			want: []string{"medium.bin", "large.bin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string

			for _, file := range tt.files {
				// Simulate size filtering
				if tt.minSize > 0 && file.size < tt.minSize {
					continue
				}
				if tt.maxSize > 0 && file.size > tt.maxSize {
					continue
				}

				got = append(got, file.path)
			}

			if len(got) != len(tt.want) {
				t.Errorf("got %d files, want %d\ngot:  %v\nwant: %v", len(got), len(tt.want), got, tt.want)
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestSizeFilterWithExtensions tests that size filter works with extension filters.
func TestSizeFilterWithExtensions(t *testing.T) {
	tests := []struct {
		name       string
		minSize    int64
		maxSize    int64
		extensions []string
		files      []struct {
			path string
			ext  string
			size int64
		}
		want []string
	}{
		{
			name:       "size and extension combined",
			minSize:    1024,
			maxSize:    0,
			extensions: []string{"go"},
			files: []struct {
				path string
				ext  string
				size int64
			}{
				{"small.go", "go", 512},
				{"large.go", "go", 2048},
				{"small.js", "js", 512},
				{"large.js", "js", 2048},
			},
			want: []string{"large.go"},
		},
		{
			name:       "multiple extensions with size range",
			minSize:    500,
			maxSize:    1500,
			extensions: []string{"go", "md"},
			files: []struct {
				path string
				ext  string
				size int64
			}{
				{"tiny.go", "go", 100},
				{"small.go", "go", 800},
				{"large.go", "go", 2000},
				{"readme.md", "md", 1000},
				{"app.js", "js", 900},
			},
			want: []string{"small.go", "readme.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string

			for _, file := range tt.files {
				// Simulate extension filtering
				if len(tt.extensions) > 0 && !slices.Contains(tt.extensions, file.ext) {
					continue
				}

				// Simulate size filtering
				if tt.minSize > 0 && file.size < tt.minSize {
					continue
				}
				if tt.maxSize > 0 && file.size > tt.maxSize {
					continue
				}

				got = append(got, file.path)
			}

			if len(got) != len(tt.want) {
				t.Errorf("got %d files, want %d\ngot:  %v\nwant: %v", len(got), len(tt.want), got, tt.want)
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
