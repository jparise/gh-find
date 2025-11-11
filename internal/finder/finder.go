// Package finder orchestrates file search across GitHub repositories.
package finder

import (
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/jparise/gh-find/internal/github"
	"golang.org/x/sync/semaphore"
)

// Finder orchestrates the file finding process.
type Finder struct {
	output *Output
	client *github.Client
}

// New creates a new Finder.
func New(stdout, stderr io.Writer, colorize, hyperlinks bool) *Finder {
	return &Finder{
		output: NewOutput(stdout, stderr, colorize, hyperlinks),
	}
}

// Find executes the search based on the provided options.
func (f *Finder) Find(ctx context.Context, opts *Options) error {
	client, err := github.NewClient(opts.ClientOpts)
	if err != nil {
		return err
	}
	f.client = client

	// Get repositories to search from all repo specs
	var allRepos []github.Repository

	for _, repoSpec := range opts.RepoSpecs {
		owner, repo, err := parseRepoSpec(repoSpec)
		if err != nil {
			return err
		}

		// Fetch either the single named repo or all of an owners repos.
		var specRepos []github.Repository
		if repo != "" {
			r, err := f.client.GetRepo(ctx, owner, repo)
			if err != nil {
				f.output.Warningf("%s/%s: %v", owner, repo, err)
				continue
			}
			specRepos = []github.Repository{r}
		} else {
			specRepos, err = f.client.ListRepos(ctx, owner, opts.RepoTypes)
			if err != nil {
				return err
			}
		}

		allRepos = append(allRepos, specRepos...)
	}

	// The full list of repos could contain duplicates (e.g. the user provided
	// an explicit owner/repo name that was also expanded from owner/*). We
	// deduplicate them while preserving input order.
	seen := make(map[string]bool)
	repos := make([]github.Repository, 0, len(allRepos))
	for _, repo := range allRepos {
		if !seen[repo.FullName] {
			seen[repo.FullName] = true
			repos = append(repos, repo)
		}
	}

	if len(repos) == 0 {
		f.output.Warningf("No repositories match the filter")
		return nil
	}

	// Process repositories concurrently with bounded parallelism
	var wg sync.WaitGroup
	var errorCount atomic.Int32
	sem := semaphore.NewWeighted(int64(opts.Jobs))

	for _, repo := range repos {
		if err := sem.Acquire(ctx, 1); err != nil {
			wg.Wait()
			return err
		}

		wg.Add(1)
		go func(repo github.Repository) {
			defer wg.Done()
			defer sem.Release(1)

			if err := f.searchRepo(ctx, repo, opts); err != nil {
				errorCount.Add(1)
				f.output.Warningf("%s: %v", repo.FullName, err)
			}
		}(repo)
	}

	wg.Wait()

	if int(errorCount.Load()) == len(repos) {
		return fmt.Errorf("failed to search all %d repositories", len(repos))
	}

	return nil
}

func filterByType(entries []github.TreeEntry, types []github.FileType) []github.TreeEntry {
	if len(types) == 0 {
		return entries
	}

	var filtered []github.TreeEntry
	for _, entry := range entries {
		fileType := github.ParseFileType(entry.Mode)
		if slices.Contains(types, fileType) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func filterByExtension(entries []github.TreeEntry, extensions []string, ignoreCase bool) []github.TreeEntry {
	if len(extensions) == 0 {
		return entries
	}

	if ignoreCase {
		normalized := make([]string, len(extensions))
		for i, ext := range extensions {
			normalized[i] = strings.ToLower(ext)
		}
		extensions = normalized
	}

	var filtered []github.TreeEntry
	for _, entry := range entries {
		matchPath := entry.Path
		if ignoreCase {
			matchPath = strings.ToLower(matchPath)
		}

		ext := filepath.Ext(matchPath)
		if ext != "" && slices.Contains(extensions, ext) {
			filtered = append(filtered, entry)
		}
	}

	return filtered
}

func filterBySize(entries []github.TreeEntry, minSize, maxSize int64) []github.TreeEntry {
	if minSize == 0 && maxSize == 0 {
		return entries
	}

	filtered := make([]github.TreeEntry, 0, len(entries))
	for _, entry := range entries {
		if minSize > 0 && entry.Size < minSize {
			continue
		}
		if maxSize > 0 && entry.Size > maxSize {
			continue
		}
		filtered = append(filtered, entry)
	}

	return filtered
}

func filterByPattern(entries []github.TreeEntry, pattern string, fullPath, ignoreCase bool) ([]github.TreeEntry, error) {
	if ignoreCase {
		pattern = strings.ToLower(pattern)
	}

	var filtered []github.TreeEntry
	for _, entry := range entries {
		matchPath := entry.Path
		if !fullPath {
			matchPath = path.Base(matchPath)
		}
		if ignoreCase {
			matchPath = strings.ToLower(matchPath)
		}

		matched, err := doublestar.Match(pattern, matchPath)
		if err != nil {
			return nil, fmt.Errorf("pattern %q failed to match path %q: %w", pattern, entry.Path, err)
		}

		if matched {
			filtered = append(filtered, entry)
		}
	}

	return filtered, nil
}

func filterByExcludes(entries []github.TreeEntry, excludes []string, fullPath, ignoreCase bool) ([]github.TreeEntry, error) {
	if len(excludes) == 0 {
		return entries, nil
	}

	if ignoreCase {
		normalized := make([]string, len(excludes))
		for i, exclude := range excludes {
			normalized[i] = strings.ToLower(exclude)
		}
		excludes = normalized
	}

	var filtered []github.TreeEntry
	for _, entry := range entries {
		matchPath := entry.Path
		if !fullPath {
			matchPath = path.Base(matchPath)
		}
		if ignoreCase {
			matchPath = strings.ToLower(matchPath)
		}

		excluded := false
		for _, excludePattern := range excludes {
			isExcluded, err := doublestar.Match(excludePattern, matchPath)
			if err != nil {
				return nil, fmt.Errorf("exclude pattern %q failed to match path %q: %w",
					excludePattern, entry.Path, err)
			}
			if isExcluded {
				excluded = true
				break
			}
		}

		if !excluded {
			filtered = append(filtered, entry)
		}
	}

	return filtered, nil
}

func (f *Finder) searchRepo(ctx context.Context, repo github.Repository, opts *Options) error {
	tree, err := f.client.GetTree(ctx, repo)
	if err != nil {
		return err
	}

	if tree.Truncated {
		f.output.Warningf("%s: exceeds GitHub's API limit (100k files or 7MB) - results are incomplete", repo.FullName)
	}

	entries := tree.Tree
	entries = filterByType(entries, opts.FileTypes)
	entries = filterByExtension(entries, opts.Extensions, opts.IgnoreCase)
	entries = filterBySize(entries, opts.MinSize, opts.MaxSize)

	entries, err = filterByPattern(entries, opts.Pattern, opts.FullPath, opts.IgnoreCase)
	if err != nil {
		return err
	}

	entries, err = filterByExcludes(entries, opts.Excludes, opts.FullPath, opts.IgnoreCase)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		f.output.Match(repo, entry.Path)
	}

	return nil
}

// parseRepoSpec parses "owner" or "owner/repo" format.
func parseRepoSpec(spec string) (owner, repo string, err error) {
	parts := strings.Split(spec, "/")
	switch len(parts) {
	case 1:
		return parts[0], "", nil
	case 2:
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf("invalid repo spec: %s (expected username or username/repo)", spec)
	}
}
