# Project 09 — `mini-grep` line filter

| | |
|---|---|
| **Difficulty** | 3 / 5 |
| **Estimated time** | 5–8 hours |
| **Prerequisites** | [01](01-unit-converter-cli.md)–[08](08-expression-calculator.md) |
| **Builds toward** | [15 – Log Pipeline](15-log-pipeline.md), [28 – Static Site Generator](28-static-site-generator.md) |

## Why this project

`grep` is the canonical streaming filter: read an arbitrarily large input **line by line**,
never holding it all in memory, and emit matches as you go. You'll live in `io.Reader` /
`io.Writer` / `bufio`, use `regexp` properly, hold only a small **ring buffer** for
`-B`/`-C` context, and adopt `grep`'s three-way **exit-code** convention (match / no match
/ error).

## Go concepts you MUST use

- [ ] `io.Reader` / `io.Writer` as the working currency; open `os.Stdin` or `os.Open`
      files behind the same interface
- [ ] `bufio.Reader` (`ReadString('\n')`) or `bufio.Scanner` with an enlarged buffer;
      `bufio.Writer` with an explicit `Flush` (via `defer`)
- [ ] `bytes` package for the `-F` fixed-string fast path (`bytes.Contains`,
      `bytes.ToLower`)
- [ ] `regexp` — `Compile`, `MatchString`, `FindAllIndex`, the `(?i)` inline flag,
      `regexp.QuoteMeta`
- [ ] `io.MultiWriter` — the `--tee FILE` option writes matches to stdout **and** a file
- [ ] a bounded **ring buffer** of the last `N` lines for `-B` context (never buffer the
      whole file)
- [ ] `os.Exit` with `grep`'s convention: `0` matched, `1` no match, `2` error
- [ ] `errors` — wrap per-file failures, keep going, remember that an error happened

## Background

**Exit codes (this is load-bearing):**

| Code | Meaning |
|------|---------|
| `0` | at least one selected line was found |
| `1` | no selected line was found (**not** an error — scripts rely on this) |
| `2` | an error occurred (bad pattern, unreadable file, bad flag) — even if some file also matched |

**Lines** are separated by `\n`. A final chunk with no trailing `\n` is still a line. `\r`
is part of the line. Output lines are always `\n`-terminated.

**Filename prefix** is printed when there is more than one input source **or** `-H` is
given, and suppressed by `-h`. Standard input's displayed name is `(standard input)`.

**Context.** `-A N` prints `N` lines after a match, `-B N` before, `-C N` both. A literal
line `--` separates non-adjacent output groups (matching `grep`). Overlapping context is
merged.

## Requirements

### Functional requirements

1. `mgrep [flags] PATTERN [file ...]`. No files, or `-`, → read `os.Stdin`.
2. Pattern is an RE2 regexp unless `-F` (fixed string). Modifiers: `-i` case-insensitive,
   `-w` match on word boundaries (`\bPATTERN\b`), `-x` match the whole line
   (`^(?:PATTERN)$`). These compose with `-F` by escaping first (`regexp.QuoteMeta`).
3. Selection: `-v` invert (select non-matching lines).
4. Output modes (mutually influencing — see order below):
   - default: print each selected line (with prefixes per `-n` / filename rules)
   - `-c`: print only a count per file (`file:count`, or just `count` for one source)
   - `-l`: print only names of files with ≥1 selected line; `-L`: names with none
   - `-o`: print only the matched substrings, one per line (ignored under `-v`)
   - `-q`: print nothing; exit as soon as the first selected line is seen
5. `-n` adds `lineno:` ; `-H` forces filename prefix; `-h` suppresses it.
6. `-A N` / `-B N` / `-C N` context (default 0). `-C` sets both. Invalid (`< 0`) → error.
7. `--color=never|auto|always` (default `auto` = only if stdout is a terminal). When on,
   wrap each match in `\x1b[1;31m` … `\x1b[0m`. Not applied under `-c/-l/-L/-q`.
8. `--tee FILE` — also append every printed output line to `FILE` (via `io.MultiWriter`).
9. Process files in argument order. A file that can't be opened prints
   `mgrep: <name>: <reason>` to stderr, sets the "error occurred" flag, and processing
   continues with the next file. Final exit code is `2` if the flag is set, else `0`/`1`.

### Exact contract — flag summary

| Flag | Meaning | Flag | Meaning |
|---|---|---|---|
| `-F` | fixed string | `-n` | line numbers |
| `-i` | ignore case | `-H` / `-h` | force / suppress filename |
| `-w` | word match | `-o` | only matching parts |
| `-x` | whole-line match | `-q` | quiet (status only) |
| `-v` | invert | `--tee F` | mirror output to a file |
| `-c` | count only | `-A/-B/-C N` | context |
| `-l` / `-L` | files with / without matches | `--color` | `never/auto/always` |

