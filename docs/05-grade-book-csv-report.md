# Project 05 — Grade Book CSV Report

| | |
|---|---|
| **Difficulty** | 2 / 5 |
| **Estimated time** | 5–7 hours |
| **Prerequisites** | [01](01-unit-converter-cli.md)–[04](04-word-frequency-counter.md) |
| **Builds toward** | [12 – JSON Config Loader](12-json-config-loader.md), [13 – Report Engine](13-report-engine.md) |

## Why this project

This is your struct-and-method project. You will define a small domain model, choose
value vs pointer receivers deliberately, use struct **embedding** for promotion, and sort
the same data two different ways — once with `sort.Interface` and once with
`slices.SortFunc` — so the trade-off is concrete. You will also use `encoding/csv` in both
directions and `text/tabwriter` for aligned output, and you will finally have a
legitimate reason to write `fallthrough`.

## Go concepts you MUST use

- [ ] `struct` types for the domain model; at least one **anonymous struct** (in tests)
- [ ] methods with **value** receivers (pure computations) and **pointer** receivers
      (mutation / large structs) — and a comment on each explaining the choice
- [ ] **struct embedding** and field/method promotion (e.g. embed a `Stats` struct)
- [ ] a **method value** and a **method expression** used somewhere (e.g.
      `Student.Percent` as a method expression)
- [ ] `encoding/csv` — `csv.Reader` (`FieldsPerRecord`), `csv.Writer` (`Flush`, `Error`)
- [ ] `text/tabwriter` for the report table
- [ ] `sort.Interface` (`Len`/`Less`/`Swap` on a named slice type) **and**
      `slices.SortFunc` — one for the student table, one for assignment detail
- [ ] `fmt.Stringer` on the letter-grade type
- [ ] `switch` with **`fallthrough`** (the `-notes` cascade, see below)

## Background

**Input CSV** (tidy / long format), header required:

```
student,assignment,score,max
Alice,HW1,18,20
Alice,HW2,25,25
```

`score >= 0`, `max > 0`; `score > max` is allowed (extra credit). A duplicate
`(student, assignment)` pair is an error.

**Percentage.** Without weights: `sum(score) / sum(max) * 100`. With `-weights FILE`
(`assignment,weight` rows): the weighted mean of `score/max` per assignment, with weights
renormalized over the assignments that student actually has.

**Letter scale** (percentage → letter):

| ≥97 | ≥93 | ≥90 | ≥87 | ≥83 | ≥80 | ≥77 | ≥73 | ≥70 | ≥67 | ≥63 | ≥60 | else |
|-----|-----|-----|-----|-----|-----|-----|-----|-----|-----|-----|-----|------|
| A+  | A   | A-  | B+  | B   | B-  | C+  | C   | C-  | D+  | D   | D-  | F    |

**`-drop-lowest N`.** Per student, drop the `N` assignments with the lowest `score/max`
ratio (ties broken by assignment name ascending) **before** computing — but only if the
student has more than `N` assignments.

**`-notes` cascade** (this is where `fallthrough` earns its place):

```go
switch {
case pct >= 90:
    notes = append(notes, "excellent")
    fallthrough
case pct >= 70:
    notes = append(notes, "passing")
    fallthrough
case pct >= 0:
    notes = append(notes, "enrolled")
}
```

An A student's notes are `excellent, passing, enrolled`; a C student's are
`passing, enrolled`; an F student's are `enrolled`.

**Class stats:** mean, median, and **population** standard deviation (`÷ n`) of the student
percentages; min and max with the owning student's name.

## Requirements

### Functional requirements

1. `gradebook [flags]`. Input from `-in FILE` or, if absent, `os.Stdin`.
2. Parse the CSV. Errors report the 1-based record line number.
3. Build the model: `Student` → list of `Score{Assignment, Points, Max}`.
4. Apply `-drop-lowest`, then `-weights`, then compute each student's earned / possible /
   percent / letter.
5. Default view: the student table + `CLASS SUMMARY` + `GRADE DISTRIBUTION` (non-zero
   buckets only, in scale order A+ → F).
6. `-student NAME`: print only that student's detail view (assignments sorted by name,
   then the dropped list). Unknown name → error, exit 1.
7. `-sort name|percent|grade` (default `name`): `name` ascending; `percent` descending
   then name ascending; `grade` best-first (A+ → F) then name ascending.
8. `-format table|csv` (default `table`). `csv` writes only the per-student data rows
   (header `student,earned,possible,percent,grade`, plus `notes` if `-notes`), no summary.
9. `-notes` adds a `NOTES` column / field.
10. All floating output uses `%.2f`. Determinism: every list is fully sorted before print.

### Exact contract — flags

| Flag | Type | Default | Notes |
|------|------|---------|-------|
| `-in` | string | `""` | input CSV path; empty → stdin |
| `-weights` | string | `""` | weights CSV path |
| `-drop-lowest` | int | `0` | `< 0` → error |
| `-student` | string | `""` | detail view for one student |
| `-sort` | string | `name` | `name` / `percent` / `grade` |
| `-format` | string | `table` | `table` / `csv` |
| `-notes` | bool | `false` | include the notes cascade |

