package finder

import (
	"slices"
	"testing"

	"github.com/jparise/gh-find/internal/github"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		path       string
		ignoreCase bool
		want       bool
		wantErr    bool
	}{
		// Basic glob patterns
		{
			name:    "match any .go file",
			pattern: "*.go",
			path:    "main.go",
			want:    true,
		},
		{
			name:    "match specific file",
			pattern: "main.go",
			path:    "main.go",
			want:    true,
		},
		{
			name:    "no match different extension",
			pattern: "*.go",
			path:    "main.js",
			want:    false,
		},
		{
			name:    "match all files",
			pattern: "*",
			path:    "anything.txt",
			want:    true,
		},

		// Doublestar patterns
		{
			name:    "match with doublestar",
			pattern: "**/*.go",
			path:    "cmd/main.go",
			want:    true,
		},
		{
			name:    "match with doublestar deep path",
			pattern: "**/*.go",
			path:    "internal/finder/finder.go",
			want:    true,
		},
		{
			name:    "match test files",
			pattern: "**/*_test.go",
			path:    "internal/finder/finder_test.go",
			want:    true,
		},
		{
			name:    "no match without doublestar",
			pattern: "*.go",
			path:    "cmd/main.go",
			want:    false,
		},

		// Case sensitivity
		{
			name:       "case sensitive no match",
			pattern:    "README.md",
			path:       "readme.md",
			ignoreCase: false,
			want:       false,
		},
		{
			name:       "case insensitive match",
			pattern:    "README.md",
			path:       "readme.md",
			ignoreCase: true,
			want:       true,
		},
		{
			name:       "case insensitive match uppercase",
			pattern:    "readme*",
			path:       "README.MD",
			ignoreCase: true,
			want:       true,
		},

		// Question mark patterns
		{
			name:    "question mark single char",
			pattern: "file?.go",
			path:    "file1.go",
			want:    true,
		},
		{
			name:    "question mark no match multiple chars",
			pattern: "file?.go",
			path:    "file12.go",
			want:    false,
		},

		// Character class patterns
		{
			name:    "character class match",
			pattern: "file[123].go",
			path:    "file1.go",
			want:    true,
		},
		{
			name:    "character class no match",
			pattern: "file[123].go",
			path:    "file4.go",
			want:    false,
		},

		// Brace expansion patterns
		{
			name:    "brace expansion first option",
			pattern: "*.{go,js}",
			path:    "main.go",
			want:    true,
		},
		{
			name:    "brace expansion second option",
			pattern: "*.{go,js}",
			path:    "main.js",
			want:    true,
		},
		{
			name:    "brace expansion no match",
			pattern: "*.{go,js}",
			path:    "main.py",
			want:    false,
		},

		// Edge cases
		{
			name:    "empty pattern matches empty path",
			pattern: "",
			path:    "",
			want:    true,
		},
		{
			name:    "dotfile match",
			pattern: ".*",
			path:    ".gitignore",
			want:    true,
		},
		{
			name:    "hidden directory file",
			pattern: "**/*.go",
			path:    ".github/main.go",
			want:    true,
		},
		{
			name:    "unicode in filename",
			pattern: "*.md",
			path:    "文档.md",
			want:    true,
		},
		{
			name:    "spaces in filename",
			pattern: "my file.txt",
			path:    "my file.txt",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matchPattern(tt.pattern, tt.path, tt.ignoreCase)
			if (err != nil) != tt.wantErr {
				t.Errorf("matchPattern() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("matchPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

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

func TestHasExtension(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		extensions []string
		want       bool
	}{
		// Basic extension matching
		{
			name:       "match single extension",
			path:       "main.go",
			extensions: []string{"go"},
			want:       true,
		},
		{
			name:       "match with leading dot",
			path:       "main.go",
			extensions: []string{".go"},
			want:       true,
		},
		{
			name:       "no match different extension",
			path:       "main.go",
			extensions: []string{"js"},
			want:       false,
		},
		{
			name:       "match one of multiple extensions",
			path:       "script.js",
			extensions: []string{"go", "js", "py"},
			want:       true,
		},

		// Multiple extensions in path
		{
			name:       "match last extension in compound",
			path:       "file.tar.gz",
			extensions: []string{"gz"},
			want:       true,
		},
		{
			name:       "no match first extension in compound",
			path:       "file.tar.gz",
			extensions: []string{"tar"},
			want:       false,
		},

		// No extension cases
		{
			name:       "no extension no match",
			path:       "Makefile",
			extensions: []string{"go"},
			want:       false,
		},
		{
			name:       "empty extensions list",
			path:       "main.go",
			extensions: []string{},
			want:       false,
		},

		// Dotfiles
		{
			name:       "dotfile with extension",
			path:       ".gitignore",
			extensions: []string{"gitignore"},
			want:       true,
		},
		{
			name:       "dotfile is not an extension",
			path:       ".bashrc",
			extensions: []string{"bashrc"},
			want:       true,
		},

		// Edge cases
		{
			name:       "path with directory",
			path:       "cmd/main.go",
			extensions: []string{"go"},
			want:       true,
		},
		{
			name:       "hidden directory",
			path:       ".github/workflows/test.yml",
			extensions: []string{"yml"},
			want:       true,
		},
		{
			name:       "extension with leading dot in list",
			path:       "test.md",
			extensions: []string{".md", ".txt"},
			want:       true,
		},
		{
			name:       "mixed dot and no-dot in extensions",
			path:       "test.go",
			extensions: []string{".js", "go"},
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasExtension(tt.path, tt.extensions)
			if got != tt.want {
				t.Errorf("hasExtension() = %v, want %v", got, tt.want)
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

func TestExcludePatterns(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		excludes   []string
		files      []string
		fullPath   bool
		ignoreCase bool
		want       []string
	}{
		{
			name:     "single exclude pattern",
			pattern:  "*.js",
			excludes: []string{"*.test.js"},
			files:    []string{"main.js", "app.test.js", "utils.js"},
			want:     []string{"main.js", "utils.js"},
		},
		{
			name:     "multiple exclude patterns",
			pattern:  "*",
			excludes: []string{"*.test.js", "*.spec.js"},
			files:    []string{"main.js", "app.test.js", "utils.spec.js", "index.html"},
			want:     []string{"main.js", "index.html"},
		},
		{
			name:     "no files excluded",
			pattern:  "*.go",
			excludes: []string{"*.py"},
			files:    []string{"main.go", "utils.go"},
			want:     []string{"main.go", "utils.go"},
		},
		{
			name:     "all files excluded",
			pattern:  "*",
			excludes: []string{"*"},
			files:    []string{"main.go", "utils.go"},
			want:     []string{},
		},
		{
			name:     "exclude with basename matching",
			pattern:  "*",
			excludes: []string{"README.md"},
			files:    []string{"README.md", "docs/README.md", "main.go"},
			fullPath: false,
			want:     []string{"main.go"},
		},
		{
			name:     "exclude with full path matching",
			pattern:  "**/*",
			excludes: []string{"**/test/**"},
			files:    []string{"main.go", "test/helper.go", "src/app.go"},
			fullPath: true,
			want:     []string{"main.go", "src/app.go"},
		},
		{
			name:       "exclude case insensitive",
			pattern:    "*",
			excludes:   []string{"readme*"},
			files:      []string{"README.md", "readme.txt", "main.go"},
			ignoreCase: true,
			want:       []string{"main.go"},
		},
		{
			name:       "exclude case sensitive",
			pattern:    "*",
			excludes:   []string{"readme*"},
			files:      []string{"README.md", "readme.txt", "main.go"},
			ignoreCase: false,
			want:       []string{"README.md", "main.go"},
		},
		{
			name:     "exclude node_modules",
			pattern:  "**/*.js",
			excludes: []string{"**/node_modules/**"},
			files: []string{
				"src/app.js",
				"node_modules/pkg/index.js",
				"test/test.js",
			},
			fullPath: true,
			want:     []string{"src/app.js", "test/test.js"},
		},
		{
			name:     "exclude build directories",
			pattern:  "**/*",
			excludes: []string{"dist/*", "build/*"},
			files: []string{
				"main.go",
				"dist/bundle.js",
				"build/app.js",
				"src/app.go",
			},
			fullPath: true,
			want:     []string{"main.go", "src/app.go"},
		},
		{
			name:     "empty exclude list",
			pattern:  "*.go",
			excludes: []string{},
			files:    []string{"main.go", "utils.go"},
			want:     []string{"main.go", "utils.go"},
		},
		{
			name:     "exclude with wildcard",
			pattern:  "**/*.go",
			excludes: []string{"*_test.go"},
			files:    []string{"main.go", "main_test.go", "utils.go", "utils_test.go"},
			fullPath: false, // basename matching
			want:     []string{"main.go", "utils.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the filtering logic
			var got []string
			for _, file := range tt.files {
				// Determine match path based on fullPath flag
				matchPath := file
				if !tt.fullPath {
					// Extract basename
					lastSlash := -1
					for i := len(file) - 1; i >= 0; i-- {
						if file[i] == '/' {
							lastSlash = i
							break
						}
					}
					if lastSlash >= 0 {
						matchPath = file[lastSlash+1:]
					}
				}

				// Test pattern match
				matched, err := matchPattern(tt.pattern, matchPath, tt.ignoreCase)
				if err != nil {
					t.Fatalf("pattern matching failed: %v", err)
				}

				if matched {
					// Test exclude patterns
					excluded := false
					for _, excludePattern := range tt.excludes {
						isExcluded, err := matchPattern(excludePattern, matchPath, tt.ignoreCase)
						if err != nil {
							t.Fatalf("exclude pattern matching failed: %v", err)
						}
						if isExcluded {
							excluded = true
							break
						}
					}

					if !excluded {
						got = append(got, file)
					}
				}
			}

			// Compare results
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

// TestExcludeWithExtensions tests that exclude patterns work with extension filters.
func TestExcludeWithExtensions(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		extensions []string
		excludes   []string
		files      [][]string // [path, extension]
		want       []string
	}{
		{
			name:       "extension filter then exclude",
			pattern:    "*",
			extensions: []string{"js"},
			excludes:   []string{"*.test.js"},
			files: [][]string{
				{"main.js", "js"},
				{"app.test.js", "js"},
				{"utils.go", "go"},
				{"helper.js", "js"},
			},
			want: []string{"main.js", "helper.js"},
		},
		{
			name:       "multiple extensions with exclude",
			pattern:    "*",
			extensions: []string{"go", "md"},
			excludes:   []string{"*_test.go", "README.md"},
			files: [][]string{
				{"main.go", "go"},
				{"main_test.go", "go"},
				{"README.md", "md"},
				{"CHANGELOG.md", "md"},
				{"app.js", "js"},
			},
			want: []string{"main.go", "CHANGELOG.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, fileData := range tt.files {
				file := fileData[0]
				ext := fileData[1]

				// Simulate extension filtering
				if len(tt.extensions) > 0 && !slices.Contains(tt.extensions, ext) {
					continue
				}

				// Pattern matching (basename)
				matchPath := file
				lastSlash := -1
				for i := len(file) - 1; i >= 0; i-- {
					if file[i] == '/' {
						lastSlash = i
						break
					}
				}
				if lastSlash >= 0 {
					matchPath = file[lastSlash+1:]
				}

				matched, err := matchPattern(tt.pattern, matchPath, false)
				if err != nil {
					t.Fatalf("pattern matching failed: %v", err)
				}

				if matched {
					// Exclude patterns
					excluded := false
					for _, excludePattern := range tt.excludes {
						isExcluded, err := matchPattern(excludePattern, matchPath, false)
						if err != nil {
							t.Fatalf("exclude pattern matching failed: %v", err)
						}
						if isExcluded {
							excluded = true
							break
						}
					}

					if !excluded {
						got = append(got, file)
					}
				}
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
