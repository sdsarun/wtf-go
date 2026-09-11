# Project 04 — Word Frequency Counter

| | |
|---|---|
| **Difficulty** | 2 / 5 |
| **Estimated time** | 4–6 hours |
| **Prerequisites** | [01](01-unit-converter-cli.md)–[03](03-date-duration-calculator.md) |
| **Builds toward** | [09 – mini-grep](09-mini-grep.md), [11 – Generic Containers](11-generic-containers.md) |

## Why this project

Slices and maps are the workhorses of Go, and text processing is where the sharp edges
live: bytes vs runes, non-deterministic map iteration, total-order sorting with
tie-breaks, and reading from "stdin or a list of files" behind one `io.Reader`. The
centrepiece is a **custom `bufio.SplitFunc`** — once you can write one, `bufio.Scanner`
stops being magic.

## Go concepts you MUST use

- [ ] `map[string]int` counting; the non-deterministic iteration order and why you must
      sort before output
- [ ] slices: build, `append`, sort; the `slices` package (`SortFunc`, `SortStableFunc`)
- [ ] `maps` package (`maps.Keys` iterator → `slices.Collect`)
- [ ] `cmp` package: `cmp.Compare`, `cmp.Or` for multi-key ordering
- [ ] `min` / `max` builtins
- [ ] a **custom `bufio.SplitFunc`** passed to `bufio.Scanner.Split`
- [ ] `unicode` (`IsLetter`, `IsDigit`, `IsSpace`) and `unicode/utf8` (`DecodeRune`)
- [ ] `strings` (`ToLower`, `TrimFunc`) and `strings.Builder`
- [ ] `io.Reader`; reading from `os.Stdin` **or** files opened with `os.Open`;
      `io.MultiReader` to concatenate sources
- [ ] all four `for` forms somewhere in the codebase

## Background

A `bufio.SplitFunc` has signature
`func(data []byte, atEOF bool) (advance int, token []byte, err error)`. The scanner calls
it with whatever is currently buffered; you return how many bytes to consume and the token
to emit (or `0, nil, nil` to ask for more data). `bufio.ScanWords` is the built-in
example — yours will define "word" differently.

A **word** here is a maximal run of runes that are letters or digits, with single
apostrophes allowed *between* such runes (so `don't` is one word but `'quoted'` yields
`quoted`). Everything else is a separator. Decode runes with `utf8.DecodeRune`, never index
bytes — `"café"` is 5 bytes, 4 runes.

Map iteration order is randomized per run. Two runs over the same input **must** produce
byte-identical output, so the sort must be a **total order**: primary key, then a
tie-break that is itself total (the word string).

## Requirements

### Functional requirements

1. `wf [flags] [file ...]`. No files, or a file named `-`, means read `os.Stdin`. Multiple
   files are concatenated (in argument order) and counted together.
2. Tokenize into words per the Background definition. Lowercase each word unless
   `-case` is given (`-case` = case-sensitive).
3. Count occurrences. Apply `-min` (drop words with count `< min`, default `1`) and
   `-stopwords FILE` (drop any word listed in that file, one per line, matched after
   lowercasing unless `-case`).
4. Order the surviving words:
   - `-sort count` (default): by count **descending**, then word **ascending**.
   - `-sort alpha`: by word ascending, then count descending.
   - `-sort length`: by rune-length descending, then word ascending, then count descending.
   - `-asc` reverses the **primary** key only (tie-breaks stay as written above).
5. Take the first `-n` rows (`-n` default `20`; `-n 0` means all).
6. Print in `-format`:
   - `table` (default): a header `WORD  COUNT`, then rows `"%-*s  %*d"` where the word
     field width is the longest displayed word and the count field width is the longest
     displayed count. No trailing spaces. Newline-terminated.
   - `list`: `"<count> <word>"` per line.