### Exact contract — table output

Using dataset `testdata/grades.csv` (9 rows: Alice HW1 18/20, HW2 25/25, Final 25/25; Bob
12/20, 10/25, 10/15; Carol 20/20, 20/25, 15/15), `gradebook -in testdata/grades.csv`:

```
STUDENT  EARNED  POSSIBLE  PERCENT  GRADE
Alice        68        70    97.14  A+
Bob          32        60    53.33  F
Carol        55        60    91.67  A-

CLASS SUMMARY
students  3
mean      80.71
median    91.67
stddev    19.49
min       53.33 (Bob)
max       97.14 (Alice)

GRADE DISTRIBUTION
A+  1
A-  1
F   1
```

(Column widths come from `tabwriter` with padding 2, right-aligned numeric columns; the
student table is one `tabwriter` block, the summary and distribution are plain text.)

### Exact contract — detail view

`gradebook -in testdata/grades.csv -student Alice`:

```
Alice — 97.14% (A+)
ASSIGNMENT  SCORE  MAX  PCT
Final          25   25  100.00
HW1            18   20   90.00
HW2            25   25  100.00
dropped: (none)
```

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | success |
| `1` | CSV parse/validation error, unknown `-student`, bad flag value, missing/unreadable file |
| `2` | `flag` parse error |

### Resolution / precedence order

1. `flag.Parse` → exit 2 on syntax error.
2. Validate `-drop-lowest >= 0`, `-sort`, `-format` → exit 1.
3. Open `-in` (or stdin) and `-weights` if set → exit 1 on failure.
4. Parse + validate rows (header, field count, numeric fields, `max > 0`, `score >= 0`,
   no duplicate pair) → exit 1 with line number.
5. If `-weights`: every weighted assignment must exist in the data? No — extra weight
   entries are ignored; an assignment with no weight entry when `-weights` is set → error.
6. Build model, drop-lowest, compute.
7. If `-student`: resolve name (exact match, case-sensitive) → not found → exit 1.
8. Render; exit 0.

### Case specification — SUCCESS

| # | Command | key assertion | exit |
|---|---|---|---|
| S1 | `gradebook -in testdata/grades.csv` | the table block above, verbatim | 0 |
| S2 | `gradebook -in testdata/grades.csv -sort percent` | rows Alice, Carol, Bob | 0 |
| S3 | `gradebook -in testdata/grades.csv -sort grade` | rows Alice (A+), Carol (A-), Bob (F) | 0 |
| S4 | `gradebook -in testdata/grades.csv -student Alice` | the detail block above | 0 |
| S5 | `gradebook -in testdata/grades.csv -student Bob` | `Bob — 53.33% (F)` + 3 assignment rows + `dropped: (none)` | 0 |
| S6 | `gradebook -in testdata/grades.csv -format csv` | `student,earned,possible,percent,grade` then 3 data rows | 0 |
| S7 | `gradebook -in testdata/grades.csv -format csv -notes` | header has trailing `,notes`; Alice row ends `...,A+,"excellent, passing, enrolled"` | 0 |
| S8 | `gradebook -in testdata/grades.csv -notes` | table has a `NOTES` column | 0 |
| S9 | `gradebook -in testdata/grades.csv -drop-lowest 1` | Bob drops HW2 (10/25 = 0.40, lowest); his percent rises; Alice/Carol drop their lowest too | 0 |
| S10 | `cat testdata/grades.csv \| gradebook` | same as S1 (stdin) | 0 |
| S11 | `gradebook -in testdata/one_student.csv` | summary with `students  1`, `stddev  0.00`, median == mean | 0 |
| S12 | `gradebook -in testdata/grades.csv -weights testdata/weights.csv` | weighted percentages (weights: HW1 0.2, HW2 0.2, Final 0.6) | 0 |
| S13 | `gradebook -in testdata/extra_credit.csv` (a score 22/20) | percent may exceed 100; grade `A+`; no error | 0 |

### Case specification — FAILURE

| # | Command | stderr | exit |
|---|---|---|---|
| F1 | `gradebook -in nope.csv` | `error: open nope.csv: no such file or directory` | 1 |
| F2 | `gradebook -in testdata/bad_header.csv` | `error: line 1: expected header "student,assignment,score,max"` | 1 |
| F3 | `gradebook -in testdata/short_row.csv` | `error: line 4: expected 4 fields, got 3` | 1 |
| F4 | `gradebook -in testdata/bad_score.csv` | `error: line 3: score "xx" is not a number` | 1 |
| F5 | `gradebook -in testdata/zero_max.csv` | `error: line 3: max must be > 0, got 0` | 1 |
| F6 | `gradebook -in testdata/neg_score.csv` | `error: line 3: score must be >= 0, got -5` | 1 |
| F7 | `gradebook -in testdata/dup_pair.csv` | `error: line 5: duplicate entry for Alice / HW1` | 1 |
| F8 | `gradebook -in testdata/grades.csv -student Zed` | `error: no student named "Zed"` | 1 |
| F9 | `gradebook -in testdata/grades.csv -drop-lowest -1` | `error: -drop-lowest must be >= 0` | 1 |
| F10 | `gradebook -in testdata/grades.csv -sort median` | `error: -sort must be name, percent, or grade` | 1 |
| F11 | `gradebook -in testdata/grades.csv -weights testdata/partial_weights.csv` | `error: assignment "Final" has no weight` | 1 |

