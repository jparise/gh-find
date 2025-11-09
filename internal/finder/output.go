package finder

import (
	"fmt"
	"io"
	"sync"

	"github.com/cli/go-gh/v2/pkg/auth"
	"github.com/mgutz/ansi"
)

// Output handles all output formatting with optional color and hyperlink support.
type Output struct {
	mu         sync.Mutex
	stdout     io.Writer
	stderr     io.Writer
	hostname   string
	hyperlinks bool

	cyan   func(string) string
	green  func(string) string
	white  func(string) string
	yellow func(string) string
	red    func(string) string
}

// NewOutput creates a new Output with optional color and hyperlink support.
func NewOutput(stdout, stderr io.Writer, colorize, hyperlinks bool) *Output {
	hostname, _ := auth.DefaultHost()

	color := func(name string) func(string) string {
		if colorize {
			return ansi.ColorFunc(name)
		}
		return ansi.ColorFunc("")
	}

	return &Output{
		stdout:     stdout,
		stderr:     stderr,
		hostname:   hostname,
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
func (o *Output) Match(owner, repo, branch, path string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	formatted := fmt.Sprintf("%s/%s:%s",
		o.cyan(owner),
		o.green(repo),
		o.white(path))

	if o.hyperlinks {
		url := fmt.Sprintf("https://%s/%s/%s/blob/%s/%s", o.hostname, owner, repo, branch, path)
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
