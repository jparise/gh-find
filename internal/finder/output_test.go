package finder

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestNewOutput(t *testing.T) {
	tests := []struct {
		name     string
		colorize bool
	}{
		{
			name:     "with colors",
			colorize: true,
		},
		{
			name:     "without colors",
			colorize: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			output := NewOutput(stdout, stderr, tt.colorize)
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
		name  string
		owner string
		repo  string
		path  string
		want  string
	}{
		{
			name:  "simple match",
			owner: "cli",
			repo:  "cli",
			path:  "main.go",
			want:  "cli/cli:main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			output := NewOutput(stdout, stderr, false)

			output.Match(tt.owner, tt.repo, tt.path)
			got := stdout.String()

			if !strings.Contains(got, tt.want) {
				t.Errorf("Match() output = %q, want to contain %q", got, tt.want)
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
			output := NewOutput(stdout, stderr, false)

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
			output := NewOutput(stdout, stderr, false)

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
	output := NewOutput(stdout, stderr, false)

	const numGoroutines = 10
	const numCalls = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3)

	for range numGoroutines {
		go func() {
			defer wg.Done()
			for range numCalls {
				output.Match("owner", "repo", "file.go")
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