### Error catalogue

| Trigger | Format |
|---|---|
| file open | `error: %v` (`*os.PathError`) |
| bad header | `error: line 1: expected header %q` |
| field count | `error: line %d: expected 4 fields, got %d` |
| non-numeric | `error: line %d: %s %q is not a number` |
| max range | `error: line %d: max must be > 0, got %s` |
| score range | `error: line %d: score must be >= 0, got %s` |
| duplicate | `error: line %d: duplicate entry for %s / %s` |
| unknown student | `error: no student named %q` |
| drop-lowest | `error: -drop-lowest must be >= 0` |
| sort | `error: -sort must be name, percent, or grade` |
| format | `error: -format must be table or csv` |
| missing weight | `error: assignment %q has no weight` |

## Suggested milestones

1. `internal/gradebook`: types (`Score`, `Student`, `Letter` with `String()`), the CSV
   parser with line-numbered errors.
2. `Student.Earned()`, `Possible()`, `Percent()` (value receivers, pure);
   `(*Student).addScore` (pointer, mutation). `LetterFor(pct) Letter`.
3. `-drop-lowest` and `-weights`.
4. `ClassStats` struct (embedded into the report model); mean/median/population stddev.
5. `byName` type implementing `sort.Interface`; `slices.SortFunc` for the other two modes
   and for the detail assignment list. Note which you'd keep in real code.
6. Renderers: `tabwriter` table + summary + distribution; `csv.Writer`; detail view.
7. `-notes` cascade. Wire `main`.
8. Build `testdata/` fixtures for every S/F row; write the golden CLI test.

## Project layout

```
projects/05-grade-book/
  cmd/gradebook/main.go
  internal/gradebook/model.go     // Score, Student, Letter, Stringer
  internal/gradebook/parse.go     // CSV in, weights in, line-numbered errors
  internal/gradebook/compute.go   // drop-lowest, weights, percent, ClassStats
  internal/gradebook/sortmodes.go // byName sort.Interface + SortFunc variants
  internal/gradebook/render.go    // table / csv / detail
  internal/gradebook/*_test.go
  cmd/gradebook/main_test.go
  testdata/*.csv
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly (fixtures committed under `testdata/`).
- [ ] Each method has a one-line comment justifying value vs pointer receiver.
- [ ] The student table is sorted, the detail assignments are sorted, the distribution is
      in scale order — output is deterministic across runs.
- [ ] `csv.Writer` errors are checked (`w.Flush(); if err := w.Error()`).
- [ ] `Letter` implements `fmt.Stringer` and is used through the interface at least once.

## Test requirements

- `TestParse` — happy path + F2–F7 (each with a fixture).
- `TestPercentNoWeights` / `TestPercentWeights` — S1 numbers and S12 numbers.
- `TestDropLowest` — S9; also the "fewer assignments than N → drop nothing" case.
- `TestClassStats` — mean 80.71, median 91.67, population stddev 19.49; and the
  single-student case (stddev 0).
- `TestSortModes` — S1/S2/S3 orders including name tie-breaks.
- `TestNotesCascade` — pct 95 → 3 notes, 75 → 2, 40 → 1, 0 → 1.
- `TestRenderTable` / `TestRenderCSV` / `TestRenderDetail` — golden strings.
- `ExampleLetter_String`.

## Stretch goals

- `-format json` (preview of Project 12).
- `-histogram` — an ASCII bar chart of the grade distribution.
- Category weights: `weights.csv` has `assignment,category` and a second file
  `categories.csv` has `category,weight`.
- `-curve N` — add `N` points to every percentage, capped at 100, before lettering.
- A `gradebook diff old.csv new.csv` mode showing who moved up/down a letter.

## Self-check questions

1. `Student.Percent()` has a value receiver; `(*Student).addScore` has a pointer
   receiver. What breaks if you swap them?
2. You embedded `ClassStats` into your report struct. What does field promotion give you,
   and what happens if `ClassStats` also has a field named `Mean` and so does the outer
   struct?
3. `sort.Interface` needs three methods on a named slice type; `slices.SortFunc` needs one
   closure. When is the interface version still worth it?
4. `Student.Percent` (method expression) has type `func(Student) float64`; `s.Percent`
   (method value) has type `func() float64`. Where did you use each, and why?
5. `csv.Reader` with `FieldsPerRecord = 4` — what does it do on a row with 3 fields, and
   how is that different from you checking `len(record)` yourself?
6. Your `-notes` cascade uses `fallthrough`. Rewrite it without `fallthrough`. Which
   version would you keep, and is this one of the rare cases where `fallthrough` reads
   better?
7. Population vs sample standard deviation: which did the spec ask for, what's the
   formula difference, and why does the single-student case make it obvious?
