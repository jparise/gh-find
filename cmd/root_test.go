package cmd

import (
	"reflect"
	"testing"

	"github.com/jparise/gh-find/internal/github"
)

func TestColorMode(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
		want    colorMode
	}{
		{
			name:    "auto",
			value:   "auto",
			wantErr: false,
			want:    colorAuto,
		},
		{
			name:    "always",
			value:   "always",
			wantErr: false,
			want:    colorAlways,
		},
		{
			name:    "never",
			value:   "never",
			wantErr: false,
			want:    colorNever,
		},
		{
			name:    "invalid value",
			value:   "invalid",
			wantErr: true,
		},
		{
			name:    "empty string",
			value:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c colorMode
			err := c.Set(tt.value)

			if tt.wantErr {
				if err == nil {
					t.Errorf("colorMode.Set(%q) expected error, got nil", tt.value)
				}
				return
			}

			if err != nil {
				t.Errorf("colorMode.Set(%q) unexpected error: %v", tt.value, err)
				return
			}

			if c != tt.want {
				t.Errorf("colorMode.Set(%q) = %v, want %v", tt.value, c, tt.want)
			}

			// Test String() method
			if c.String() != tt.value {
				t.Errorf("colorMode.String() = %q, want %q", c.String(), tt.value)
			}

			// Test Type() method
			if c.Type() != "colorMode" {
				t.Errorf("colorMode.Type() = %q, want %q", c.Type(), "colorMode")
			}
		})
	}
}

func TestRepoTypes(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
		want    github.RepoTypes
	}{
		{
			name:    "single type",
			value:   "sources",
			wantErr: false,
			want:    github.RepoTypes{Sources: true},
		},
		{
			name:    "CSV",
			value:   "sources,forks",
			wantErr: false,
			want:    github.RepoTypes{Sources: true, Forks: true},
		},
		{
			name:    "all (single)",
			value:   "all",
			wantErr: false,
			want:    github.RepoTypes{}.All(),
		},
		{
			name:    "all (mixed)",
			value:   "all,sources",
			wantErr: false,
			want:    github.RepoTypes{}.All(),
		},
		{
			name:    "invalid type",
			value:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f repoTypesFlag
			err := f.Set(tt.value)

			if tt.wantErr {
				if err == nil {
					t.Errorf("repoTypesFlag.Set(%q) expected error, got nil", tt.value)
				}
				return
			}

			if err != nil {
				t.Errorf("repoTypesFlag.Set(%q) unexpected error: %v", tt.value, err)
				return
			}

			if !reflect.DeepEqual(github.RepoTypes(f), tt.want) {
				t.Errorf("repoTypesFlag.Set(%q) = %v, want %v", tt.value, f, tt.want)
			}
		})
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantPattern string
		wantRepos   []string
		wantErr     bool
	}{
		{
			name:    "no args",
			args:    []string{},
			wantErr: true,
		},
		{
			name:        "single repo",
			args:        []string{"cli/cli"},
			wantPattern: "*",
			wantRepos:   []string{"cli/cli"},
			wantErr:     false,
		},
		{
			name:        "explicit pattern with single repo",
			args:        []string{"*.go", "cli/cli"},
			wantPattern: "*.go",
			wantRepos:   []string{"cli/cli"},
			wantErr:     false,
		},
		{
			name:        "explicit pattern with multiple repos",
			args:        []string{"*.go", "cli/cli", "cli/go-gh"},
			wantPattern: "*.go",
			wantRepos:   []string{"cli/cli", "cli/go-gh"},
			wantErr:     false,
		},
		{
			name:        "empty pattern with single repo",
			args:        []string{"", "cli/cli"},
			wantPattern: "*",
			wantRepos:   []string{"cli/cli"},
			wantErr:     false,
		},
		{
			name:        "empty pattern with multiple repos",
			args:        []string{"", "cli/cli", "cli/go-gh"},
			wantPattern: "*",
			wantRepos:   []string{"cli/cli", "cli/go-gh"},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, repos, err := parseArgs(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseArgs(%v) expected error, got nil", tt.args)
				}
				return
			}

			if err != nil {
				t.Errorf("parseArgs(%v) unexpected error: %v", tt.args, err)
				return
			}

			if pattern != tt.wantPattern {
				t.Errorf("parseArgs(%v) pattern = %q, want %q", tt.args, pattern, tt.wantPattern)
			}

			if len(repos) != len(tt.wantRepos) {
				t.Errorf("parseArgs(%v) repos = %v, want %v", tt.args, repos, tt.wantRepos)
				return
			}

			for i, repo := range repos {
				if repo != tt.wantRepos[i] {
					t.Errorf("parseArgs(%v) repos[%d] = %q, want %q", tt.args, i, repo, tt.wantRepos[i])
				}
			}
		})
	}
}

func TestParseByteSize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		// Plain bytes
		{name: "plain number", input: "1024", want: 1024},
		{name: "zero", input: "0", want: 0},
		{name: "bytes suffix", input: "500b", want: 500},
		{name: "bytes uppercase", input: "500B", want: 500},

		// Kilobytes
		{name: "kilobytes", input: "1k", want: 1024},
		{name: "kilobytes kb", input: "10kb", want: 10240},
		{name: "kilobytes uppercase", input: "5K", want: 5120},
		{name: "kilobytes kib", input: "2kib", want: 2048},
		{name: "kilobytes uppercase KB", input: "3KB", want: 3072},

		// Megabytes
		{name: "megabytes", input: "1m", want: 1048576},
		{name: "megabytes mb", input: "5mb", want: 5242880},
		{name: "megabytes uppercase", input: "2M", want: 2097152},
		{name: "megabytes MiB", input: "3MiB", want: 3145728},

		// Gigabytes
		{name: "gigabytes", input: "1g", want: 1073741824},
		{name: "gigabytes gb", input: "2gb", want: 2147483648},
		{name: "gigabytes uppercase", input: "1G", want: 1073741824},
		{name: "gigabytes GiB", input: "1GiB", want: 1073741824},

		// Terabytes
		{name: "terabytes", input: "1t", want: 1099511627776},
		{name: "terabytes tb", input: "2tb", want: 2199023255552},
		{name: "terabytes TiB", input: "1TiB", want: 1099511627776},

		// Petabytes
		{name: "petabytes", input: "1p", want: 1125899906842624},
		{name: "petabytes pb", input: "1pb", want: 1125899906842624},
		{name: "petabytes PiB", input: "1PiB", want: 1125899906842624},

		// Decimal numbers
		{name: "decimal kilobytes", input: "1.5k", want: 1536},
		{name: "decimal megabytes", input: "2.5m", want: 2621440},
		{name: "decimal gigabytes", input: "0.5g", want: 536870912},

		// Whitespace handling
		{name: "leading whitespace", input: "  10m", want: 10485760},
		{name: "trailing whitespace", input: "10m  ", want: 10485760},
		{name: "whitespace around", input: "  10m  ", want: 10485760},
		{name: "whitespace before unit", input: "10 m", want: 10485760},

		// Error cases
		{name: "empty string", input: "", wantErr: true},
		{name: "invalid number", input: "abc", wantErr: true},
		{name: "invalid unit", input: "10x", wantErr: true},
		{name: "negative number", input: "-10m", wantErr: true},
		{name: "just a unit", input: "mb", wantErr: true},
		{name: "multiple decimals", input: "1.5.5m", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseByteSize(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseByteSize(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("parseByteSize(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got != tt.want {
				t.Errorf("parseByteSize(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
