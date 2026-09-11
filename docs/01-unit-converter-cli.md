# Project 01 — Unit Converter CLI

| | |
|---|---|
| **Difficulty** | 1 / 5 |
| **Estimated time** | 2–4 hours |
| **Prerequisites** | none — this is the starting point |
| **Builds toward** | [03 – Date & Duration Calculator](03-date-duration-calculator.md), [07 – Todo CLI](07-todo-cli.md) |

## Why this project

You will build a small but complete command-line program end to end: parse arguments,
validate them, do the work, format output, and return correct exit codes. It exists to
make you use Go's constant system, `switch`, functions with multiple return values, and
the `flag` / `strconv` packages — the pieces every later project assumes you already own.

## Go concepts you MUST use

- [ ] `type` definitions: a named integer type (`Dimension`) and a struct (`Unit`)
- [ ] typed and untyped constants; a `const` block; `iota`; `iota` in an expression
      (`1 << (10 * iota)` for the binary data-size units)
- [ ] the blank identifier `_` at least once meaningfully (e.g. skipping `iota` 0)
- [ ] an **expression** `switch` and a **tagless** `switch` (`switch { case … }`)
- [ ] a function with multiple return values, and one function with **named** returns
      used with a naked `return`
- [ ] zero values (rely on them; explain them in the self-check)
- [ ] `init()` to populate the unit registry
- [ ] `flag` — define flags, a custom `Usage`, `flag.Parse`, `flag.Args`
- [ ] `strconv.ParseFloat` and `strconv.FormatFloat`
- [ ] `os.Exit` with distinct codes; write errors to `os.Stderr`

## Background

Converting within one dimension (length, mass, digital storage) is "multiply by a ratio":
pick a base unit, store every unit's size in base units, then
`target = value * from.factor / to.factor`.

Temperature is different — Celsius, Fahrenheit and Kelvin have different **zero points**,
so they need explicit formulas, not one ratio.

Digital storage has two families: decimal (`kB = 1000 B`) and binary (`KiB = 1024 B`).
They must stay distinct.

Absolute zero is `0 K = -273.15 °C = -459.67 °F`. A temperature below it is physically
meaningless and your program must reject it.

## Requirements

### Functional requirements

1. Invoked as `convert [flags] <value> <from> <to>`; prints the converted number followed
   by a newline to **stdout**.
2. Supported dimensions and units (tokens matched **case-insensitively**):
   - **Length** (base: metre) — `mm cm m km in ft yd mi nmi`
   - **Mass** (base: gram) — `mg g kg t oz lb st`
   - **Temperature** — `C F K`
   - **Data** (base: byte) — decimal `B kB MB GB TB`, binary `KiB MiB GiB TiB`
3. `<value>` is parsed with `strconv.ParseFloat`. Scientific notation (`1.5e3`), a leading
   `+`, and negatives are accepted at parse time. `NaN` and `Inf` are rejected.
4. Conversion is defined only **within one dimension**. Cross-dimension is an error that
   names both dimensions.
5. Length, mass and data values must be `>= 0`; a negative value there is a domain error.
   Temperature may be negative but must be `>=` absolute zero for its unit.
6. `-precision N` (default `4`, range `0..15`) → `strconv.FormatFloat(x, 'f', N, 64)`
   (round-half-to-even — state this in your README).
7. `-exact` prints the unrounded result (`strconv.FormatFloat(x, 'g', -1, 64)`) and
   overrides `-precision`.
8. `-list` prints every unit grouped by dimension in the exact format below, exits 0.
   `-version` prints `convert <Version>` and exits 0.
9. Converting a unit to itself returns the input value — still routed through the normal
   path, no special-case shortcut.

### Exact contract

Invocation: `convert [flags] <value> <from> <to>`. **Flags must come before the positional
args** — Go's `flag` package stops at the first non-flag token (a deliberate teaching
point; see failure table F30 and self-check 7).

