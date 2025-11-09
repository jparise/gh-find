package finder

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestNewOutput(t *testing.T) {
	tests := []struct {
		name       string
		colorize   bool
		hyperlinks bool
	}{
		{
			name:       "with colors and hyperlinks",
			colorize:   true,
			hyperlinks: true,
		},
		{
			name:       "with colors only",
			colorize:   true,
			hyperlinks: false,
		},
		{
			name:       "without colors or hyperlinks",
			colorize:   false,
			hyperlinks: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			output := NewOutput(stdout, stderr, tt.colorize, tt.hyperlinks)
			colorFuncs := []struct {
				name string
				fn   func(string) string
			}{
				{"cyan", output.cyan},
				{"green", output.green},
				{"white", output.white},
				{"yellow", output.yellow},
				{"red", output.red},
			}
			for _, cf := range colorFuncs {
				if cf.fn == nil {
					t.Errorf("NewOutput() %s color func is nil", cf.name)
				}
				s := cf.fn("test")
				if tt.colorize {
					if s == "test" {
						t.Errorf("NewOutput() expected %s color func to return ANSI codes", cf.name)
					}
				} else {
					if s != "test" {
						t.Errorf("NewOutput() expected %s color func to return plain string, got %q", cf.name, s)
					}
				}
			}
		})
	}
}

func TestMatch(t *testing.T) {
	tests := []struct {
		name       string
		owner      string
		repo       string
		branch     string
		path       string
		hyperlinks bool
		want       string
		wantURL    string
	}{
		{
			name:       "simple match without hyperlinks",
			owner:      "cli",
			repo:       "cli",
			branch:     "trunk",
			path:       "main.go",
			hyperlinks: false,
			want:       "cli/cli:main.go",
		},
		{
			name:       "match with hyperlinks",
			owner:      "cli",
			repo:       "cli",
			branch:     "trunk",
			path:       "main.go",
			hyperlinks: true,
			want:       "cli/cli:main.go",
			wantURL:    "https://github.com/cli/cli/blob/trunk/main.go",
		},
		{
			name:       "nested path with hyperlinks",
			owner:      "golang",
			repo:       "go",
			branch:     "master",
			path:       "src/cmd/go/main.go",
			hyperlinks: true,
			want:       "golang/go:src/cmd/go/main.go",
			wantURL:    "https://github.com/golang/go/blob/master/src/cmd/go/main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			output := NewOutput(stdout, stderr, false, tt.hyperlinks)

			output.Match(tt.owner, tt.repo, tt.branch, tt.path)
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

func TestInfof(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []any
		want   string
	}{
		{
			name:   "simple info",
			format: "processing repository",
			want:   "processing repository",
		},
		{
			name:   "with format args",
			format: "searching %s/%s",
			args:   []any{"owner", "repo"},
			want:   "searching owner/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			output := NewOutput(stdout, stderr, false, false)

			output.Infof(tt.format, tt.args...)
			got := stderr.String()

			if !strings.Contains(got, tt.want) {
				t.Errorf("Infof() output = %q, want to contain %q", got, tt.want)
			}

			if stdout.Len() != 0 {
				t.Errorf("Infof() wrote to stdout: %q", stdout.String())
			}
		})
	}
}

func TestOutputThreadSafety(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	output := NewOutput(stdout, stderr, false, false)

	const numGoroutines = 10
	const numCalls = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3)

	for range numGoroutines {
		go func() {
			defer wg.Done()
			for range numCalls {
				output.Match("owner", "repo", "main", "file.go")
			}
		}()
		go func() {
			defer wg.Done()
			for range numCalls {
				output.Warningf("warning")
			}
		}()
		go func() {
			defer wg.Done()
			for range numCalls {
				output.Infof("info")
			}
		}()
	}

	wg.Wait()

	stdoutLines := strings.Count(stdout.String(), "\n")
	stderrLines := strings.Count(stderr.String(), "\n")

	if want := numGoroutines * numCalls; stdoutLines != want {
		t.Errorf("stdout lines = %d, want %d", stdoutLines, want)
	}
	if want := numGoroutines * numCalls * 2; stderrLines != want {
		t.Errorf("stderr lines = %d, want %d (Warningf + Infof)", stderrLines, want)
	}
}
