package cmd

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/cli/go-gh/v2/pkg/term"
	"github.com/jparise/gh-find/internal/finder"
	"github.com/jparise/gh-find/internal/github"
	"github.com/spf13/cobra"
)

// colorMode represents when to use colored output.
type colorMode string

const (
	colorAuto   colorMode = "auto"
	colorAlways colorMode = "always"
	colorNever  colorMode = "never"
)

// String is used both by fmt.Print and by Cobra in help text.
func (c *colorMode) String() string {
	return string(*c)
}

// Set must have pointer receiver to validate and set the value.
func (c *colorMode) Set(v string) error {
	switch v {
	case "auto", "always", "never":
		*c = colorMode(v)
		return nil
	default:
		return fmt.Errorf("must be one of \"auto\", \"always\", or \"never\"")
	}
}

// Type is only used in help text.
func (c *colorMode) Type() string {
	return "colorMode"
}

var (
	version = "dev"

	// Flags.
	color      = colorAuto
	repoTypes  []string
	ignoreCase bool
	fullPath   bool
	extensions []string
	excludes   []string
	minSize    string
	maxSize    string
	noCache    bool
	cacheDir   string
	cacheTTL   time.Duration
	jobs       int
)

var rootCmd = &cobra.Command{
	Use:   "gh-find [<pattern>] <repository>...",
	Short: "Find files across GitHub repositories",
	Long: `gh-find is a find(1)-like utility for GitHub repositories.

<pattern> is a glob pattern to match files:
  *              Match any characters (e.g., "*.go")
  **             Match across directories (e.g., "**/*.js")
  ?              Match single character (e.g., "file?.txt")
  [...]          Match character class (e.g., "file[0-9].txt")
  {...}          Match alternatives (e.g., "*.{go,md}")

When searching a single repository, pattern defaults to "*". When searching
multiple repositories, the first argument is the pattern and the rest are
repositories.

<repository> can be:
  <owner>        Search all repositories for a user or organization
  <owner>/<repo> Search a specific repository

You can specify multiple repositories to search across them all.

Examples:
  gh find "*.go" cli
  gh find "*.go" cli/cli cli/go-gh
  gh find --repo-types sources,forks "*.md" torvalds
  gh find -p "**/*_test.go" golang/go
  gh find "*" cli/cli cli/go-gh
  gh find -e go -e md cli
  gh find --min-size 50k "*.go" golang/go
  gh find "*.js" -E "*.test.js" -E "*.spec.js" facebook/react
  gh find --min-size 10k --max-size 100k "*.go" cli/cli`,
	Version: version,
	Args:    cobra.MinimumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if jobs < 1 || jobs > 100 {
			return fmt.Errorf("--jobs must be between 1 and 100, got %d", jobs)
		}

		// Validate --repo-types values
		for _, rt := range repoTypes {
			switch rt {
			case string(github.RepoTypeSources),
				string(github.RepoTypeForks),
				string(github.RepoTypeArchives),
				string(github.RepoTypeMirrors),
				string(github.RepoTypeAll):
				// Valid
			default:
				return fmt.Errorf("invalid repo type %q: must be one of sources, forks, archives, mirrors, or all", rt)
			}
		}

		// Don't allow mixing "all" with other types
		if len(repoTypes) > 1 && slices.Contains(repoTypes, string(github.RepoTypeAll)) {
			return fmt.Errorf("repo type \"all\" cannot be combined with other types")
		}

		return nil
	},
	RunE: run,
}

func init() {
	rootCmd.Flags().StringSliceVar(&repoTypes, "repo-types", []string{"sources"},
		"repo types to include: sources,forks,archives,mirrors,all")
	rootCmd.Flags().Var(&color, "color",
		"colorize output: auto, always, never")
	rootCmd.Flags().BoolVarP(&ignoreCase, "ignore-case", "i", false,
		"case-insensitive pattern matching")
	rootCmd.Flags().BoolVarP(&fullPath, "full-path", "p", false,
		"match pattern against full path (default: basename only)")
	rootCmd.Flags().StringSliceVarP(&extensions, "extension", "e", []string{},
		"filter by file extension (can be specified multiple times)")
	rootCmd.Flags().StringSliceVarP(&excludes, "exclude", "E", []string{},
		"exclude patterns (can be specified multiple times)")
	rootCmd.Flags().StringVar(&minSize, "min-size", "",
		"minimum file size (e.g., 1M, 500k, 1GB)")
	rootCmd.Flags().StringVar(&maxSize, "max-size", "",
		"maximum file size (e.g., 5M, 1GB)")
	rootCmd.Flags().BoolVar(&noCache, "no-cache", false,
		"bypass cache, always fetch fresh data")
	rootCmd.Flags().StringVar(&cacheDir, "cache-dir", "",
		"override cache directory location")
	rootCmd.Flags().DurationVar(&cacheTTL, "cache-ttl", 24*time.Hour,
		"cache time-to-live (e.g., 1h, 30m, 24h)")
	rootCmd.Flags().IntVarP(&jobs, "jobs", "j", 10,
		"maximum concurrent API requests")
}