| Flag | Type | Default | Valid range | Effect |
|------|------|---------|-------------|--------|
| `-precision` | int | `4` | `0`–`15` | digits after the decimal point; `strconv.FormatFloat(x, 'f', p, 64)` |
| `-exact` | bool | `false` | — | print `strconv.FormatFloat(x, 'g', -1, 64)`; overrides `-precision` |
| `-list` | bool | `false` | — | print the unit list, exit 0 |
| `-version` | bool | `false` | — | print `convert <Version>`, exit 0 (`Version` is a package const, e.g. `0.1.0`) |

Positional args (required unless `-list` or `-version`): `value`, `from`, `to`.

Units — canonical token → base-unit factor:

| Dimension | Base | Units (factor) |
|-----------|------|----------------|
| Length | `m` | `mm` 0.001, `cm` 0.01, `m` 1, `km` 1000, `in` 0.0254, `ft` 0.3048, `yd` 0.9144, `mi` 1609.344, `nmi` 1852 |
| Mass | `g` | `mg` 0.001, `g` 1, `kg` 1000, `t` 1e6, `oz` 28.349523125, `lb` 453.59237, `st` 6350.29318 |
| Temperature | — | `C`, `F`, `K` (formula-based) |
| Data | `B` | `B` 1, `kB` 1e3, `MB` 1e6, `GB` 1e9, `TB` 1e12, `KiB` 1024, `MiB` 1048576, `GiB` 1073741824, `TiB` 1099511627776 |

Temperature formulas: `F = C·9/5 + 32`; `C = (F−32)·5/9`; `K = C + 273.15`.
Absolute zero: `0 K = −273.15 °C = −459.67 °F`.

`-list` output (exactly this; units in ascending-factor order, temperature `C F K`, data
decimal then binary):

```
Length (base: m)
  mm cm m km in ft yd mi nmi
Mass (base: g)
  mg g kg t oz lb st
Temperature
  C F K
Data (base: B)
  B kB MB GB TB KiB MiB GiB TiB
```

### Exit codes

| Code | Meaning |
|------|---------|
| `0`  | success, including `-list` and `-version` |
| `1`  | semantic error — bad value, unknown unit, cross-dimension, negative where disallowed, below absolute zero, `-precision` out of range |
| `2`  | usage error — wrong positional-arg count, or a flag error raised by the `flag` package itself (`-h`, unknown flag, non-int `-precision`) |

All *your* error messages go to **stderr** as one line beginning `error: `. The `flag`
package's own messages are formatted by it and followed by the usage text.

### Resolution / precedence order (checks run in exactly this order)

1. `flag.Parse()` — on any flag error the `flag` package prints its message + usage to
   stderr and exits `2`.
2. `-version` set → print `convert <Version>`, exit `0`.
3. `-list` set → print the unit list, exit `0`. *(short-circuits before every check below,
   so `convert -list -precision 99 1 m km` still just lists and exits 0)*
4. `-precision` in `0..15`? else `error: -precision must be between 0 and 15, got N`, exit `1`.
5. exactly 3 positional args? else `error: expected 3 arguments (value from to), got N`, exit `2`.
6. `value` parses as a finite float? else exit `1` (see error catalogue).
7. `from` resolves to a known unit? else `error: unknown unit "<from>"`, exit `1`.
8. `to` resolves to a known unit? else `error: unknown unit "<to>"`, exit `1`.
9. same dimension? else `error: cannot convert: "<from>" is <dimA> but "<to>" is <dimB>`, exit `1`.
10. domain check — length/mass/data: value `>= 0`, else
    `error: <dim> values cannot be negative: <rawValue>`, exit `1`. temperature: input
    `>=` absolute zero for its unit, else
    `error: <rawValue> <°C|°F|K> is below absolute zero (0 K)`, exit `1`.
    *(A validated input always yields a valid output — only the input is checked; prove
    this in self-check 8.)*
11. convert, format per `-exact`/`-precision`, print to stdout + newline, exit `0`.

### Case specification — SUCCESS (every one must pass exactly)

