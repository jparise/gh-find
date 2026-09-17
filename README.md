# gh-find

> A `find(1)`-like utility for GitHub repositories

`gh-find` searches for files across GitHub repositories from the command line. It offers [intuitive pattern matching](#pattern-matching), [many filtering options](#options), and sensible, performance-minded defaults. It works well in shell pipelines, CI checks, and [agent workflows](#using-with-ai-agents) that need to locate code across repos without cloning them.

![demo](demo.gif)

## Installation

### As a GitHub CLI extension

```bash
gh extension install jparise/gh-find
```

### From source

Requires Go 1.26 or later.

```bash
git clone https://github.com/jparise/gh-find
cd gh-find
go build
gh extension install .
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
- `--cache-dir path` - Override the platform-specific GitHub CLI cache directory
- `--cache-ttl duration` - Cache time-to-live (default: 24h, e.g., `1h`, `30m`)

#### Output
- `-c, --color mode` - Colorize output: `auto`, `always`, `never` (default: `auto`)
- `--hyperlink mode` - Hyperlink output: `auto`, `always`, `never` (default: `auto`)

## Examples

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

## Using with AI Agents

`gh-find` fits naturally into agent workflows that need to locate code across repositories without cloning them. Filters like `--type`, `--extension`, and `--exclude` keep results small and on-topic, which preserves the agent's context window. Local caching (see `--cache-ttl`) makes iterative runs cheap.

For example, an agent asked to audit GitHub Actions workflows across an org can narrow the search before reading any files:

```bash
gh find -p ".github/workflows/*.{yml,yaml}" myorg
```

## Rate Limits

`gh-find` uses your GitHub CLI authentication. Standard API limits are:

- [REST API](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api): 5,000 requests/hour
- [GraphQL API](https://docs.github.com/en/graphql/overview/rate-limits-and-query-limits-for-the-graphql-api): 5,000 points/hour

Each uncached search uses:

- For an explicit repository: 1 REST request for metadata and 1 for its recursive tree
- For owner expansion: 1 REST request to detect the owner type, 1+ paginated listing requests, and 1 recursive tree request per selected repository

When commit date filtering is enabled (`--changed-within`/`--changed-before`), each repository also uses 1 GraphQL request per batch of up to 100 matching paths (e.g., 450 matching paths in one repository = 5 requests).

Local cache hits don't count against any rate limits.

## Common Issues

**API truncation** - [GitHub's Git Trees API](https://docs.github.com/en/rest/git/trees) truncates responses for repositories with >100,000 files or >7MB tree data. Partial results are returned with a warning.

**No repositories found?** - Default `--repo-types sources` excludes forks/archives. Try `--repo-types all`.

**Pattern not matching subdirectories?** - Patterns match basename by default. Use `-p` for full paths.

## License

This software is released under the terms of the [MIT License](LICENSE).
