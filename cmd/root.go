// Package cmd implements the command-line interface for gh-find.
package cmd

import (
	"fmt"
	"math"
	"os"
	"os/signal"
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

const (
	maxJobs = 100
)

// outputMode represents when to enable output features (color, hyperlinks, etc).
type outputMode string

const (
	outputAuto   outputMode = "auto"
	outputAlways outputMode = "always"
	outputNever  outputMode = "never"
)

func (m *outputMode) String() string {
	return string(*m)
}

func (m *outputMode) Set(v string) error {
	switch v {
	case "auto", "always", "never":
		*m = outputMode(v)
		return nil
	default:
		return fmt.Errorf("must be one of \"auto\", \"always\", or \"never\"")
	}
}

func (m *outputMode) Type() string {
	return "mode"
}

type fileTypesFlag []github.FileType

func (f *fileTypesFlag) String() string {
	if f == nil || len(*f) == 0 {
		return ""
	}
	strs := make([]string, len(*f))
	for i, ft := range *f {
		strs[i] = string(ft)
	}
	return strings.Join(strs, ",")
}

func (f *fileTypesFlag) Set(v string) error {
	switch v {
	case "f", "file":
		*f = append(*f, github.FileTypeFile)
	case "d", "dir", "directory":
		*f = append(*f, github.FileTypeDirectory)
	case "l", "symlink":
		*f = append(*f, github.FileTypeSymlink)
	case "x", "executable":
		*f = append(*f, github.FileTypeExecutable)
	case "s", "submodule":
		*f = append(*f, github.FileTypeSubmodule)
	default:
		return fmt.Errorf("must be one of f, file, d, dir, directory, l, symlink, x, executable, s, submodule")
	}
	return nil
}

func (f *fileTypesFlag) Type() string {
	return "filetype"
}

type extensionsFlag []string

func (e *extensionsFlag) String() string {
	if e == nil || len(*e) == 0 {
		return ""
	}
	return strings.Join(*e, ",")
}

func (e *extensionsFlag) Set(v string) error {
	// Normalize to ensure it starts with a dot
	if !strings.HasPrefix(v, ".") {
		v = "." + v
	}
	*e = append(*e, v)
	return nil
}

func (e *extensionsFlag) Type() string {
	return "ext"
}

type repoTypesFlag github.RepoTypes

func (f *repoTypesFlag) String() string {
	if f == nil {
		return string(github.RepoTypeSources)
	}

	return github.RepoTypes(*f).String()
}

func (f *repoTypesFlag) Set(v string) error {
	parts := strings.SplitSeq(v, ",")

	for part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if part == "all" {
			*f = repoTypesFlag(github.RepoTypes{}.All())
			return nil
		}

		switch github.RepoType(part) {
		case github.RepoTypeSources:
			f.Sources = true
		case github.RepoTypeForks:
			f.Forks = true
		case github.RepoTypeArchives:
			f.Archives = true
		case github.RepoTypeMirrors:
			f.Mirrors = true
		default:
			return fmt.Errorf("invalid repo type %q: must be one of %s, or all", part, strings.Join(github.ValidRepoTypes, ", "))
		}
	}

	return nil
}

func (f *repoTypesFlag) Type() string {
	return "types"
}

type jobsCount int

func (j *jobsCount) Set(s string) error {
	v, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	if v < 1 || v > maxJobs {
		return fmt.Errorf("must be between 1 and %d", maxJobs)
	}
	*j = jobsCount(v)
	return nil
}

func (j *jobsCount) String() string {
	return strconv.Itoa(int(*j))
}

func (j *jobsCount) Type() string {
	return "count"
}

type byteSize int64

func (b *byteSize) Set(s string) error {
	size, err := parseByteSize(s)
	if err != nil {
		return err
	}
	if size <= 0 {
		return fmt.Errorf("must be greater than 0")
	}
	*b = byteSize(size)
	return nil
}

func (b *byteSize) String() string {
	if *b == 0 {
		return ""
	}
	return strconv.FormatInt(int64(*b), 10)
}

func (b *byteSize) Type() string {
	return "size"
}