func Execute() error {
	return rootCmd.Execute()
}

// parseByteSize parses a human-readable size string into bytes.
// Supports formats like "1M", "500k", "1.5G", "1024" (plain bytes).
// Units are case-insensitive and use binary (1024-based) multipliers.
func parseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size string")
	}

	// Find where the unit starts (last non-digit character)
	i := len(s) - 1
	for i >= 0 && !unicode.IsDigit(rune(s[i])) && s[i] != '.' {
		i--
	}

	// Parse the number part
	numStr := s[:i+1]
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q: %w", numStr, err)
	}
	if num < 0 {
		return 0, fmt.Errorf("size cannot be negative")
	}

	// Parse the unit suffix
	unit := strings.ToLower(strings.TrimSpace(s[i+1:]))
	var multiplier float64
	switch unit {
	case "", "b":
		multiplier = 1
	case "k", "kb", "kib":
		multiplier = 1024
	case "m", "mb", "mib":
		multiplier = 1024 * 1024
	case "g", "gb", "gib":
		multiplier = 1024 * 1024 * 1024
	case "t", "tb", "tib":
		multiplier = 1024 * 1024 * 1024 * 1024
	case "p", "pb", "pib":
		multiplier = 1024 * 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown unit %q (supported: b, k, m, g, t, p)", unit)
	}

	result := num * multiplier
	if result > float64(math.MaxInt64) {
		return 0, fmt.Errorf("size too large (exceeds max int64)")
	}

	return int64(result), nil
}

// parseArgs parses command-line arguments into a pattern and repository specs.
func parseArgs(args []string) (pattern string, repoSpecs []string, err error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("at least one repository is required")
	}

	// Single arg: it's a repo (pattern defaults to "*")
	// Multiple args: first is pattern, rest are repos
	if len(args) == 1 {
		pattern = "*"
		repoSpecs = args
	} else {
		pattern = args[0]
		repoSpecs = args[1:]

		if pattern == "" {
			pattern = "*"
		}
	}

	return pattern, repoSpecs, nil
}

func run(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pattern, repoSpecs, err := parseArgs(args)
	if err != nil {
		return err
	}

	var colorize bool
	switch color {
	case colorAlways:
		colorize = true
	case colorNever:
		colorize = false
	case colorAuto:
		terminal := term.FromEnv()
		colorize = terminal.IsColorEnabled()
	}

	// Convert validated strings to typed constants
	repoTypesTyped := make([]github.RepoType, len(repoTypes))
	for i, s := range repoTypes {
		repoTypesTyped[i] = github.RepoType(s)
	}

	// Parse size filters
	var minSizeBytes, maxSizeBytes int64

	if minSize != "" {
		size, err := parseByteSize(minSize)
		if err != nil {
			return fmt.Errorf("invalid --min-size %q: %w", minSize, err)
		}
		if size == 0 {
			return fmt.Errorf("--min-size must be greater than 0")
		}
		minSizeBytes = size
	}

	if maxSize != "" {
		size, err := parseByteSize(maxSize)
		if err != nil {
			return fmt.Errorf("invalid --max-size %q: %w", maxSize, err)
		}
		if size == 0 {
			return fmt.Errorf("--max-size must be greater than 0")
		}
		maxSizeBytes = size
	}

	// Validate that min <= max if both specified
	if minSizeBytes > 0 && maxSizeBytes > 0 && minSizeBytes > maxSizeBytes {
		return fmt.Errorf("--min-size cannot be greater than --max-size")
	}

	// Build search options
	opts := &finder.Options{
		Pattern:    pattern,
		RepoSpecs:  repoSpecs,
		RepoTypes:  repoTypesTyped,
		IgnoreCase: ignoreCase,
		FullPath:   fullPath,
		Extensions: extensions,
		Excludes:   excludes,
		MinSize:    minSizeBytes,
		MaxSize:    maxSizeBytes,
		ClientOpts: github.ClientOptions{
			DisableCache: noCache,
			CacheDir:     cacheDir,
			CacheTTL:     cacheTTL,
		},
		Jobs: jobs,
	}

	// Create finder and run search
	f := finder.New(cmd.OutOrStdout(), cmd.ErrOrStderr(), colorize)
	return f.Find(ctx, opts)
}