### Resolution / precedence order

1. `flag.Parse`; syntax error → exit 2.
2. `PATTERN` present? else usage, exit 2. Validate `-A/-B/-C >= 0`, `--color` value.
3. Build the final regexp: apply `-F` (`QuoteMeta`), then `-w`, then `-x`, then `-i`.
   Compile; failure → `mgrep: invalid pattern: <err>` to stderr, exit 2.
4. Determine source list; compute whether filename prefixes apply.
5. For each source: stream lines, track matches, emit per the output mode.
6. `-q` short-circuits on first match (exit 0).
7. Exit: `2` if any error flag set; else `0` if any match; else `1`.

### Case specification — SUCCESS / NO-MATCH

Fixtures — `testdata/a.txt`:
```
alpha
beta foo
gamma
delta
epsilon foo bar
```
`testdata/b.txt`:
```
foo at start
nothing here
```

| # | Command | stdout | exit |
|---|---|---|---|
| S1 | `mgrep foo testdata/a.txt` | `beta foo` / `epsilon foo bar` | 0 |
| S2 | `mgrep -n foo testdata/a.txt` | `2:beta foo` / `5:epsilon foo bar` | 0 |
| S3 | `mgrep -n foo testdata/a.txt testdata/b.txt` | `testdata/a.txt:2:beta foo` / `testdata/a.txt:5:epsilon foo bar` / `testdata/b.txt:1:foo at start` | 0 |
| S4 | `mgrep -c foo testdata/a.txt testdata/b.txt` | `testdata/a.txt:2` / `testdata/b.txt:1` | 0 |
| S5 | `mgrep -v foo testdata/a.txt` | `alpha` / `gamma` / `delta` | 0 |
| S6 | `mgrep -l foo testdata/a.txt testdata/b.txt` | `testdata/a.txt` / `testdata/b.txt` | 0 |
| S7 | `mgrep -L foo testdata/a.txt testdata/b.txt` | *(nothing)* | 1 |
| S8 | `mgrep -o 'f.o' testdata/a.txt` | `foo` / `foo` | 0 |
| S9 | `mgrep -w foo testdata/a.txt` | `beta foo` / `epsilon foo bar` | 0 |
| S10 | `mgrep -w oo testdata/a.txt` | *(nothing)* | 1 |
| S11 | `mgrep -x foo testdata/a.txt` | *(nothing)* | 1 |
| S12 | `mgrep -x 'beta foo' testdata/a.txt` | `beta foo` | 0 |
| S13 | `mgrep -i FOO testdata/a.txt` | `beta foo` / `epsilon foo bar` | 0 |
| S14 | `mgrep -F 'foo bar' testdata/a.txt` | `epsilon foo bar` | 0 |
| S15 | `mgrep -F 'a.b' testdata/a.txt` | *(nothing — literal `a.b`)* | 1 |
| S16 | `mgrep -A1 foo testdata/a.txt` | `beta foo` / `gamma` / `--` / `epsilon foo bar` | 0 |
| S17 | `mgrep -B1 foo testdata/a.txt` | `alpha` / `beta foo` / `--` / `delta` / `epsilon foo bar` | 0 |
| S18 | `mgrep -C1 -n foo testdata/a.txt` | `1-alpha` / `2:beta foo` / `3-gamma` / `--` / `4-delta` / `5:epsilon foo bar` | 0 |
| S19 | `printf 'x\nfoo\ny\n' \| mgrep foo` | `foo` | 0 |
| S20 | `printf 'x\ny\n' \| mgrep foo` | *(nothing)* | 1 |
| S21 | `mgrep -q foo testdata/a.txt` | *(nothing)* | 0 |
| S22 | `mgrep -q zzz testdata/a.txt` | *(nothing)* | 1 |
| S23 | `mgrep --color=always foo testdata/b.txt` | `\x1b[1;31mfoo\x1b[0m at start` | 0 |
| S24 | `mgrep --tee out.txt -n foo testdata/a.txt` | S2 output, and `out.txt` contains the same lines | 0 |
| S25 | `mgrep -h -n foo testdata/a.txt testdata/b.txt` | line-numbered, **no** filename prefix | 0 |

### Case specification — FAILURE