| # | Command | stdout | exit |
|---|---------|--------|------|
| S1 | `convert 100 km mi` | `62.1371` | 0 |
| S2 | `convert 1 mi km` | `1.6093` | 0 |
| S3 | `convert 1 mi ft` | `5280.0000` | 0 |
| S4 | `convert 1 ft in` | `12.0000` | 0 |
| S5 | `convert 1 nmi km` | `1.8520` | 0 |
| S6 | `convert 2.5 m cm` | `250.0000` | 0 |
| S7 | `convert 1 yd m` | `0.9144` | 0 |
| S8 | `convert 0 m km` | `0.0000` | 0 |
| S9 | `convert 1e6 mm km` | `1.0000` | 0 |
| S10 | `convert 10 kg lb` | `22.0462` | 0 |
| S11 | `convert 1 lb oz` | `16.0000` | 0 |
| S12 | `convert 1 st kg` | `6.3503` | 0 |
| S13 | `convert 1 t kg` | `1000.0000` | 0 |
| S14 | `convert 500 g kg` | `0.5000` | 0 |
| S15 | `convert 0 C F` | `32.0000` | 0 |
| S16 | `convert 100 C F` | `212.0000` | 0 |
| S17 | `convert -40 C F` | `-40.0000` | 0 |
| S18 | `convert -100 C F` | `-148.0000` | 0 |
| S19 | `convert 98.6 F C` | `37.0000` | 0 |
| S20 | `convert 0 C K` | `273.1500` | 0 |
| S21 | `convert 300 K C` | `26.8500` | 0 |
| S22 | `convert 32 F K` | `273.1500` | 0 |
| S23 | `convert 273.15 K F` | `32.0000` | 0 |
| S24 | `convert 25 C C` | `25.0000` | 0 |
| S25 | `convert -273.15 C K` | `0.0000` | 0 |
| S26 | `convert -459.67 F K` | `0.0000` | 0 |
| S27 | `convert 1 GiB MB` | `1073.7418` | 0 |
| S28 | `convert 1 GiB MiB` | `1024.0000` | 0 |
| S29 | `convert 1 MB KiB` | `976.5625` | 0 |
| S30 | `convert 1 TiB GB` | `1099.5116` | 0 |
| S31 | `convert 1000 kB MB` | `1.0000` | 0 |
| S32 | `convert 1024 B KiB` | `1.0000` | 0 |
| S33 | `convert -precision 0 100 km mi` | `62` | 0 |
| S34 | `convert -precision 2 100 km mi` | `62.14` | 0 |
| S35 | `convert -precision 15 1 mi km` | `1.609344000000000` | 0 |
| S36 | `convert -exact 1 mi km` | `1.609344` | 0 |
| S37 | `convert +100 km mi` | `62.1371` | 0 |
| S38 | `convert 1.5e3 m km` | `1.5000` | 0 |
| S39 | `convert 100 KM MI` | `62.1371` | 0 |
| S40 | `convert 1 gib mib` | `1024.0000` | 0 |
| S41 | `convert -list` | the unit-list block above | 0 |
| S42 | `convert -list 100 km mi` | the unit-list block above | 0 |
| S43 | `convert -list -precision 99 1 m km` | the unit-list block above | 0 |
| S44 | `convert -version` | `convert 0.1.0` | 0 |
| S45 | `convert -version -list` | `convert 0.1.0` | 0 |

### Case specification — FAILURE (every one must produce exactly this)

