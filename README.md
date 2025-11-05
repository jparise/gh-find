# gh-find

> A `find(1)`-like utility for GitHub repositories

`gh-find` is a fast, user-friendly way to search for files across GitHub repositories. It supports intuitive glob patterns, multiple filtering options, and sensible defaults.

## Features

- **Intuitive glob patterns**: Use `**/*.go` or `*.test.js` instead of complex regex
- **Concurrent search**: Search multiple repositories in parallel with configurable concurrency
- **Smart caching**: Automatic response caching reduces API calls and respects rate limits
- **Repository filtering**: Search across sources, forks, archives, and mirrors
- **Extension filtering**: Quick filtering by file extension with `-e/--extension`
- **Case-insensitive matching**: Optional case-insensitive pattern matching with `-i`
- **Full-path matching**: Match against full file paths with `-p`, not just basenames

## Installation

### As a GitHub CLI extension

```bash
gh extension install jparise/gh-find
```

### From source

```bash
git clone https://github.com/jparise/gh-find
cd gh-find
go build
gh extension install .
```

## Quick Start

```bash
# Find all Go files in a specific repository
gh find "*.go" cli/cli

# Search across multiple repositories
gh find "*.go" cli/cli cli/go-gh golang/go

# Find all markdown files for a user/org (searches all their repos)
gh find "*.md" torvalds

# Case-insensitive search
gh find -i "readme*" cli

# Match against full paths (great for finding tests)
gh find -p "**/*_test.go" golang/go

# Filter by file extension
gh find -e go -e md cli

# Exclude test files
gh find "*.js" -E "*.test.js" -E "*.spec.js" facebook/react

# Find large files (over 50KB)
gh find --min-size 50k "*.go" golang/go

# Include forks and archives in search
gh find --repo-types sources,forks,archives "*.md" torvalds

# Increase concurrency for faster searches
gh find -j 20 "*.rs" rust-lang/rust

# Bypass cache to get fresh results
gh find --no-cache "*.py" python/cpython
```

## Usage

```
gh-find [<pattern>] <repository>... [flags]
```

### Arguments

- `pattern`: Glob pattern to match files against (optional)
  - When searching a **single repository**, pattern defaults to `*` (all files)
  - When searching **multiple repositories**, the first argument is the pattern
- `repository`: One or more repositories to search. Can be:
  - `owner` - Search all repositories for a user or organization
  - `owner/repo` - Search a specific repository

Examples:
```bash
gh find cli/cli                         # Single repo: pattern defaults to "*"
gh find "*.go" cli/cli                  # Single repo: explicit pattern
gh find "*.go" cli cli/cli golang/go    # Multiple repos: pattern required
gh find "*" cli/cli cli/go-gh           # Multiple repos: all files
```

### Pattern Matching

By default, patterns match against the **basename** (filename) only:

```bash
gh find "*.go" cli/cli                  # Matches any .go file
gh find "main.go" cli/cli               # Matches main.go in any directory
```

Use `-p/--full-path` to match against the full path:

```bash
gh find -p "cmd/**/*.go" cli/cli             # Only .go files in cmd/ directory
gh find -p "**/test/**/*.js" facebook/react  # Only .js files in test/ directories
```

Glob patterns support:
- `*` - Matches any characters except `/`
- `**` - Matches any characters including `/` (directory traversal)
- `?` - Matches any single character
- `[abc]` - Matches one character from the set
- `{a,b}` - Matches either pattern

### Options

#### Pattern Matching
- `-i, --ignore-case` - Case-insensitive pattern matching
- `-p, --full-path` - Match pattern against full path instead of basename
- `-e, --extension EXT` - Filter by file extension (can be specified multiple times)
- `-E, --exclude PATTERN` - Exclude files matching pattern (can be specified multiple times)
- `--min-size SIZE` - Minimum file size (e.g., `1M`, `500k`, `1GB`)
- `--max-size SIZE` - Maximum file size (e.g., `5M`, `1GB`)

#### Repository Filtering
- `--repo-types TYPE[,TYPE...]` - Repository types to include (default: `sources`)
  - Valid types: `sources`, `forks`, `archives`, `mirrors`, `all`
  - Cannot combine `all` with other types
  - Archives are independent (you can have archived forks)

#### Performance
- `-j, --jobs N` - Maximum concurrent API requests (default: 10)
  - Increase for faster searches: `-j 20`
  - Decrease if hitting rate limits: `-j 5`

#### Caching
- `--no-cache` - Bypass cache, always fetch fresh data
- `--cache-dir PATH` - Override cache directory (default: `~/.cache/gh/`)
- `--cache-ttl DURATION` - Cache time-to-live (default: 24h, e.g., `1h`, `30m`)

#### Other
- `--color MODE` - Colorize output: `auto`, `always`, `never` (default: `auto`)

## Repository Filtering

By default, `gh-find` only searches **source repositories** (excludes forks, archives, and mirrors). Use `--repo-types` to customize:

```bash
# Only forks
gh find --repo-types forks "*.md" octocat

# Sources and forks
gh find --repo-types sources,forks "*.go" cli

# Everything including archives
gh find --repo-types all "*.js" facebook

# Only archived repositories
gh find --repo-types archives "*.py" myorg
```

**Note**: Archives are an independent attribute. A repository can be both a fork and archived, so `--repo-types forks,archives` will find both regular forks and archived forks.

## Caching

`gh-find` automatically caches GitHub API responses to improve performance and preserve your rate limit.

### How It Works

- **What's cached**: All GET requests to the GitHub API (repository lists, file trees)
- **Where**: `~/.cache/gh/` by default (shared with `gh` CLI and other extensions), or configurable with `--cache-dir`
- **Duration**: 24 hours by default, configurable with `--cache-ttl`
- **Cache keys**: Based on request method, URL, and authentication

### Performance Impact

Without cache (searching 100 repositories):
- API calls: ~101 requests
- Time: ~60 seconds
- Rate limit cost: 101/5000

With cache (100% hit rate on repeated search):
- API calls: 0 requests
- Time: <1 second
- Rate limit cost: 0/5000

### Cache Control

```bash
# Force fresh data (bypass cache)
gh find --no-cache "*.go" cli/cli

# Custom cache location
gh find --cache-dir /tmp/gh-cache "*.go" cli

# Shorter cache duration (1 hour)
gh find --cache-ttl 1h "*.go" cli
```

You might want to disable the cache when searching recently updated repositories or after pushing new files.

## Performance Tuning

### Concurrency

The `-j/--jobs` flag controls how many repositories are searched concurrently:

```bash
# Default: 10 concurrent requests
gh find "*.go" myorg

# Faster: 20 concurrent requests (uses rate limit faster)
gh find -j 20 "*.go" myorg

# Slower: 5 concurrent requests (conserves rate limit)
gh find -j 5 "*.go" myorg
```

**Recommendations**:
- **Small searches** (1-20 repos): Use default (`-j 10`)
- **Large searches** (50+ repos): Increase to `-j 20` if you have rate limit headroom
- **Rate limit concerns**: Decrease to `-j 5` to spread requests over time

### Rate Limits

GitHub API has rate limits:
- **Authenticated**: 5,000 requests/hour
- **Unauthenticated**: 60 requests/hour

Authentication is handled via `gh auth login`.

Each repository search typically uses:
- 1 request to list repositories (cached)
- 1 request per repository for file tree (cached)

**Tip**: Use caching to minimize rate limit impact. Cached requests don't count against your limit.

## Known Limitations

### Truncated Repositories

GitHub's Git Trees API truncates responses for repositories with >100,000 files or >7MB of tree data. When this happens:

```
Warning: username/repo has >100k files, results incomplete
```

Results will be partial but still useful. This limitation comes from the GitHub API, not `gh-find`.

### Rate Limits

If you exceed the rate limit, you'll see:

```
Rate limit exceeded (0/5000). Resets at 14:23 (in 45m).
```

## Examples

### Find all test files across multiple repositories

```bash
gh find -p "**/*_test.go" golang/go kubernetes/kubernetes
```

### Find README files (case-insensitive) in all repos for an org

```bash
gh find -i "readme*" myorg
```

### Find TypeScript files, excluding node_modules

```bash
gh find -p "**/*.ts" -E "**/node_modules/**" facebook/react
```

### Find larger Go files (over 50KB)

```bash
gh find --min-size 50k -e go golang/go
```

### Find small configuration files (under 10KB)

```bash
gh find --max-size 10k "*.json" -E "**/node_modules/**" cli
```

### Find files in a size range (between 10KB and 100KB)

```bash
gh find --min-size 10k --max-size 100k "*.go" cli/cli
```

### Exclude test and spec files

```bash
gh find "*.js" -E "*.test.js" -E "*.spec.js" cli
```

### Find all Python files in archived repositories

```bash
gh find --repo-types archives -e py myorg
```

### Fast search with increased concurrency

```bash
gh find -j 25 "Dockerfile" kubernetes
```

### Search with fresh data (bypass 24h cache)

```bash
gh find --cache-ttl 0s "*.go" cli/cli
```

## Troubleshooting

### "No repositories match the filter"

Check your `--repo-types` filter. By default, only source repositories are searched (excludes forks, archives, mirrors).

Try: `gh find --repo-types all "*.go" username`

### "Pattern matching not working as expected"

By default, patterns match the **basename** (filename) only. Use `-p/--full-path` to match against the full path:

```bash
# Won't work (matches basename only)
gh find "cmd/*.go" cli/cli

# Works (matches full path)
gh find -p "cmd/*.go" cli/cli
```

### "Rate limit exceeded"

You've hit GitHub's API rate limit (5,000 requests/hour). Options:

1. Wait for the reset time shown in the error
2. Use cached results (cache is enabled by default)
3. Reduce concurrency: `gh find -j 5 ...`

### "Failed to get owner type"

The username or organization doesn't exist, or you don't have access. Verify:

```bash
gh api users/username
```

## License

This software is released under the terms of the [MIT License](LICENSE).