| # | Command | stderr | exit |
|---|---|---|---|
| F1 | `mgrep` | usage | 2 |
| F2 | `mgrep '('` | `mgrep: invalid pattern: error parsing regexp: ...` | 2 |
| F3 | `mgrep foo nope.txt` | `mgrep: nope.txt: no such file or directory` | 2 |
| F4 | `mgrep foo testdata/a.txt nope.txt` | `mgrep: nope.txt: no such file or directory` (and a.txt's matches still printed to stdout) | 2 |
| F5 | `mgrep -A -1 foo testdata/a.txt` | `mgrep: -A must be >= 0` | 2 |
| F6 | `mgrep --color=maybe foo x` | `mgrep: --color must be never, auto, or always` | 2 |
| F7 | `mgrep --tee /root/no.txt foo testdata/a.txt` | `mgrep: /root/no.txt: permission denied` | 2 |

### Error catalogue

| Trigger | Format |
|---|---|
| no pattern | usage to stderr |
| bad regexp | `mgrep: invalid pattern: %v` |
| file open | `mgrep: %s: %s` (name, cleaned reason) |
| context flag | `mgrep: -%s must be >= 0` |
| color value | `mgrep: --color must be never, auto, or always` |
| tee open | `mgrep: %s: %s` |

## Suggested milestones

1. `internal/grep`: `Options` struct; `buildRegexp(opts) (*regexp.Regexp, error)` doing
   `-F`/`-w`/`-x`/`-i` composition. Test it hard.
2. `type lineSource` yielding `(lineNo int, text []byte, err error)` from any `io.Reader`,
   streaming, with a configurable max line size.
3. The core loop for the **default** output mode (no context): match, prefix, print via
   `bufio.Writer`.
4. `-c`, `-l`, `-L`, `-o`, `-q`, `-v`.
5. Context: a ring buffer for `-B`, a "print next K lines" counter for `-A`, group
   separators, overlap merging.
6. `--color`, `--tee` (`io.MultiWriter`), filename-prefix logic.
7. `main`: flag parsing, exit-code assembly. Golden CLI test.

## Project layout

```
projects/09-mini-grep/
  cmd/mgrep/main.go
  internal/grep/options.go
  internal/grep/pattern.go    // buildRegexp
  internal/grep/source.go     // streaming line source
  internal/grep/engine.go     // the match loop + all output modes + context
  internal/grep/*_test.go
  cmd/mgrep/main_test.go
  testdata/
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly, including exit codes.
- [ ] A 100 MB generated file is grepped with **flat memory** (`-B 3`): verify with
      `-memprofile` that resident set does not scale with file size.
- [ ] `bufio.Writer` is always `Flush`ed (via `defer`), even on early `-q` exit.
- [ ] Exit code is `2` whenever any file errored, regardless of matches elsewhere.
- [ ] `-o` with a zero-width match (`mgrep -o 'a*' file`) does not infinite-loop.

## Test requirements

- `TestBuildRegexp` — `-F` escapes metacharacters; `-w`/`-x`/`-i` compose; `-F -w` on
  `a.b`.
- `TestEngineModes` — every S row via an in-memory `io.Reader` + `bytes.Buffer` output.
- `TestContext` — `-A`, `-B`, `-C`, adjacent matches merge (no `--`), non-adjacent get
  `--`, match near start/end of file.
- `TestExitCodes` — match → 0, no match → 1, missing file → 2, missing file + match → 2.
- `TestFilenamePrefixRules` — 1 source, 2 sources, `-H`, `-h`, stdin name.
- `TestTee` — the file gets exactly the printed lines.
- `TestHugeLine` — a 5 MB line without `\n` is handled (or errors cleanly, your choice —
  document it).
- `TestZeroWidthMatch` — `-o 'x*'`.
- `ExampleRun`.

## Stretch goals

- `-r` recursive directory search (`filepath.WalkDir`) — but that's really Project 28's
  territory; do it after.
- `-m N` stop after N matches per file.
- `--include`/`--exclude` glob filters.
- `-P` for a tiny subset of PCRE you implement (lookahead) — or just document why RE2
  refuses lookahead.
- Parallel file processing with a worker pool — come back after Project 14.

## Self-check questions

1. `bufio.Scanner` has a default max token size. What is it, what error do you get past
   it, and why might `bufio.Reader.ReadString('\n')` be the safer choice here?
2. `-B 3` needs the last 3 lines at all times but must not grow with the file. Sketch the
   ring buffer: what's its capacity, and what do you do when a match happens at line 1?
3. `-F -w` on pattern `a.b`: what is the actual regexp you compile, and why must
   `QuoteMeta` run *before* you wrap it in `\b…\b`?
4. Exit code 1 means "no match", not "error". Name two shell one-liners that break if you
   return 2 for "no match" instead.
5. `--tee` uses `io.MultiWriter(stdout, file)`. If the file write fails halfway, what
   happens to the stdout write, and is that acceptable for a tee?
6. `regexp.FindAllIndex` on `a*` against `"baaa"` — how many matches, at what offsets, and
   how do you advance past a zero-width match without looping forever?
7. Your default output uses one `bufio.Writer` over `os.Stdout`. Why is an unbuffered
   `fmt.Fprintln(os.Stdout, ...)` per line measurably slower on a big match set?
