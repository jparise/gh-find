# gh-find

> A `find(1)`-like utility for GitHub repositories

`gh-find` searches for files across GitHub repositories from the command line. It offers intuitive glob patterns, many filtering options, and sensible defaults.

## Features

- Glob patterns (`**/*.go`, `*.test.js`) instead of regex
- Search specific branches, tags, or commits (`owner/repo@ref`)
- Concurrent search across multiple repositories
- Automatic caching reduces API calls
- Filter by repository type (sources, forks, archives, mirrors)
- Filter by file type (`-t`), extension (`-e`), or size
- Case-insensitive (`-i`) and full-path (`-p`) matching

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

## Examples

### Basic Usage

```bash
# Find all files in a repository
gh find cli/cli

# Find Go files in specific repository
gh find "*.go" cli/cli

# Search multiple repositories
gh find "*.go" cli/cli golang/go

# Search all repos under an owner (user or organization)
gh find "*.md" torvalds
```

### Branches, Tags, and Commits

```bash
# Search a specific branch
gh find "*.go" cli/cli@trunk

# Search a tag
gh find "*.go" cli/cli@v2.40.0

# Search a commit SHA
gh find "*.go" golang/go@abc123def

# Search different refs in different repos
gh find "*.go" cli/cli@main golang/go@release-branch.go1.21
```

### Pattern Matching

```bash
# Case-insensitive search
gh find -i "readme*" cli

# Match against full paths (e.g., find tests)
gh find -p "**/*_test.go" golang/go

# Exclude patterns
gh find "*.js" -E "*.test.js" -E "*.spec.js" facebook/react
```

### Filtering

```bash
# Filter by extension
gh find -e go -e md cli

# Filter by type (files only, no directories)
gh find -t f "README*" cli

# Filter by type (executables)
gh find -t x "*.sh" cli/cli

# Filter by size (files over 50KB)
gh find --min-size 50k "*.go" golang/go

# Include forks and archives (default only searches source repos)
gh find --repo-types sources,forks,archives "*.md" cli
```

### Sorting Results

```bash
# Sort alphabetically
gh find "*.go" cli/cli | sort

# Reverse order
gh find "*.go" cli/cli | sort -r

# Sort by repo name, then path
gh find "*.go" cli golang/go | sort -t: -k1,1 -k2
```

## Usage

```
gh find [<pattern>] <repository>... [flags]
```

### Arguments

- `pattern` - Glob pattern (optional, defaults to `*` for single repo)
- `repository` - One or more repositories to search:
  - `owner` - All repos for a user or organization
  - `owner/repo` - Specific repository (uses default branch)
  - `owner/repo@ref` - Specific repository at branch, tag, or commit SHA

```bash
gh find cli/cli                      # Single repo: defaults to "*"
gh find "*.go" cli/cli               # Single repo: explicit pattern
gh find "*.go" cli golang/go         # Multiple repos: pattern required
gh find "*.go" cli/cli@trunk         # Specific repository branch
gh find "*.go" cli/cli@v2.40.0       # Specific repository tag
```

### Pattern Matching

Patterns match **basename** (filename) by default. Use `-p/--full-path` for full path matching.

```bash
# Basename (default)
gh find "*.go" cli/cli               # Matches any .go file
gh find "main.go" cli/cli            # Matches main.go in any directory

# Full path
gh find -p "cmd/**/*.go" cli/cli     # Only .go files in cmd/
```

Glob syntax: `*` (any chars), `**` (with `/`), `?` (single char), `[abc]` (char set), `{a,b}` (alternatives)

### Options

#### Pattern Matching
- `-i, --ignore-case` - Case-insensitive pattern matching
- `-p, --full-path` - Match pattern against full path instead of basename
- `-t, --type type` - Filter by file type (can be specified multiple times for OR matching)
  - Valid types: `f`/`file`, `d`/`dir`/`directory`, `l`/`symlink`, `x`/`executable`, `s`/`submodule`
  - Examples: `-t f` (files only), `-t f -t d` (files or directories)
- `-e, --extension ext` - Filter by file extension (can be specified multiple times)
- `-E, --exclude pattern` - Exclude files matching pattern (can be specified multiple times)
- `--min-size size` - Minimum file size (e.g., `1M`, `500k`, `1GB`)
- `--max-size size` - Maximum file size (e.g., `5M`, `1GB`)

#### Repository Filtering
- `--repo-types type[,type...]` - Repository types to include when expanding owners (default: `sources`)
  - Valid types: `sources`, `forks`, `archives`, `mirrors`, `all`
  - Only affects owner expansion (e.g., `cli` → all repos). Explicitly specified repos (e.g., `cli/archived-fork`) are always included
  - Examples: `--repo-types sources,forks,archives` or `--repo-types all`

#### Performance
- `-j, --jobs N` - Maximum concurrent API requests (default: 10, max: 100)
  - Increase for faster searches: `-j 20`
  - Decrease if hitting rate limits: `-j 5`

#### Caching
- `--no-cache` - Bypass cache, always fetch fresh data
- `--cache-dir path` - Override cache directory (default: `~/.cache/gh/`)
- `--cache-ttl duration` - Cache time-to-live (default: 24h, e.g., `1h`, `30m`)

#### Output
- `-c, --color mode` - Colorize output: `auto`, `always`, `never` (default: `auto`)
- `--hyperlink mode` - Hyperlink output: `auto`, `always`, `never` (default: `auto`)
  - `auto` only enables hyperlinks when color is also enabled

## Caching

API responses are cached automatically to improve performance and reduce rate limit usage.

- **What**: All GET requests (repository lists, file trees)
- **Where**: `~/.cache/gh/` (configurable with `--cache-dir`)
- **TTL**: 24 hours (configurable with `--cache-ttl`)

```bash
# Bypass cache for fresh results
gh find --no-cache "*.go" cli/cli

# Custom cache location or TTL
gh find --cache-dir /tmp/cache "*.go" cli
gh find --cache-ttl 1h "*.go" cli
```

## Performance

Control concurrency with `-j/--jobs` (default: 10):

```bash
gh find -j 20 "*.go" cli    # Faster (uses rate limit faster)
gh find -j 5 "*.go" cli     # Slower (conserves rate limit)
```

GitHub API rate limits:
- Authenticated: 5,000 requests/hour
- Unauthenticated: 60 requests/hour

Each search uses 1 request per repository (plus 1 for listing repos). Cached requests don't count against limits.

## Known Limitations

[GitHub's Git Trees API](https://docs.github.com/en/rest/git/trees) truncates responses for repositories with >100,000 files or >7MB tree data. Results will be partial with a warning.

## Troubleshooting

**"No repositories match the filter"** - Check `--repo-types`. Default excludes forks, archives, mirrors. Try `--repo-types all`.

**"Pattern matching not working"** - Patterns match basename by default. Use `-p` for full path matching:
```bash
gh find -p "cmd/*.go" cli/cli   # Full path
gh find "*.go" cli/cli          # Basename
```

**"Rate limit exceeded"** - Wait for reset, use cache (enabled by default), or reduce concurrency with `-j 5`.

**"Failed to get owner type"** - Username/org doesn't exist or no access. Verify with `gh api users/username`.

## License

This software is released under the terms of the [MIT License](LICENSE).
