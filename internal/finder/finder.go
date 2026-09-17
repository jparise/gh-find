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
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/jparise/gh-find/internal/github"
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

	var allRepos []github.Repository

	for _, spec := range opts.RepoSpecs {
		var repos []github.Repository

		// Fetch either the single named repo or all of an owner's repos.
		if spec.Repo != "" {
			r, err := f.client.GetRepo(ctx, spec.Owner, spec.Repo)
			if err != nil {
				f.output.Warningf("%s/%s: %v", spec.Owner, spec.Repo, err)
				continue
			}
			if spec.Ref != "" {
				r.Ref = spec.Ref
				r.ExplicitRef = true
			}
			repos = []github.Repository{r}
		} else {
			repos, err = f.client.ListRepos(ctx, spec.Owner, opts.RepoTypes)
			if err != nil {
				return err
			}
		}

		allRepos = append(allRepos, repos...)
	}

	// The full list of repos could contain duplicates (e.g. the user provided
	// an explicit owner/repo name that was also expanded from owner/*). We
	// deduplicate them while preserving input order.
	seen := make(map[string]bool)
	repos := make([]github.Repository, 0, len(allRepos))
	for _, repo := range allRepos {
		repoKey := repo.FullName + "@" + repo.Ref
		if !seen[repoKey] {
			seen[repoKey] = true
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
	slots := make(chan struct{}, opts.Jobs)

	for _, repo := range repos {
		if err := ctx.Err(); err != nil {
			wg.Wait()
			return err
		}
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		}

		wg.Go(func() {
			defer func() { <-slots }()

			if err := f.searchRepo(ctx, repo, opts); err != nil {
				errorCount.Add(1)
				f.output.Warningf("%s: %v", repo.FullName, err)
			}
		})
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

	return slices.DeleteFunc(slices.Clone(entries), func(entry github.TreeEntry) bool {
		return !slices.Contains(types, github.ParseFileType(entry.Mode))
	})
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

	return slices.DeleteFunc(slices.Clone(entries), func(entry github.TreeEntry) bool {
		matchPath := entry.Path
		if ignoreCase {
			matchPath = strings.ToLower(matchPath)
		}

		ext := filepath.Ext(matchPath)
		return ext == "" || !slices.Contains(extensions, ext)
	})
}

func filterBySize(entries []github.TreeEntry, minSize, maxSize int64) []github.TreeEntry {
	if minSize == 0 && maxSize == 0 {
		return entries
	}

	return slices.DeleteFunc(slices.Clone(entries), func(entry github.TreeEntry) bool {
		return (minSize > 0 && entry.Size < minSize) || (maxSize > 0 && entry.Size > maxSize)
	})
}

func filterByPattern(entries []github.TreeEntry, pattern string, fullPath, ignoreCase bool) ([]github.TreeEntry, error) {
	if ignoreCase {
		pattern = strings.ToLower(pattern)
	}
	if !doublestar.ValidatePattern(pattern) {
		return nil, fmt.Errorf("invalid pattern %q", pattern)
	}

	return slices.DeleteFunc(slices.Clone(entries), func(entry github.TreeEntry) bool {
		matchPath := entry.Path
		if !fullPath {
			matchPath = path.Base(matchPath)
		}
		if ignoreCase {
			matchPath = strings.ToLower(matchPath)
		}

		return !doublestar.MatchUnvalidated(pattern, matchPath)
	}), nil
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
	for _, exclude := range excludes {
		if !doublestar.ValidatePattern(exclude) {
			return nil, fmt.Errorf("invalid exclude pattern %q", exclude)
		}
	}

	return slices.DeleteFunc(slices.Clone(entries), func(entry github.TreeEntry) bool {
		matchPath := entry.Path
		if !fullPath {
			matchPath = path.Base(matchPath)
		}
		if ignoreCase {
			matchPath = strings.ToLower(matchPath)
		}

		for _, excludePattern := range excludes {
			if doublestar.MatchUnvalidated(excludePattern, matchPath) {
				return true
			}
		}
		return false
	}), nil
}

func filterByDate(commits []github.FileCommitInfo, entries []github.TreeEntry, changedAfter, changedBefore *time.Time) []github.TreeEntry {
	if changedAfter == nil && changedBefore == nil {
		return entries
	}

	pathDates := make(map[string]time.Time, len(commits))
	for _, info := range commits {
		pathDates[info.Path] = info.CommittedDate
	}

	return slices.DeleteFunc(slices.Clone(entries), func(entry github.TreeEntry) bool {
		commitDate, ok := pathDates[entry.Path]
		return !ok ||
			(changedAfter != nil && commitDate.Before(*changedAfter)) ||
			(changedBefore != nil && commitDate.After(*changedBefore))
	})
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

	if opts.ChangedAfter != nil || opts.ChangedBefore != nil {
		paths := make([]string, len(entries))
		for i, entry := range entries {
			paths[i] = entry.Path
		}

		commits, err := f.client.GetFileCommitDates(ctx, repo, paths)
		if err != nil {
			return err
		}

		entries = filterByDate(commits, entries, opts.ChangedAfter, opts.ChangedBefore)
	}

	for _, entry := range entries {
		f.output.Match(repo, entry.Path)
	}

	return nil
}
