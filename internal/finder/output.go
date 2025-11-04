package finder

import (
	"fmt"
	"io"
	"sync"

	"github.com/mgutz/ansi"
)

// Output handles all output formatting with optional color support.
type Output struct {
	mu     sync.Mutex
	stdout io.Writer
	stderr io.Writer

	cyan   func(string) string
	green  func(string) string
	white  func(string) string
	yellow func(string) string
	red    func(string) string
}

// NewOutput creates a new Output with optional color support.
func NewOutput(stdout, stderr io.Writer, colorize bool) *Output {
	color := func(name string) func(string) string {
		if colorize {
			return ansi.ColorFunc(name)
		}
		return ansi.ColorFunc("")
	}

	return &Output{
		stdout: stdout,
		stderr: stderr,
		cyan:   color("cyan"),
		green:  color("green+b"), // bold green
		white:  color("white"),
		yellow: color("yellow"),
		red:    color("red+b"), // bold red
	}
}

// Match writes a file match in the format: owner/repo:path.
func (o *Output) Match(owner, repo, path string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	fmt.Fprintf(o.stdout, "%s/%s:%s\n",
		o.cyan(owner),
		o.green(repo),
		o.white(path))
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
