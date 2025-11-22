# gh-find

> A `find(1)`-like utility for GitHub repositories

`gh-find` searches for files across GitHub repositories from the command line. It offers intuitive glob patterns, many filtering options, and sensible defaults.

## Features

- Glob patterns (`**/*.go`, `*.test.js`) instead of regex
- Search specific branches, tags, or commits (`owner/repo@ref`)
- Concurrent search across multiple repositories
- Automatic caching reduces API calls
- Filter by repository type (sources, forks, archives, mirrors)
- Filter by file type (`-t`), extension (`-e`), size, or last modified date
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

### File Matching

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

# Filter by last changed date (files changed in last 2 weeks)
gh find --changed-within 2weeks "*.go" cli/cli

# Filter by last changed date (files not changed in last month)
gh find --changed-before 30days cli/cli

# Combine filters (Go files changed this week over 10KB)
gh find --newer 1week --min-size 10k "*.go" golang/go

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

- `pattern` - Glob pattern (optional, defaults to `*`)
- `repository` - One or more repositories to search:
  - `owner` - All repos for a user or organization (see `--repo-types`)
  - `owner/repo` - Specific repository (default branch)
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

#### Glob Syntax

| Pattern  | Matches                                    | Example                                                 |
|----------|--------------------------------------------|---------------------------------------------------------|
| `*`      | Any sequence of characters (excluding `/`) | `*.go` matches `main.go`, `util.go`                     |
| `**`     | Zero or more directories                   | `**/test/*.go` matches `test/foo.go`, `pkg/test/bar.go` |
| `?`      | Any single character (excluding `/`)       | `file?.go` matches `file1.go`, `fileX.go`               |
| `[abc]`  | Any character in the set                   | `[ft]ile.go` matches `file.go`, `tile.go`               |
| `[a-z]`  | Any character in the range                 | `file[0-9].go` matches `file1.go`, `file9.go`           |
| `[^abc]` | Any character NOT in the set               | `[^t]est.go` matches `best.go`, `rest.go`               |
| `{a,b}`  | Alternatives (one must match)              | `*.{go,md}` matches `file.go`, `README.md`              |

*Note:* `**` must appear as its own path component (surrounded by `/`). Use backslash to escape special characters.

### Options

#### File Filtering
- `-i, --ignore-case` - Case-insensitive pattern matching
- `-p, --full-path` - Match pattern against full path instead of basename
- `-t, --type type` - Filter by file type (can be specified multiple times for OR matching)
  - Valid types: `f`/`file`, `d`/`dir`/`directory`, `l`/`symlink`, `x`/`executable`, `s`/`submodule`
  - Examples: `-t f` (files only), `-t f -t d` (files or directories)
- `-e, --extension ext` - Filter by file extension (can be specified multiple times)
- `-E, --exclude pattern` - Exclude files matching pattern (can be specified multiple times)
- `--min-size size` - Minimum file size (e.g., `1M`, `500k`, `1GB`)
- `--max-size size` - Maximum file size (e.g., `5M`, `1GB`)
- `--changed-within duration` - Filter files changed within duration or since date (e.g., `2weeks`, `1d`, `10h`, `2018-10-27`) [alias: `--newer`]
- `--changed-before duration` - Filter files changed before duration ago or date (e.g., `2weeks`, `1d`, `10h`, `2018-10-27`) [alias: `--older`]

#### Repository Filtering
- `--repo-types type[,type...]` - Repository types to include when expanding owners (default: `sources`)
  - Valid types: `sources`, `forks`, `archives`, `mirrors`, `all`
  - Only affects owner expansion (e.g., `cli` → all repos). Explicitly specified repos (e.g., `cli/archived-fork`) are always included

#### Performance
- `-j, --jobs N` - Maximum concurrent API requests (default: 10, max: 100)

#### Caching
- `--no-cache` - Bypass cache, always fetch fresh data
- `--cache-dir path` - Override cache directory (default: `~/.cache/gh/`)
- `--cache-ttl duration` - Cache time-to-live (default: 24h, e.g., `1h`, `30m`)

#### Output
- `-c, --color mode` - Colorize output: `auto`, `always`, `never` (default: `auto`)
- `--hyperlink mode` - Hyperlink output: `auto`, `always`, `never` (default: `auto`)

## Rate Limits

The GitHub API is rate limited:
- [REST API](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api): 5,000 requests/hour (authenticated), 60/hour (unauthenticated)
- [GraphQL API](https://docs.github.com/en/graphql/overview/rate-limits-and-query-limits-for-the-graphql-api): 5,000 points/hour (authenticated), 0 (unauthenticated, disabled)

Each search uses:
- 1 REST request per repository
- 1 REST request for listing an owner's repos (if needed)

And when commit date filtering is enabled (`--changed-within`/`--changed-before`):
- 1+ GraphQL requests per repository (for commit dates), batched at 100 files per request (e.g., 450 matching files = 5 GraphQL requests)

Local cache hits don't count against any rate limits.

## Common Issues

**API truncation** - [GitHub's Git Trees API](https://docs.github.com/en/rest/git/trees) truncates responses for repositories with >100,000 files or >7MB tree data. Partial results are returned with a warning.

**No repositories found?** - Default `--repo-types sources` excludes forks/archives. Try `--repo-types all`.

**Pattern not matching subdirectories?** - Patterns match basename by default. Use `-p` for full paths.

## License

This software is released under the terms of the [MIT License](LICENSE).
