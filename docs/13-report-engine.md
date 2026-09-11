# Project 13 — Report / Plugin Engine

| | |
|---|---|
| **Difficulty** | 3 / 5 |
| **Estimated time** | 9–13 hours |
| **Prerequisites** | [08](08-expression-calculator.md), [11](11-generic-containers.md), [12](12-json-config-loader.md) |
| **Builds toward** | [20 – REST API](20-rest-api.md), [26 – Reverse Proxy](26-reverse-proxy.md), [29 – Task Scheduler](29-task-scheduler.md) |

## Why this project

Interfaces are how Go does polymorphism and plugin architecture. You'll build a
`Source → []Transform → Renderer` pipeline where every stage is an interface, plugins
**self-register** in `init()`, cell values are `any` and rendered via a **type switch**,
and a `Money` type implements `fmt.Formatter`. It reuses Project 08's expression evaluator
for the `filter`/`derive` transforms — a satisfying callback.

## Go concepts you MUST use

- [ ] several small interfaces (`Source`, `Transform`, `Renderer`) and one **composed**
      interface (`type Pipeline interface { … }` or embedding `io.Writer` in a renderer
      interface)
- [ ] the empty interface / `any` — a `Cell` is `any`; a `Record` is `[]Cell` + a schema
- [ ] **type switches** and comma-ok **type assertions** (`if s, ok := v.(fmt.Stringer)`)
- [ ] a **registry** populated from `init()` in each plugin file; `RegisterSource` /
      `RegisterTransform` / `RegisterRenderer` panic on duplicate names
- [ ] `fmt.Stringer` on an enum type; `fmt.Formatter` on `Money` (handle `%v`, `%+v`,
      `%d`, `%.2f`, width, the `#` flag)
- [ ] `sort.Interface` + `sort.Stable` for the `sort` transform (stability matters for
      multi-key sorts)
- [ ] pluggable `io.Writer` sinks — render to stdout, a file, or a `gzip.Writer`
- [ ] variadic constructors / options
- [ ] reuse `learning/go/projects/08-calc/internal/calc` for expression transforms

## Background

**Data model.**

```go
type Type int // TypeString, TypeInt, TypeFloat, TypeBool, TypeTime, TypeNull
type Column struct { Name string; Type Type }
type Cell = any                       // string | int64 | float64 | bool | time.Time | nil | fmt.Stringer
type Record []Cell
type Table struct { Cols []Column; Rows []Record }
```

**Interfaces.**

```go
type Source interface {
    Read(ctx context.Context) (*Table, error)
}
type Transform interface {
    Name() string
    Apply(*Table) (*Table, error)
}
type Renderer interface {
    Render(w io.Writer, t *Table) error
}
```

**Registry.** Each plugin file:

```go
func init() { RegisterSource("csv", func(cfg json.RawMessage) (Source, error) { … }) }
```

`report sources` lists `keys(registry)` sorted — proof that every `init()` ran.

**CSV type inference:** try `int64`, then `float64`, then `bool` (`true`/`false`), then
`time.RFC3339`, else `string`. An empty field is `nil`. A column's `Type` is the
most-general type seen (any `nil` allowed alongside).

## Requirements

### Built-in plugins

**Sources** (`{"type": "...", ...}`):

| type | config | behaviour |
|---|---|---|
| `csv` | `{"path": "f.csv", "header": true}` | read + infer types |
| `json` | `{"path": "f.json"}` | a JSON array of objects → rows (union of keys → columns) |
| `range` | `{"n": 100, "cols": ["i","sq"]}` | rows `i, i*i` — deterministic test source |
| `inline` | `{"cols": [...], "rows": [[...],...]}` | literal table |

**Transforms:**

| type | config | behaviour |
|---|---|---|
| `select` | `{"cols": ["a","b"]}` | keep/reorder columns; unknown col → error |
| `rename` | `{"map": {"old":"new"}}` | rename columns |
| `filter` | `{"where": "amount > 100 && region == \"EU\""}` | keep rows where the expression (evaluated per row, columns as variables) is truthy |
| `derive` | `{"as": "net", "expr": "amount * 0.9"}` | add a computed column |
| `sort` | `{"by": ["region","amount"], "desc": [false,true]}` | stable multi-key sort |
| `limit` | `{"n": 10, "offset": 0}` | slice rows |
| `aggregate` | `{"group_by": ["region"], "aggs": [{"col":"amount","fn":"sum","as":"total"}]}` | `fn` ∈ `sum avg count min max`; output columns = group keys + agg columns, one row per group, groups sorted by key |

**Renderers:**

| type | config | output |
|---|---|---|
| `table` | `{}` | `tabwriter` grid with a header + a `---` rule row |
| `csv` | `{}` | `encoding/csv`, RFC 4180 |
| `json` | `{"pretty": true}` | array of objects |
| `markdown` | `{}` | a GitHub table |
| `template` | `{"text": "{{range .Rows}}...{{end}}"}` | `text/template` with `.Cols` / `.Rows` |

### The CLI (`report`)

