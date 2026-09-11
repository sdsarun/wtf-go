// Command convert — see docs/01-unit-converter-cli.md for the full specification.
package main

import (
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run implements the CLI end to end and returns the process exit code.
// It is the seam a golden CLI test drives — see docs/01-unit-converter-cli.md's "Test requirements".
//
// TODO(you): implement per docs/01-unit-converter-cli.md:
//   - Requirements / Exact contract
//   - Resolution / precedence order
//   - Case specification (every success and failure row)
func run(args []string, stdout, stderr io.Writer) int {
	panic("TODO: implement run — see docs/01-unit-converter-cli.md")
}
