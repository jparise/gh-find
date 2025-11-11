package finder

import (
	"fmt"
	"io"
	"sync"

	"github.com/jparise/gh-find/internal/github"
	"github.com/mgutz/ansi"
)

// Output handles all output formatting with optional color and hyperlink support.
type Output struct {
	mu         sync.Mutex
	stdout     io.Writer
	stderr     io.Writer
	hyperlinks bool

	cyan   func(string) string
	green  func(string) string
	white  func(string) string
	yellow func(string) string
	red    func(string) string
}

// NewOutput creates a new Output with optional color and hyperlink support.
func NewOutput(stdout, stderr io.Writer, colorize, hyperlinks bool) *Output {
	color := func(name string) func(string) string {
		if colorize {
			return ansi.ColorFunc(name)
		}
		return ansi.ColorFunc("")
	}

	return &Output{
		stdout:     stdout,
		stderr:     stderr,
		hyperlinks: hyperlinks,
		cyan:       color("cyan"),
		green:      color("green+b"),
		white:      color("white"),
		yellow:     color("yellow"),
		red:        color("red+b"),
	}
}

func makeHyperlink(url, text string) string {
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, text)
}

// Match writes a file match in the format: owner/repo:path.
func (o *Output) Match(repo github.Repository, path string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	formatted := fmt.Sprintf("%s/%s:%s",
		o.cyan(repo.Owner),
		o.green(repo.Name),
		o.white(path))

	if o.hyperlinks {
		url := fmt.Sprintf("%s/blob/%s/%s", repo.URL, repo.DefaultBranch, path)
		formatted = makeHyperlink(url, formatted)
	}

	fmt.Fprintf(o.stdout, "%s\n", formatted)
}

// Warningf writes a formatted warning message to stderr.
func (o *Output) Warningf(format string, args ...any) {
	o.mu.Lock()
	defer o.mu.Unlock()
	fmt.Fprintf(o.stderr, o.yellow("Warning: ")+format+"\n", args...)
}

// Infof writes a formatted informational message to stderr.
func (o *Output) Infof(format string, args ...any) {
	o.mu.Lock()
	defer o.mu.Unlock()
	fmt.Fprintf(o.stderr, format+"\n", args...)
}
