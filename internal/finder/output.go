package finder

import (
	"fmt"
	"io"
	"sync"

	"github.com/jparise/gh-find/internal/github"
)

const (
	ansiCyan      = "0;36"
	ansiBoldGreen = "0;1;32"
	ansiWhite     = "0;37"
	ansiYellow    = "0;33"
)

// Output handles all output formatting with optional color and hyperlink support.
type Output struct {
	mu         sync.Mutex
	stdout     io.Writer
	stderr     io.Writer
	colorize   bool
	hyperlinks bool
}

// NewOutput creates a new Output with optional color and hyperlink support.
func NewOutput(stdout, stderr io.Writer, colorize, hyperlinks bool) *Output {
	return &Output{stdout: stdout, stderr: stderr, colorize: colorize, hyperlinks: hyperlinks}
}

func (o *Output) color(code, text string) string {
	if !o.colorize {
		return text
	}
	return "\033[" + code + "m" + text + "\033[0m"
}

func makeHyperlink(url, text string) string {
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, text)
}

// Match writes a file match in the format: owner/repo:path or owner/repo@ref:path.
func (o *Output) Match(repo github.Repository, path string) {
	repoName := repo.Name
	if repo.ExplicitRef {
		repoName += "@" + repo.Ref
	}

	formatted := fmt.Sprintf("%s/%s:%s",
		o.color(ansiCyan, repo.Owner),
		o.color(ansiBoldGreen, repoName),
		o.color(ansiWhite, path))

	if o.hyperlinks {
		url := fmt.Sprintf("%s/blob/%s/%s", repo.URL, repo.Ref, path)
		formatted = makeHyperlink(url, formatted)
	}

	o.mu.Lock()
	fmt.Fprintln(o.stdout, formatted)
	o.mu.Unlock()
}

// Warningf writes a formatted warning message to stderr.
func (o *Output) Warningf(format string, args ...any) {
	o.mu.Lock()
	defer o.mu.Unlock()
	fmt.Fprintf(o.stderr, o.color(ansiYellow, "Warning: ")+format+"\n", args...)
}