| # | Command | stderr | exit |
|---|---------|--------|------|
| F1 | `convert 5 m kg` | `error: cannot convert: "m" is length but "kg" is mass` | 1 |
| F2 | `convert 5 kg m` | `error: cannot convert: "kg" is mass but "m" is length` | 1 |
| F3 | `convert 5 C m` | `error: cannot convert: "C" is temperature but "m" is length` | 1 |
| F4 | `convert 5 GB kg` | `error: cannot convert: "GB" is data but "kg" is mass` | 1 |
| F5 | `convert 5 m parsec` | `error: unknown unit "parsec"` | 1 |
| F6 | `convert 5 furlong m` | `error: unknown unit "furlong"` | 1 |
| F7 | `convert 5 m furlong` | `error: unknown unit "furlong"` | 1 |
| F8 | `convert 5 xyz abc` | `error: unknown unit "xyz"` | 1 |
| F9 | `convert -5 m km` | `error: length values cannot be negative: -5` | 1 |
| F10 | `convert -5 kg lb` | `error: mass values cannot be negative: -5` | 1 |
| F11 | `convert -5 MB KiB` | `error: data values cannot be negative: -5` | 1 |
| F12 | `convert -300 C K` | `error: -300 °C is below absolute zero (0 K)` | 1 |
| F13 | `convert -500 F C` | `error: -500 °F is below absolute zero (0 K)` | 1 |
| F14 | `convert -1 K C` | `error: -1 K is below absolute zero (0 K)` | 1 |
| F15 | `convert abc m km` | `error: invalid value "abc": not a number` | 1 |
| F16 | `convert '' m km` | `error: invalid value "": not a number` | 1 |
| F17 | `convert NaN m km` | `error: invalid value "NaN": not a finite number` | 1 |
| F18 | `convert Inf m km` | `error: invalid value "Inf": not a finite number` | 1 |
| F19 | `convert +Inf m km` | `error: invalid value "+Inf": not a finite number` | 1 |
| F20 | `convert -Inf C K` | `error: invalid value "-Inf": not a finite number` | 1 |
| F21 | `convert -precision -1 100 km mi` | `error: -precision must be between 0 and 15, got -1` | 1 |
| F22 | `convert -precision 16 100 km mi` | `error: -precision must be between 0 and 15, got 16` | 1 |
| F23 | `convert -precision abc 100 km mi` | flag pkg: `invalid value "abc" for flag -precision: …` + usage | 2 |
| F24 | `convert -foo 100 km mi` | flag pkg: `flag provided but not defined: -foo` + usage | 2 |
| F25 | `convert -h` | flag pkg: usage text | 2 |
| F26 | `convert` | `error: expected 3 arguments (value from to), got 0` | 2 |
| F27 | `convert 100` | `error: expected 3 arguments (value from to), got 1` | 2 |
| F28 | `convert 100 km` | `error: expected 3 arguments (value from to), got 2` | 2 |
| F29 | `convert 100 km mi cm` | `error: expected 3 arguments (value from to), got 4` | 2 |
| F30 | `convert 100 km mi -precision 2` | `error: expected 3 arguments (value from to), got 5` — flags after positionals are treated as positionals | 2 |

### Error catalogue (use these formats verbatim)

| Trigger | `fmt` format (args) |
|---------|---------------------|
| cross-dimension | `error: cannot convert: %q is %s but %q is %s` (fromTok, fromDim, toTok, toDim) |
| unknown unit | `error: unknown unit %q` (token — report `from` first, then `to`) |
| negative disallowed | `error: %s values cannot be negative: %s` (dimName, rawValueArg) |
| below absolute zero | `error: %s %s is below absolute zero (0 K)` (rawValueArg, `°C`/`°F`/`K`) |
| non-numeric value | `error: invalid value %q: not a number` (rawValueArg) |
| non-finite value | `error: invalid value %q: not a finite number` (rawValueArg) |
| precision range | `error: -precision must be between 0 and 15, got %d` (p) |
| arg count | `error: expected 3 arguments (value from to), got %d` (n) |

## Suggested milestones

1. **Registry + `-list`.** Define `Dimension`, `Unit`, the `init()`-built registry; make
   `-list` / `-version` work. No conversion yet.
2. **Arg & flag handling.** Parse flags, validate positional-arg count, wire exit codes 2
   (usage) vs 1 (semantic). Parse `<value>` with `strconv.ParseFloat` + finiteness check.
3. **Ratio conversion.** `toBase` / `fromBase` for length, mass, data. Same-unit,
   cross-dimension.
4. **Temperature.** Three formulas behind an expression `switch`; absolute-zero checks
   with a tagless `switch`.
5. **Output formatting.** `-precision` and `-exact`.
6. **Tests + README.** Golden CLI test (one row per case) + unit tests; README documents
   the rounding mode and exit codes.

## Project layout