7. `-stats` adds a block to **stderr** (so it doesn't pollute a piped `list`):
   ```
   files   2
   lines   140
   words   1024
   unique  412
   runes   6001
   ```
   `unique` counts distinct words **before** `-min` / `-stopwords` filtering; `words` is
   the total token count; `runes` counts runes of the raw input (all sources).
8. Empty input (no tokens): print just the header (for `table`) or nothing (for `list`),
   exit 0. `-stats` still prints, all zeros except `files`.

### Exact contract — flags

| Flag | Type | Default | Notes |
|------|------|---------|-------|
| `-n` | int | `20` | rows to print; `0` = all; negative → error |
| `-min` | int | `1` | minimum count; `< 1` → error |
| `-sort` | string | `count` | one of `count`, `alpha`, `length` |
| `-asc` | bool | `false` | reverse the primary sort key |
| `-case` | bool | `false` | case-sensitive counting |
| `-stopwords` | string | `""` | path to a stop-word list |
| `-format` | string | `table` | `table` or `list` |
| `-stats` | bool | `false` | print the stats block to stderr |

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | success (including empty input) |
| `1` | a file could not be opened/read; `-stopwords` file missing; bad flag value (`-n < 0`, `-min < 1`, unknown `-sort`/`-format`) |
| `2` | `flag` package parse error |

### Resolution / precedence order

1. `flag.Parse`; flag syntax error → exit 2.
2. Validate `-n`, `-min`, `-sort`, `-format` → exit 1 on bad values.
3. Load `-stopwords` if set → missing/unreadable → exit 1.
4. Build the combined reader from the file list (open each; first failure → exit 1,
   naming the file).
5. Scan + count + accumulate stats.
6. Filter (`-stopwords`, `-min`), sort, truncate to `-n`.
7. Print report to stdout; if `-stats`, print block to stderr. Exit 0.

### Case specification — SUCCESS

Input string `IN = "The cat sat on the mat. The CAT!"` unless noted.

| # | Command (stdin = `IN`) | stdout | exit |
|---|---|---|---|
| S1 | `wf -n 3` | `WORD  COUNT` / `the       3` / `cat       2` / `mat       1` | 0 |
| S2 | `wf -n 3 -format list` | `3 the` / `2 cat` / `1 mat` | 0 |
| S3 | `wf -case -n 2` | `WORD  COUNT` / `The      2` / `CAT      1`  *(tie broken by word asc: `CAT` < `The`? `C`(67) < `T`(84) → yes)* → actually `CAT`,`The` then `cat`,`mat`,`on`,`sat` all 1; top 2 = `The`(2)... wait `The`=2,`CAT`=1,`cat`=1,`mat`=1,`on`=1,`sat`=1,`The`... hmm `The` appears twice, `the` zero (case-sensitive). Count desc then word asc → `The`(2), then ties at 1 sorted asc: `CAT`,`cat`,`mat`,`on`,`sat`. top 2 → `The`, `CAT` | 0 |
| S4 | `wf -sort alpha -n 4` | `cat`(2), `mat`(1), `on`(1), `sat`(1) | 0 |
| S5 | `wf -sort length -n 2` | words by rune-len desc: `the`/`cat`/`mat`/`sat`/`on` → len3 ties asc then `on`. top 2 = `cat`(2), `mat`(1) | 0 |
| S6 | `wf -sort count -asc -n 2` | count asc, tie word asc: `mat`(1), `on`(1) | 0 |
| S7 | `wf -min 2` | only `the`(3), `cat`(2) | 0 |
| S8 | `wf -n 0 -format list` | all 5 rows | 0 |
| S9 | `printf '' \| wf` | `WORD  COUNT` only | 0 |
| S10 | `printf '' \| wf -format list` | (no output) | 0 |
| S11 | `wf -n 1 -stats` (stderr) | stdout: `the` row; stderr: `files   0` / `lines   1` / `words   8` / `unique  5` / `runes   31` | 0 |
| S12 | `wf don't don't won't` (as a file? no — stdin `"don't don't won't"`) | `don't`(2), `won't`(1) | 0 |
| S13 | stdin `"café Café CAFÉ"`, `wf -n 1` | `café       3` | 0 |
| S14 | stdin `"a-b-c"`, `wf -format list` | `1 a` / `1 b` / `1 c` | 0 |
| S15 | two files `f1`=`"a a b"`, `f2`=`"b c"`; `wf f1 f2 -format list` | `2 a` / `2 b` / `1 c` | 0 |
| S16 | `wf - f2` with stdin `"x"` , `f2`=`"x y"` | `2 x` / `1 y` | 0 |
| S17 | stopwords file `sw`=`"the\non"`; `wf -stopwords sw -format list` (stdin `IN`) | `2 cat` / `1 mat` / `1 sat` | 0 |

### Case specification — FAILURE

| # | Command | stderr | exit |
|---|---|---|---|
| F1 | `wf -n -1` | `error: -n must be >= 0` | 1 |
| F2 | `wf -min 0` | `error: -min must be >= 1` | 1 |
| F3 | `wf -sort frequency` | `error: -sort must be count, alpha, or length` | 1 |
| F4 | `wf -format xml` | `error: -format must be table or list` | 1 |
| F5 | `wf missing.txt` | `error: open missing.txt: no such file or directory` | 1 |
| F6 | `wf -stopwords nope.txt` | `error: open nope.txt: no such file or directory` | 1 |
| F7 | `wf f1 missing.txt` (f1 ok) | `error: open missing.txt: no such file or directory` | 1 |
| F8 | `wf -n` | `flag` pkg: `flag needs an argument: -n` + usage | 2 |

### Error catalogue

| Trigger | Format |
|---|---|
| `-n` | `error: -n must be >= 0` |
| `-min` | `error: -min must be >= 1` |
| `-sort` | `error: -sort must be count, alpha, or length` |
| `-format` | `error: -format must be table or list` |
| file open | `error: %v` (the wrapped `*os.PathError` from `os.Open`) |

## Suggested milestones

1. `internal/wordcount`: the `SplitFunc` (`ScanWords`) + tests against tricky inputs
   (apostrophes, unicode, leading/trailing punctuation, numbers).
2. `Counter` type: `Add(word string)`, holds `map[string]int` + totals.
3. Source assembly: `func sources(paths []string, stdin io.Reader) (io.Reader, closeFn, error)`.
4. Filtering + the three sort modes via `slices.SortStableFunc` + `cmp.Or`.
5. `table` and `list` renderers with computed column widths.
6. `-stats` to stderr; wire `main`.
7. Tests: every S/F row.

## Project layout

```
projects/04-word-frequency/
  cmd/wf/main.go
  internal/wordcount/scan.go     // ScanWords SplitFunc
  internal/wordcount/counter.go  // Counter, filtering, sorting, rendering
  internal/wordcount/scan_test.go
  internal/wordcount/counter_test.go
  cmd/wf/main_test.go
  testdata/                      // sample inputs + golden outputs
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly.
- [ ] Running any success case **twice** gives byte-identical stdout (total-order sort).
- [ ] `ScanWords` never panics on invalid UTF-8 and never splits a multi-byte rune.
- [ ] No business logic in `main`; `run(args, stdin, stdout, stderr) int` seam.
- [ ] `go test -race ./...` passes (even though single-threaded — habit).

## Test requirements

- `TestScanWords` — table: `""`, `"  "`, `"hello"`, `"a,b;c"`, `"don't stop"`,
  `"'quote'"`, `"naïve café"`, `"one\ntwo\tthree"`, `"123 abc123 12.5"` (→ `123`,
  `abc123`, `12`, `5`), invalid UTF-8 bytes.
- `TestSortModes` — the same small counter rendered under all three `-sort` modes and
  `-asc`, asserting full order including tie-breaks.
- `TestRenderTableWidths` — words of differing lengths → aligned columns, no trailing
  spaces.
- `TestSources` — stdin-only, single file, multi-file, `-` mixed with files, missing file.
- `TestStats` — S11 numbers.
- `TestDeterminism` — count a 1000-word input, render twice, `bytes.Equal`.
- `ExampleCounter`.

## Stretch goals

- `-format json` (array of `{ "word": ..., "count": ... }`) — preview of Project 12.
- `-ngram N` — count N-grams instead of single words.
- `-top-per-file` — a column per input file.
- A streaming mode that never holds all tokens, only the map (it already should — verify
  with a huge input and `-memprofile`).
- Unicode normalization awareness: note in the README why `café` (composed) and `café`
  (decomposed `e` + combining acute) count as different words without `golang.org/x/text`.

## Self-check questions

1. Your `SplitFunc` gets `data = []byte("caf")` with `atEOF == false` while the input is
   `"café x"`. What must you return, and why is returning a token here a bug?
2. Why does `for word := range counts` produce a different order each run, and what
   exactly does the Go runtime randomize?
3. `slices.SortFunc` vs `slices.SortStableFunc` — for `-sort count` with a word tie-break,
   does stability matter? What if you *didn't* have a tie-break?
4. `len("café")` is 5, `utf8.RuneCountInString("café")` is 4, `len([]rune("café"))` is 4.
   Which does `-sort length` need, and what does each of the other two measure?
5. You read stdin with `bufio.NewScanner(os.Stdin)`. A 2 MB line with no spaces makes it
   fail. Why, and what's the fix?
6. `cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Word, b.Word))` — walk through
   why swapping `a`/`b` in the first term but not the second gives "count desc, word asc".
