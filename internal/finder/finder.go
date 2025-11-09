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

	for _, entry := range tree.Tree {
		// Apply type filter
		if len(opts.FileTypes) > 0 {
			if !matchesFileType(&entry, opts.FileTypes) {
				continue
			}
		}

		// Apply extension filter
		if len(opts.Extensions) > 0 {
			if !hasExtension(entry.Path, opts.Extensions) {
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
		matchPath := entry.Path
		if !opts.FullPath {
			matchPath = filepath.Base(entry.Path)
		}

		matched, err := matchPattern(opts.Pattern, matchPath, opts.IgnoreCase)
		if err != nil {
			return fmt.Errorf("pattern %q failed to match path %q: %w", opts.Pattern, entry.Path, err)
		}

		if matched {
			excluded := false
			for _, excludePattern := range opts.Excludes {
				isExcluded, err := matchPattern(excludePattern, matchPath, opts.IgnoreCase)
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

// matchPattern matches a path against a glob pattern.
func matchPattern(pattern, path string, ignoreCase bool) (bool, error) {
	if ignoreCase {
		pattern = strings.ToLower(pattern)
		path = strings.ToLower(path)
	}

	return doublestar.Match(pattern, path)
}

// hasExtension checks if a path has one of the specified extensions.
func hasExtension(path string, extensions []string) bool {
	ext := filepath.Ext(path)
	if ext != "" && ext[0] == '.' {
		ext = ext[1:] // Remove leading dot
	}

	for _, e := range extensions {
		// Remove leading dot if present
		if e != "" && e[0] == '.' {
			e = e[1:]
		}
		if ext == e {
			return true
		}
	}
	return false
}

// matchesFileType checks if an entry matches any of the specified file types (OR logic).
func matchesFileType(entry *github.TreeEntry, fileTypes []github.FileType) bool {
	fileType := github.ParseFileType(entry.Mode)
	return slices.Contains(fileTypes, fileType)
}
