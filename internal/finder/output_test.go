package finder

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/jparise/gh-find/internal/github"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		name       string
		repo       github.Repository
		path       string
		colorize   bool
		hyperlinks bool
		want       string
		wantURL    string
	}{
		{
			name: "simple match without hyperlinks",
			repo: github.Repository{
				Owner: "cli",
				Name:  "cli",
				Ref:   "trunk",
				URL:   "https://github.com/cli/cli",
			},
			path:       "main.go",
			hyperlinks: false,
			want:       "cli/cli:main.go",
		},
		{
			name: "colored match",
			repo: github.Repository{
				Owner: "color-owner",
				Name:  "color-repo",
				Ref:   "color-ref",
				URL:   "https://example.com/color",
			},
			path:     "rainbow.txt",
			colorize: true,
			want:     "\x1b[0;36mcolor-owner\x1b[0m/\x1b[0;1;32mcolor-repo\x1b[0m:\x1b[0;37mrainbow.txt\x1b[0m",
		},
		{
			name: "match with hyperlinks",
			repo: github.Repository{
				Owner: "cli",
				Name:  "cli",
				Ref:   "trunk",
				URL:   "https://github.com/cli/cli",
			},
			path:       "main.go",
			hyperlinks: true,
			want:       "cli/cli:main.go",
			wantURL:    "https://github.com/cli/cli/blob/trunk/main.go",
		},
		{
			name: "nested path with hyperlinks",
			repo: github.Repository{
				Owner: "golang",
				Name:  "go",
				Ref:   "master",
				URL:   "https://github.com/golang/go",
			},
			path:       "src/cmd/go/main.go",
			hyperlinks: true,
			want:       "golang/go:src/cmd/go/main.go",
			wantURL:    "https://github.com/golang/go/blob/master/src/cmd/go/main.go",
		},
		{
			name: "explicit ref shown in output",
			repo: github.Repository{
				Owner:       "cli",
				Name:        "cli",
				Ref:         "v2.40.0",
				ExplicitRef: true,
				URL:         "https://github.com/cli/cli",
			},
			path:       "main.go",
			hyperlinks: false,
			want:       "cli/cli@v2.40.0:main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			output := NewOutput(stdout, stderr, tt.colorize, tt.hyperlinks)

			output.Match(tt.repo, tt.path)
			got := stdout.String()

			if !strings.Contains(got, tt.want) {
				t.Errorf("Match() output = %q, want to contain %q", got, tt.want)
			}

			if tt.hyperlinks && !strings.Contains(got, tt.wantURL) {
				t.Errorf("Match() output = %q, want to contain URL %q", got, tt.wantURL)
			}

			if stderr.Len() != 0 {
				t.Errorf("Match() wrote to stderr: %q", stderr.String())
			}
		})
	}
}

func TestWarningf(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []any
		want   string
	}{
		{
			name:   "simple warning",
			format: "something went wrong",
			want:   "Warning: something went wrong",
		},
		{
			name:   "with format args",
			format: "%s/%s has %d files",
			args:   []any{"owner", "repo", 100000},
			want:   "Warning: owner/repo has 100000 files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			output := NewOutput(stdout, stderr, false, false)

			output.Warningf(tt.format, tt.args...)
			got := stderr.String()

			if !strings.Contains(got, tt.want) {
				t.Errorf("Warningf() output = %q, want to contain %q", got, tt.want)
			}

			if stdout.Len() != 0 {
				t.Errorf("Warningf() wrote to stdout: %q", stdout.String())
			}
		})
	}
}

func TestOutputThreadSafety(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	output := NewOutput(stdout, stderr, false, false)

	repo := github.Repository{
		Owner: "owner",
		Name:  "repo",
		Ref:   "main",
		URL:   "https://github.com/owner/repo",
	}

	const numGoroutines = 10
	const numCalls = 100

	var wg sync.WaitGroup
	for range numGoroutines {
		wg.Go(func() {
			for range numCalls {
				output.Match(repo, "file.go")
			}
		})
		wg.Go(func() {
			for range numCalls {
				output.Warningf("warning")
			}
		})
	}

	wg.Wait()

	stdoutLines := strings.Count(stdout.String(), "\n")
	stderrLines := strings.Count(stderr.String(), "\n")

	if want := numGoroutines * numCalls; stdoutLines != want {
		t.Errorf("stdout lines = %d, want %d", stdoutLines, want)
	}
	if want := numGoroutines * numCalls; stderrLines != want {
		t.Errorf("stderr lines = %d, want %d", stderrLines, want)
	}
}