| Command | Behaviour |
|---|---|
| `report run PIPELINE.json [-o OUT] [-gzip]` | build + execute; write to stdout or `-o`; `-gzip` wraps the writer |
| `report sources` / `report transforms` / `report renderers` | list registered plugin names, one per line, sorted |
| `report explain PIPELINE.json` | print the resolved pipeline as `source → t1 → t2 → … → renderer` with each stage's config |
| `report schema PIPELINE.json` | run only the source + transforms' schema effects (no rows) and print the final `Column` list |

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | success |
| `1` | pipeline error — unknown plugin, bad plugin config, source read error, transform error (unknown column, bad expression, bad `fn`), render error |
| `2` | usage — no/unknown subcommand, missing PIPELINE arg, unknown flag |

### Exact contract — `table` renderer

Source `range` n=3 cols `["i","sq"]`, transform `derive as="label" expr="i + sq"`:

```
I  SQ  LABEL
-  --  -----
0  0   0
1  1   2
2  4   6
```

(Left-aligned via `tabwriter`, padding 2; the rule row repeats `-` to each column's
header width.)

### Exact contract — `Money` formatting

`Money` wraps cents (`int64`). With `-money total` on a `table` render, a `total` value of
`123456` cents prints:

| verb | output |
|---|---|
| `%v` / `%s` | `$1,234.56` |
| `%d` | `123456` (cents) |
| `%.2f` | `1234.56` |
| `%+v` | `USD 1234.56` |
| `%#v` | `report.Money(123456)` |
| `%10v` | `right-padded to width 10` |

### Case specification — SUCCESS

Fixtures under `testdata/`. `sales.csv` has columns `region,product,amount` with 8 rows
across regions EU/US/APAC.

| # | Command | key assertion | exit |
|---|---|---|---|
| S1 | `report sources` | `csv` / `inline` / `json` / `range` (sorted, one per line) | 0 |
| S2 | `report transforms` | `aggregate` … `sort` (7 names, sorted) | 0 |
| S3 | `report renderers` | `csv` / `json` / `markdown` / `table` / `template` | 0 |
| S4 | `report run testdata/p_topregions.json` | table of regions by descending total, top 3 | 0 |
| S5 | `report run testdata/p_topregions.json -o out.csv` (renderer csv) | `out.csv` is valid RFC-4180 CSV | 0 |
| S6 | `report run testdata/p_filter.json` (`where amount > 100`) | only rows with amount > 100 | 0 |
| S7 | `report run testdata/p_derive.json` (`net = amount * 0.9`) | new `net` column, float, = amount*0.9 | 0 |
| S8 | `report run testdata/p_sort_multi.json` (by region asc, amount desc) | stable multi-key order | 0 |
| S9 | `report run testdata/p_agg.json` (group by region, sum + count) | one row per region, sorted by region | 0 |
| S10 | `report run testdata/p_json_out.json` | a JSON array of objects; types preserved (numbers not quoted) | 0 |
| S11 | `report run testdata/p_markdown.json` | a GitHub markdown table | 0 |
| S12 | `report run testdata/p_template.json` | rendered via the supplied `text/template` | 0 |
| S13 | `report run testdata/p_range.json -gzip -o out.csv.gz` | `out.csv.gz` decompresses to the expected CSV | 0 |
| S14 | `report explain testdata/p_topregions.json` | `csv → filter → aggregate → sort → limit → table` | 0 |
| S15 | `report schema testdata/p_agg.json` | `region string, total float, n int` | 0 |

### Case specification — FAILURE

