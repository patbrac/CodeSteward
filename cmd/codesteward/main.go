// Command codesteward is the CodeSteward CLI entry point.
package main

import (
	"os"

	"github.com/codesteward-ai/codesteward/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}