var (
	version = "dev"

	color      = outputAuto
	hyperlink  = outputAuto
	repoTypes  = repoTypesFlag{Sources: true}
	fileTypes  fileTypesFlag
	ignoreCase bool
	fullPath   bool
	extensions extensionsFlag
	excludes   []string
	minSize    byteSize
	maxSize    byteSize
	noCache    bool
	cacheDir   string
	cacheTTL   time.Duration
	jobs       = jobsCount(10)
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
  gh find -p "**/*_test.go" golang/go
  gh find "*" cli/cli cli/go-gh
  gh find -e go -e md cli
  gh find --min-size 50k "*.go" golang/go
  gh find "*.js" -E "*.test.js" -E "*.spec.js" facebook/react
  gh find --min-size 10k --max-size 100k "*.go" cli/cli`,
	Version: version,
	Args:    cobra.MinimumNArgs(1),
	RunE:    run,
}

func init() {
	rootCmd.Flags().SortFlags = false

	// Pattern matching
	rootCmd.Flags().BoolVarP(&ignoreCase, "ignore-case", "i", false,
		"case-insensitive pattern matching")
	rootCmd.Flags().BoolVarP(&fullPath, "full-path", "p", false,
		"match pattern against full path")

	// File filtering
	rootCmd.Flags().VarP(&fileTypes, "type", "t",
		"filter by file type: f/file, d/dir/directory, l/symlink, x/executable, s/submodule")
	rootCmd.Flags().VarP(&extensions, "extension", "e",
		"filter by file extension (can be specified multiple times)")
	rootCmd.Flags().StringSliceVarP(&excludes, "exclude", "E", []string{},
		"exclude patterns (can be specified multiple times)")
	rootCmd.Flags().Var(&minSize, "min-size",
		"minimum file size (e.g., 1M, 500k, 1GB)")
	rootCmd.Flags().Var(&maxSize, "max-size",
		"maximum file size (e.g., 5M, 1GB)")

	// Repository selection
	rootCmd.Flags().Var(&repoTypes, "repo-types",
		"repo types when expanding owners (sources,forks,archives,mirrors,all)")

	// Output control
	rootCmd.Flags().VarP(&color, "color", "c",
		"colorize output: auto, always, never")
	rootCmd.Flags().Var(&hyperlink, "hyperlink",
		"hyperlink output: auto, always, never")

	// Performance & caching
	rootCmd.Flags().VarP(&jobs, "jobs", "j",
		"maximum concurrent API requests")
	rootCmd.Flags().BoolVar(&noCache, "no-cache", false,
		"bypass cache, always fetch fresh data")
	rootCmd.Flags().StringVar(&cacheDir, "cache-dir", "",
		"override cache directory location")
	rootCmd.Flags().DurationVar(&cacheTTL, "cache-ttl", 24*time.Hour,
		"cache time-to-live (e.g., 1h, 30m, 24h)")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// parseByteSize parses a human-readable size string into bytes.
// Supports formats like "1M", "500k", "1024" (plain bytes).
// Units are case-insensitive and use binary (1024-based) multipliers.
func parseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size string")
	}

	// Find where the unit starts (last non-digit character)
	i := len(s) - 1
	for i >= 0 && !unicode.IsDigit(rune(s[i])) {
		i--
	}

	// Parse the number part
	numStr := s[:i+1]
	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q: %w", numStr, err)
	}
	if num < 0 {
		return 0, fmt.Errorf("size cannot be negative")
	}

	// Parse the unit suffix
	unit := strings.ToLower(strings.TrimSpace(s[i+1:]))
	var multiplier int64
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

	if num > math.MaxInt64/multiplier {
		return 0, fmt.Errorf("size too large (exceeds max int64)")
	}

	return num * multiplier, nil
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

	terminal := term.FromEnv()

	var colorize bool
	switch color {
	case outputAlways:
		colorize = true
	case outputNever:
		colorize = false
	case outputAuto:
		colorize = terminal.IsColorEnabled()
	}

	var hyperlinks bool
	switch hyperlink {
	case outputAlways:
		hyperlinks = true
	case outputNever:
		hyperlinks = false
	case outputAuto:
		hyperlinks = terminal.IsColorEnabled() && colorize
	}

	// Validate that min <= max if both specified
	if minSize > 0 && maxSize > 0 && minSize > maxSize {
		return fmt.Errorf("--min-size cannot be greater than --max-size")
	}

	// Build search options
	opts := &finder.Options{
		Pattern:    pattern,
		RepoSpecs:  repoSpecs,
		RepoTypes:  github.RepoTypes(repoTypes),
		FileTypes:  []github.FileType(fileTypes),
		IgnoreCase: ignoreCase,
		FullPath:   fullPath,
		Extensions: []string(extensions),
		Excludes:   excludes,
		MinSize:    int64(minSize),
		MaxSize:    int64(maxSize),
		ClientOpts: github.ClientOptions{
			DisableCache: noCache,
			CacheDir:     cacheDir,
			CacheTTL:     cacheTTL,
		},
		Jobs: int(jobs),
	}

	// Create finder and run search
	f := finder.New(cmd.OutOrStdout(), cmd.ErrOrStderr(), colorize, hyperlinks)
	return f.Find(ctx, opts)
}
