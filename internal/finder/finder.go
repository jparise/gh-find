// Package finder orchestrates file search across GitHub repositories.
package finder

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"sync"

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
	// Initialize GitHub client
	client, err := github.NewClient(opts.ClientOpts)
	if err != nil {
		return err
	}
	f.client = client

	// Get repositories to search from all repo specs
	var allRepos []*github.Repository

	for _, repoSpec := range opts.RepoSpecs {
		// Parse repo spec
		owner, repo, err := parseRepoSpec(repoSpec)
		if err != nil {
			f.output.Warningf("Invalid repo spec %q: %v", repoSpec, err)
			continue
		}

		// Get repositories for this spec
		var specRepos []*github.Repository
		if repo != "" {
			// Single repo
			r, err := f.client.GetRepo(ctx, owner, repo)
			if err != nil {
				f.output.Warningf("Failed to fetch %s: %v", repoSpec, err)
				continue
			}
			specRepos = []*github.Repository{r}
		} else {
			// All repos for user/org (API calls are cached by go-gh)
			specRepos, err = f.client.ListRepos(ctx, owner, opts.RepoTypes)
			if err != nil {
				f.output.Warningf("Failed to list repos for %s: %v", owner, err)
				continue
			}
		}

		allRepos = append(allRepos, specRepos...)
	}

	// Deduplicate repos by full name (in case user specified same repo multiple times)
	repoMap := make(map[string]*github.Repository)
	for _, repo := range allRepos {
		repoMap[repo.FullName] = repo
	}
	repos := make([]*github.Repository, 0, len(repoMap))
	for _, repo := range repoMap {
		repos = append(repos, repo)
	}

	if len(repos) == 0 {
		f.output.Infof("No repositories match the filter")
		return nil
	}

	// Process repositories concurrently with bounded parallelism
	var wg sync.WaitGroup
	sem := semaphore.NewWeighted(int64(opts.Jobs))

	for _, repo := range repos {
		if err := sem.Acquire(ctx, 1); err != nil {
			return err
		}

		wg.Add(1)
		go func(repo *github.Repository) {
			defer wg.Done()
			defer sem.Release(1)

			if err := f.searchRepo(ctx, repo, opts); err != nil {
				f.output.Warningf("%s: %v", repo.FullName, err)
			}
		}(repo)
	}

	wg.Wait()
	return nil
}

func (f *Finder) searchRepo(ctx context.Context, repo *github.Repository, opts *Options) error {
	tree, err := f.client.GetTree(ctx, repo)
	if err != nil {
		return err
	}

	if tree.Truncated {
		f.output.Warningf("%s has >100k files, results incomplete", repo.FullName)
	}

	pattern := opts.Pattern
	excludes := opts.Excludes
	extensions := opts.Extensions

	if opts.IgnoreCase {
		pattern = strings.ToLower(pattern)
		excludes = make([]string, len(excludes))
		for i, exclude := range opts.Excludes {
			excludes[i] = strings.ToLower(exclude)
		}
		extensions = make([]string, len(opts.Extensions))
		for i, ext := range opts.Extensions {
			extensions[i] = strings.ToLower(ext)
		}
	}

	for _, entry := range tree.Tree {
		// Apply type filter
		if len(opts.FileTypes) > 0 {
			if !matchesFileType(&entry, opts.FileTypes) {
				continue
			}
		}

		path := entry.Path
		if !opts.FullPath {
			path = filepath.Base(path)
		}
		if opts.IgnoreCase {
			path = strings.ToLower(path)
		}

		// Apply extension filter
		if len(extensions) > 0 {
			ext := filepath.Ext(path)
			if ext == "" || !slices.Contains(extensions, ext) {
				continue
			}
		}

		// Apply size filter
		if opts.MinSize > 0 && entry.Size < opts.MinSize {
			continue
		}
		if opts.MaxSize > 0 && entry.Size > opts.MaxSize {
			continue
		}

		// Apply pattern matching
		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			return fmt.Errorf("pattern %q failed to match path %q: %w", opts.Pattern, entry.Path, err)
		}

		if matched {
			excluded := false
			for _, excludePattern := range excludes {
				isExcluded, err := doublestar.Match(excludePattern, path)
				if err != nil {
					return fmt.Errorf("exclude pattern %q failed to match path %q: %w",
						excludePattern, entry.Path, err)
				}
				if isExcluded {
					excluded = true
					break
				}
			}

			if !excluded {
				f.output.Match(repo.Owner, repo.Name, repo.DefaultBranch, entry.Path)
			}
		}
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

// matchesFileType checks if an entry matches any of the specified file types (OR logic).
func matchesFileType(entry *github.TreeEntry, fileTypes []github.FileType) bool {
	fileType := github.ParseFileType(entry.Mode)
	return slices.Contains(fileTypes, fileType)
}