| # | Command | stderr | exit |
|---|---|---|---|
| F1 | `report run testdata/p_bad_source.json` (`"type": "sqlite"`) | `error: unknown source "sqlite"` | 1 |
| F2 | `report run testdata/p_bad_transform.json` (`"type": "pivot"`) | `error: unknown transform "pivot"` | 1 |
| F3 | `report run testdata/p_bad_renderer.json` | `error: unknown renderer "pdf"` | 1 |
| F4 | `report run testdata/p_select_missing.json` (`select ["nope"]`) | `error: select: no column "nope"` | 1 |
| F5 | `report run testdata/p_filter_badexpr.json` (`where "amount >"`) | `error: filter: invalid expression: unexpected end of input at position 9` | 1 |
| F6 | `report run testdata/p_filter_badcol.json` (`where "missing > 1"`) | `error: filter: row 1: unknown identifier "missing"` | 1 |
| F7 | `report run testdata/p_agg_badfn.json` (`"fn": "median"`) | `error: aggregate: unknown function "median"` | 1 |
| F8 | `report run testdata/p_csv_missing.json` (path doesn't exist) | `error: csv source: open testdata/nope.csv: no such file or directory` | 1 |
| F9 | `report run testdata/p_not_json.json` | `error: testdata/p_not_json.json: invalid pipeline: <detail>` | 1 |
| F10 | `report` | usage | 2 |
| F11 | `report frobnicate` | `error: unknown command "frobnicate"` + usage | 2 |
| F12 | `report run` | `error: expected 1 argument (pipeline), got 0` | 2 |

### Error catalogue

| Trigger | Format |
|---|---|
| unknown plugin | `error: unknown %s %q` (kind, name) |
| plugin config | `error: %s: invalid config: %v` |
| source read | `error: %s source: %v` |
| transform | `error: %s: %s` (name, detail; row-scoped errors include `row %d`) |
| render | `error: render: %v` |
| bad pipeline file | `error: %s: invalid pipeline: %v` |
| registry duplicate | **panic** `report: %s %q already registered` |

## Suggested milestones

1. Data model (`Type`, `Column`, `Table`), `Type.String()`, the `Cell` type-switch
   stringifier + tests over every dynamic type.
2. Registry (`RegisterX` + `init()` in each plugin file); `report sources/transforms/renderers`.
3. Sources: `inline`, `range`, `csv` (with type inference), `json`.
4. Transforms: `select`, `rename`, `limit`, `sort` (`sort.Stable`).
5. `filter` / `derive` wired to `internal/calc` — build an `Env` per row from the record.
6. `aggregate`.
7. Renderers: `table`, `csv`, `json`, `markdown`, `template`.
8. `Money` + `fmt.Formatter`; `-money` option; `-gzip` sink.
9. `run` / `explain` / `schema`. Golden CLI tests + fixtures.

## Project layout

```
projects/13-report/
  cmd/report/main.go
  internal/report/model.go        // Type, Column, Cell, Table, cell stringify
  internal/report/registry.go
  internal/report/pipeline.go     // parse pipeline.json, build, run
  internal/report/source_*.go     // csv, json, range, inline (each with init())
  internal/report/transform_*.go  // select, rename, filter, derive, sort, limit, aggregate
  internal/report/render_*.go     // table, csv, json, markdown, template
  internal/report/money.go        // fmt.Formatter
  internal/report/*_test.go
  cmd/report/main_test.go
  testdata/
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly.
- [ ] Adding a new renderer requires **only** a new `render_x.go` with an `init()` — no
      edits to `pipeline.go` or `main.go` (prove it: add a trivial `null` renderer in a
      test file and it shows up in `report renderers`).
- [ ] The cell type-switch handles every `Cell` variant incl. `nil` and `fmt.Stringer`,
      with a test per branch.
- [ ] `Money` passes a `fmt` verb table test.
- [ ] `sort` with two keys is stable (equal-key rows keep source order — tested).
- [ ] Duplicate registration panics with the exact string.

## Test requirements

- `TestCellString` — one row per dynamic type + `nil` + a custom `Stringer`.
- `TestRegistry` — all built-ins present; duplicate → panic; a test-only plugin appears.
- `TestSources` — csv inference (int/float/bool/time/string/empty), json key union,
  range determinism.
- `TestTransforms` — one per transform incl. error rows (F4–F7).
- `TestFilterExpr` / `TestDeriveExpr` — truthiness rules, string comparison, `&&`/`||`,
  bad expr, unknown column.
- `TestAggregate` — sum/avg/count/min/max, multi-key group, group ordering.
- `TestRenderers` — golden output per renderer for one small table.
- `TestMoneyFormatter` — the verb table.
- `TestGzipSink`.
- `TestPipelineEndToEnd` — S4/S9 full runs.
- `ExampleMoney`.

## Stretch goals

- A `sql`-ish source that reads from Project 12's `Config` + a fake in-memory dataset.
- `join` transform (inner/left) on a key.
- A `--watch` mode that re-runs the pipeline when the source file changes.
- Streaming mode: `Source.Rows() iter.Seq2[Record, error]` so a huge CSV never fully
  materializes (bridge to Project 11's iterators + Project 09's streaming).
- A `report serve PIPELINE.json` that exposes the result at `/report.json` (after
  Project 20).

## Self-check questions

1. `Source`, `Transform`, `Renderer` are three separate interfaces. What does composing
   them into `type Pipeline interface { Source; Renderer }` buy you, and why *didn't* you
   make one big interface with all the methods?
2. A `Cell` is `any`. In the type switch, `case fmt.Stringer:` must come **after**
   `case string:` — or does it? What does `"hello".(fmt.Stringer)` do?
3. `nil` in a type switch: `case nil:` matches an untyped nil interface. What about a
   `(*Money)(nil)` stored in the `Cell`? Which case catches it and is that what you want?
4. Plugins register in `init()`. What guarantees every plugin file's `init()` runs before
   `main`, given they're in the same package? What if they were in *different* packages?
5. `sort.Stable` vs `sort.Sort` for `by: ["region", "amount"]` — you could also do one
   sort with a combined comparator. When is "sort by amount, then stable-sort by region"
   equivalent, and when is it not?
6. `fmt.Formatter.Format(f fmt.State, verb rune)` — how do you read the requested width
   and the `#` flag out of `f`, and how do you actually write the padded output?
7. Your `filter` builds a `calc.Env` per row. For a 1M-row table that's 1M env
   allocations. How would you reuse one env, and what breaks if a `derive` expression has
   a side effect (it can't here — but why not)?