```
projects/01-unit-converter/
  cmd/convert/main.go          // flags, arg validation, output, exit codes — no conversion math
  internal/units/units.go      // Dimension, Unit, registry, init(), Lookup()
  internal/units/convert.go    // toBase/fromBase, temperature formulas, Convert()
  internal/units/units_test.go
  internal/units/convert_test.go
  cmd/convert/main_test.go      // the golden CLI table
  README.md
  Makefile
```

`cmd/convert/main.go` owns all I/O and process exit; `internal/units` is pure — it never
calls `os.Exit` and never prints. Give `main` a testable seam:
`func run(args []string, stdout, stderr io.Writer) int`.

## Definition of Done

Extends the universal checklist in [README.md](README.md). Additionally:

- [ ] Every row of both Case-specification tables (S1–S45, F1–F30) reproduces exactly —
      same stdout/stderr text, same exit code.
- [ ] `internal/units` has no `os.Exit`, no printing, no package-level mutable state
      except the registry built in `init()`.
- [ ] `-list` output is byte-for-byte identical across runs.
- [ ] `go test ./...` passes; `go vet ./...` and `gofmt -l .` are clean.
- [ ] Every exported identifier in `internal/units` has a doc comment.
- [ ] `README.md` states the rounding mode and the exit-code table.

## Test requirements

**A. Golden CLI test** (`cmd/convert/main_test.go`, table-driven): call
`run([]string, stdout, stderr)` for every S1–S45 and F1–F30 row; assert stdout, stderr
and the returned code. This table *is* the spec — keep them in sync.

**B. Unit tests** (`internal/units`, table-driven):

- `TestParseValue` — `"100"`, `"-273.15"`, `"1.5e3"`, `"+4"` (ok); `"abc"`, `""` (not a
  number); `"NaN"`, `"Inf"`, `"+Inf"`, `"-Inf"` (not finite).
- `TestConvertRatio` — one row per (S2–S14, S27–S32); assert within relative `1e-9`.
- `TestConvertTemperature` — S15–S26; assert within `1e-9`.
- `TestDomainErrors` — F9–F14.
- `TestCrossDimension` — F1–F4; message contains both dimension names.
- `TestUnknownUnit` — F5–F8; `from` reported before `to`.
- `TestCaseInsensitive` — `KM`/`Km`/`km` resolve equal; `kib` == `KiB`.
- `TestRoundTrip` — every registered unit, value `123.456`: to base and back, within
  relative `1e-9`.
- `TestFormat` — precision `0`, `2`, `4`, `15`, and `-exact`.
- `TestListDeterministic` — render `-list` twice, assert identical.
- `ExampleConvert` — a testable example that appears in `go doc`.

*(No benchmarks/fuzzing required here — they start at Project 11.)*

## Stretch goals

- Add **area** and **volume**, parsing `km2` / `m3` tokens.
- `-verbose`: print `100 km = 62.1371 mi`.
- No positional args → read `value from to` lines from stdin in a loop (a mini REPL).
- Unit **aliases**: `meter`, `metre`, `meters` → `m`.
- `-table FROM`: print `FROM` converted to every other unit in its dimension.

## Self-check questions

1. On `KiB = 1 << (10 * iota)` inside your data-unit `const` block, what is `iota`, and
   why is the first entry `_`?
2. `strconv.ParseFloat("10", 64)` returns `(float64, error)`; a map lookup with comma-ok
   returns `(V, bool)`. When do you use which, and why can't `ParseFloat` return a bool?
3. If you skip the `ok` from your registry lookup, what `Unit` value does conversion run
   with, and why a wrong number rather than a panic?
4. Could this registry have been a package-level composite literal instead of `init()`?
   When is `init()` *actually* required?
5. In `func toBase(v float64, u Unit) (base float64, err error)`, what is `base` before
   your first assignment, and what does a naked `return` in the error branch send back?
6. Temperature can't convert through one multiplicative factor. What structural change
   did that force in `Convert`, and which `switch` form did you use to branch on
   dimension?
7. Case F30: `convert 100 km mi -precision 2` fails with "got 5". Why does Go's `flag`
   package not see `-precision` here, and what argument order does it require?
8. F12–F14 check the *input* against absolute zero; there is no *output* check. Prove
   that a temperature input `>=` absolute zero can never convert to one below it.
