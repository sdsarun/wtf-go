#!/usr/bin/env bash
# scripts/new-project.sh — scaffold the generic, project-agnostic part of a new
# projects/NN-slug/ directory, following the convention documented in docs/README.md.
#
# It generates a blank, compilable skeleton — every function body is
# panic("TODO: ...") pointing back at that project's docs/NN-slug.md brief. No
# solution logic is ever generated here.
#
# Usage:
#   scripts/new-project.sh <NN> <slug> <binary> <package>
#
#   NN        two-digit project number, e.g. 02
#   slug      the project's directory slug, matching docs/NN-slug.md, e.g. bit-toolkit
#   binary    the cmd/ binary name, e.g. bits
#   package   the internal/ package name, e.g. bitkit
#
# Example:
#   scripts/new-project.sh 02 bit-toolkit bits bitkit
#
# After running, open docs/NN-slug.md and hand-add that project's specific named
# types/functions to internal/<package>/<package>.go — the part that differs per
# project, since only the project's own brief says what belongs there. See
# projects/01-unit-converter/ for a worked example of that step.

set -euo pipefail

usage() {
	sed -n '2,25p' "$0" | sed 's/^# \{0,1\}//'
}

if [ "$#" -ne 4 ]; then
	usage >&2
	exit 2
fi

NN="$1"
SLUG="$2"
BINARY="$3"
PACKAGE="$4"

case "$NN" in
'' | *[!0-9]*)
	echo "error: NN must be numeric, got \"$NN\"" >&2
	exit 2
	;;
esac

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="$REPO_ROOT/projects/${NN}-${SLUG}"

# The project directory slug (matching that doc's own "Project layout" section, e.g.
# projects/01-unit-converter/) doesn't always match the doc's filename slug (e.g.
# docs/01-unit-converter-cli.md) — the curriculum docs use longer, more descriptive
# filenames than their internal directory examples. Find the real doc file by NN prefix
# instead of assuming the two slugs are identical.
DOC=""
for candidate in "$REPO_ROOT"/docs/"${NN}"-*.md; do
	if [ -e "$candidate" ]; then
		DOC="docs/$(basename "$candidate")"
		break
	fi
done
if [ -z "$DOC" ]; then
	DOC="docs/${NN}-${SLUG}.md"
	echo "warning: no docs/${NN}-*.md found — using $DOC; double-check NN" >&2
fi

if [ -e "$DIR" ]; then
	echo "error: $DIR already exists — refusing to overwrite" >&2
	exit 1
fi

mkdir -p "$DIR/cmd/$BINARY" "$DIR/internal/$PACKAGE"

cat >"$DIR/cmd/$BINARY/main.go" <<EOF
// Command $BINARY — see $DOC for the full specification.
package main

import (
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run implements the CLI end to end and returns the process exit code.
// It is the seam a golden CLI test drives — see $DOC's "Test requirements".
//
// TODO(you): implement per $DOC:
//   - Requirements / Exact contract
//   - Resolution / precedence order
//   - Case specification (every success and failure row)
func run(args []string, stdout, stderr io.Writer) int {
	panic("TODO: implement run — see $DOC")
}
EOF

cat >"$DIR/cmd/$BINARY/main_test.go" <<EOF
package main

import "testing"

// TestCLI should be table-driven over every row of $DOC's Case specification
// (both the SUCCESS and FAILURE tables) — see "Test requirements".
func TestCLI(t *testing.T) {
	t.Skip("TODO(you): implement — see $DOC \"Test requirements\"")
}
EOF

cat >"$DIR/internal/$PACKAGE/$PACKAGE.go" <<EOF
// Package $PACKAGE implements the core logic behind $BINARY.
// See $DOC for the full specification — start with "Go concepts you MUST use"
// and "Requirements".
package $PACKAGE

// TODO(you): add the types and functions $DOC names (check its "Project layout"
// comments and any signatures given in its Requirements/Background/self-check
// sections), each stubbed with panic("TODO: ...") until implemented.
EOF

cat >"$DIR/README.md" <<EOF
# $NN — $SLUG

Full specification: [$DOC](../../$DOC)

## What it is

_TODO(you): one paragraph — what this program does._

## How to run

\`\`\`
go run ./cmd/$BINARY -- ...
\`\`\`

_TODO(you): fill in real examples once implemented._

## Design notes

_TODO(you): why the packages are split the way they are._
EOF

cat >"$DIR/Makefile" <<'EOF'
.PHONY: build test race vet fmt cover clean
build:  ; go build ./...
test:   ; go test ./...
race:   ; go test -race ./...
vet:    ; go vet ./...
fmt:    ; gofmt -l .
cover:  ; go test -coverprofile=cover.out ./... && go tool cover -func=cover.out
clean:  ; rm -f cover.out *.prof
EOF

echo "created $DIR:"
find "$DIR" -type f | sort
