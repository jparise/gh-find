// gh-find is a find(1)-like utility for discovering files across GitHub repositories.
package main

import (
	"fmt"
	"os"

	"github.com/jparise/gh-find/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
