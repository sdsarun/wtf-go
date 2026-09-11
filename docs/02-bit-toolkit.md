# Project 02 — Number Bases & Bit Toolkit

| | |
|---|---|
| **Difficulty** | 1 / 5 |
| **Estimated time** | 3–5 hours |
| **Prerequisites** | [01 – Unit Converter CLI](01-unit-converter-cli.md) |
| **Builds toward** | [10 – Binary File Format Parser](10-binary-file-parser.md) |

## Why this project

Integers, bits, and number bases are the substrate everything else sits on. This project
forces you to think in binary and two's complement, to use every bitwise operator, and to
drive `fmt` and `strconv` hard. It also introduces **subcommands** (`flag.NewFlagSet`),
which several later CLIs need.

## Go concepts you MUST use

- [ ] every bitwise operator: `&`, `|`, `^` (both binary XOR and unary NOT), `&^`
      (AND NOT / bit clear), `<<`, `>>`
- [ ] bit-flag constants with `1 << iota`
- [ ] fixed-width integer types (`uint8/16/32/64`, `int8/16/32/64`) and conversions
      between them; deliberate **overflow / wraparound**
- [ ] signed vs unsigned: arithmetic, right shift (arithmetic vs logical), formatting
- [ ] `math/bits` — `OnesCount64`, `LeadingZeros64`, `TrailingZeros64`, `Len64`,
      `Reverse64`, `RotateLeft64`
- [ ] `strconv.ParseInt`, `ParseUint`, `FormatInt`, `FormatUint` with an explicit base;
      base-0 auto-detection
- [ ] `fmt` verbs: `%b %o %x %X %c %q %d %v`, zero-padding & width (`%08b`, `%#x`)
- [ ] rune ↔ integer conversions; `%c` and `%q` for runes
- [ ] subcommands via `flag.NewFlagSet`
- [ ] a `Stringer` on a bit-flag type

## Background

**Two's complement:** a width-`W` signed integer stores `x` as the low `W` bits of
`x mod 2^W`; the top bit is the sign. `int8(-1)` and `uint8(255)` have identical bits
(`1111_1111`). Converting between same-width signed/unsigned types in Go **keeps the
bits** and reinterprets them.

**Arithmetic vs logical right shift:** `>>` on a signed type copies the sign bit in
(arithmetic); on an unsigned type it shifts in zeros (logical). Go picks based on the
operand's type.

**`&^` (bit clear):** `a &^ b` clears in `a` every bit that is set in `b`. Equivalent to
`a & ^b`. Useful for turning flags off.

**Unix permission bits:** a mode like `0755` packs three groups of `rwx` — owner, group,
other — three bits each, most significant group first. `0755` = `111_101_101` =
`rwxr-xr-x`.

## Requirements

### Functional requirements

1. `bits <subcommand> …`. Subcommands: `conv`, `show`, `calc`, `not`, `perms`, `char`,
   `ord`. Unknown or missing subcommand → usage on stderr, exit 2.
2. **`bits conv <number> [-from B] [-to B]`** — convert an integer string between bases.
   `-from` / `-to` default `10` / `2`; valid range `2..36`, plus `-from 0` = auto-detect
   from a `0x` / `0o` / `0b` prefix. A leading `-` means negative. Output is the number in
   base `-to`, lowercase, no prefix. Digits above 9 are `a`–`z`.
3. **`bits show <number> [-width 8|16|32|64] [-signed]`** — parse `<number>` (base-0
   auto-detect; default decimal), then print the report block defined below. `-width`
   defaults to `64`. With `-signed`, the value is interpreted as a two's-complement signed
   integer of that width.
4. **`bits calc <a> <op> <b> [-width W] [-signed]`** — `op` ∈
   `+ - * / % & | ^ &^ shl shr`. Compute at the given width with wraparound. Print the
   result block. Division or modulo by zero → error, exit 1. `shl` / `shr` by `>= W` →
   result `0` for unsigned / `shl`, and `0` or `-1` for signed `shr` (sign-propagating);
   this is defined behaviour here, not an error.
5. **`bits not <number> [-width W]`** — bitwise complement (`^x`) at width `W`
   (default 64), printed in the result block. Always treated as unsigned for display.
6. **`bits perms <mode>`** — `<mode>` is 3 or 4 octal digits (`755`, `0644`, `0o600`).
   Print the symbolic string (`rwxr-xr-x`) and a per-group breakdown.
   **`bits perms -encode <symbolic>`** — inverse: `rwxr-xr-x` → `0755`.
7. **`bits char <codepoint>`** — `<codepoint>` as decimal, `0x2603`, or `U+2603`. Print
   the character, its `%q` form, and `U+XXXX`. Error if not a valid Unicode code point.
8. **`bits ord <char>`** — exactly one character (one rune). Print its code point as
   decimal, hex, and `U+XXXX`. Error if the argument is not exactly one rune.

### Exact contract — output blocks

**`bits show` report** (example: `bits show 202 -width 16`):

```
value          202
hex            0xca
octal          0o312
binary         11001010
binary (width) 0000000011001010
bit length     8
set bits       4
leading zeros  8
trailing zeros 1
power of two   false
```

With `-signed` and a value whose top bit is set at that width, add a line
`signed         -54` (the two's-complement interpretation) directly under `value`.
`binary (width)` is grouped in 4-bit nibbles with `_` separators once width ≥ 8:
`0000_0000_1100_1010`.

**`bits calc` / `bits not` result block** (example: `bits calc 255 + 1 -width 8`):

```
result   0
hex      0x0
binary   00000000
overflow true
```

`overflow` is `true` when the mathematically exact result does not fit the width (for
`+ - *`), else `false`. For bit ops and shifts, `overflow` is always `false`.

**`bits perms 755` output:**

```
symbolic  rwxr-xr-x
owner     rwx (7)
group     r-x (5)
other     r-x (5)
```

**`bits char 0x2603` output:**

```
char   ☃
quoted '☃'
code   U+2603
```

**`bits ord ☃` output:**

```
decimal 9731
hex     0x2603
code    U+2603
```

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | success |
| `1` | semantic error — unparseable number, base out of range, digit invalid for base, `/ 0` or `% 0`, bad code point, bad permission string |
| `2` | usage error — unknown/missing subcommand, unknown flag, missing positional args (`flag` package or your own count check) |

Your errors: one `error: ` line to stderr.

### Resolution / precedence order

1. No args or unknown subcommand → print top-level usage, exit 2.
2. Dispatch to the subcommand's `*flag.FlagSet`; parse it. Flag errors → exit 2.
3. Validate positional-arg count for the subcommand → wrong count → exit 2.
4. Validate flag values (`-width` ∈ {8,16,32,64}; bases ∈ `2..36` or `0`) → exit 1.
5. Parse the number(s) with the right `strconv` call → parse failure → exit 1.
6. Do the work; print the block; exit 0.

### Case specification — SUCCESS

| # | Command | stdout (first line shown; blocks abbreviated) | exit |
|---|---------|--------|------|
| S1 | `bits conv 255` | `11111111` | 0 |
| S2 | `bits conv 255 -to 16` | `ff` | 0 |
| S3 | `bits conv ff -from 16 -to 10` | `255` | 0 |
| S4 | `bits conv 11111111 -from 2 -to 16` | `ff` | 0 |
| S5 | `bits conv -255 -to 16` | `-ff` | 0 |
| S6 | `bits conv 0x1F -from 0 -to 10` | `31` | 0 |
| S7 | `bits conv 0b1010 -from 0 -to 10` | `10` | 0 |
| S8 | `bits conv 35 -to 36` | `z` | 0 |
| S9 | `bits conv 0 -to 2` | `0` | 0 |
| S10 | `bits show 8` | block; `power of two   true`, `set bits       1` | 0 |
| S11 | `bits show 202 -width 16` | the example block above | 0 |
| S12 | `bits show 202 -width 8 -signed` | block with `signed         -54` | 0 |
| S13 | `bits show 0` | block; `bit length     0`, `power of two   false` | 0 |
| S14 | `bits show 0xFF` | block; `set bits       8`, `hex            0xff` | 0 |
| S15 | `bits calc 12 & 10` | `result   8` | 0 |
| S16 | `bits calc 12 \| 10` | `result   14` | 0 |
| S17 | `bits calc 12 ^ 10` | `result   6` | 0 |
| S18 | `bits calc 12 &^ 10` | `result   4` | 0 |
| S19 | `bits calc 1 shl 4` | `result   16` | 0 |
| S20 | `bits calc 255 + 1 -width 8` | `result   0`, `overflow true` | 0 |
| S21 | `bits calc 100 + 27 -width 8` | `result   127`, `overflow false` | 0 |
| S22 | `bits calc 100 + 28 -width 8 -signed` | `result   -128`, `overflow true` | 0 |
| S23 | `bits calc -1 shr 1 -width 8 -signed` | `result   -1` (sign propagates) | 0 |
| S24 | `bits calc 255 shr 1 -width 8` | `result   127` | 0 |
| S25 | `bits not 0 -width 8` | `result   255` | 0 |
| S26 | `bits not 0xFF -width 8` | `result   0` | 0 |
| S27 | `bits perms 755` | the example block above | 0 |
| S28 | `bits perms 0644` | `symbolic  rw-r--r--` | 0 |
| S29 | `bits perms -encode rwxr-xr-x` | `0755` | 0 |
| S30 | `bits perms -encode rw-------` | `0600` | 0 |
| S31 | `bits char 65` | `char   A` | 0 |
| S32 | `bits char U+2603` | `char   ☃` | 0 |
| S33 | `bits char 0x1F600` | `char   😀` | 0 |
| S34 | `bits ord A` | `decimal 65` | 0 |
| S35 | `bits ord ☃` | the example block above | 0 |

### Case specification — FAILURE

| # | Command | stderr | exit |
|---|---------|--------|------|
| F1 | `bits` | top-level usage | 2 |
| F2 | `bits wat` | `error: unknown subcommand "wat"` + usage | 2 |
| F3 | `bits conv` | `error: expected 1 argument (number), got 0` | 2 |
| F4 | `bits conv 10 -from 1` | `error: base must be 0 or between 2 and 36, got 1` | 1 |
| F5 | `bits conv 10 -to 37` | `error: base must be 0 or between 2 and 36, got 37` | 1 |
| F6 | `bits conv 9 -from 2` | `error: "9" is not a valid base-2 integer` | 1 |
| F7 | `bits conv xyz -from 16` | `error: "xyz" is not a valid base-16 integer` | 1 |
| F8 | `bits show abc` | `error: "abc" is not a valid integer` | 1 |
| F9 | `bits show 10 -width 7` | `error: -width must be 8, 16, 32, or 64, got 7` | 1 |
| F10 | `bits calc 10 / 0` | `error: division by zero` | 1 |
| F11 | `bits calc 10 % 0` | `error: division by zero` | 1 |
| F12 | `bits calc 10 wat 3` | `error: unknown operator "wat"` | 1 |
| F13 | `bits calc 10 +` | `error: expected 3 arguments (a op b), got 2` | 2 |
| F14 | `bits perms 999` | `error: "999" is not a valid octal mode` | 1 |
| F15 | `bits perms 12345` | `error: mode must be 3 or 4 octal digits` | 1 |
| F16 | `bits perms -encode rwxrwx` | `error: symbolic mode must be exactly 9 characters` | 1 |
| F17 | `bits perms -encode rwxr-xr-Q` | `error: invalid symbolic mode character 'Q' at position 8` | 1 |
| F18 | `bits char 0x110000` | `error: 0x110000 is not a valid Unicode code point` | 1 |
| F19 | `bits char -1` | `error: -1 is not a valid Unicode code point` | 1 |
| F20 | `bits ord AB` | `error: expected exactly one character, got 2` | 1 |
| F21 | `bits ord ""` | `error: expected exactly one character, got 0` | 1 |

### Error catalogue

| Trigger | Format |
|---|---|
| unknown subcommand | `error: unknown subcommand %q` |
| arg count | `error: expected N argument(s) (<names>), got M` |
| base range | `error: base must be 0 or between 2 and 36, got %d` |
| invalid digits | `error: %q is not a valid base-%d integer` |
| invalid integer | `error: %q is not a valid integer` |
| width | `error: -width must be 8, 16, 32, or 64, got %d` |
| divide by zero | `error: division by zero` |
| unknown operator | `error: unknown operator %q` |
| bad octal mode | `error: %q is not a valid octal mode` |
| mode length | `error: mode must be 3 or 4 octal digits` |
| symbolic length | `error: symbolic mode must be exactly 9 characters` |
| symbolic char | `error: invalid symbolic mode character %q at position %d` |
| code point | `error: %#x is not a valid Unicode code point` / `error: %d is not a valid Unicode code point` |
| one rune | `error: expected exactly one character, got %d` |

## Suggested milestones

1. Top-level dispatch + `conv` (pure `strconv`, base 0..36).
2. A `parseInt(s string) (uint64, bool, error)` helper returning value + negativity;
   `show` report using `math/bits`.
3. `calc` with wraparound: compute in `uint64`, mask to width, reinterpret for signed;
   detect overflow by comparing against the exact `big`-free arithmetic (hint: for `+`,
   overflow iff `(a>0 && b>0 && sum<a)` style checks, done per width).
4. `not`.
5. A `Perm` bit-flag type (`Execute = 1 << iota; Write; Read`) with a `String()`;
   `perms` both directions.
6. `char` / `ord` with `utf8.RuneCountInString`, `strconv.QuoteRune`, `%U`.
7. Tests.

## Project layout

```
projects/02-bit-toolkit/
  cmd/bits/main.go            // subcommand dispatch, flag sets, I/O
  internal/bitkit/convert.go  // base conversion
  internal/bitkit/report.go   // show / calc / not blocks
  internal/bitkit/perm.go     // Perm flag type + String() + parse/encode
  internal/bitkit/text.go     // char / ord
  internal/bitkit/*_test.go
  cmd/bits/main_test.go
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly.
- [ ] `calc` wraparound is correct for all four widths, signed and unsigned, verified by
      test against known values.
- [ ] `Perm.String()` round-trips with the parser for all 512 modes (`TestPermRoundTrip`).
- [ ] `math/bits` is used for popcount / leading / trailing zeros / bit length — you do
      **not** hand-roll these.

## Test requirements

- `TestConv` — S1–S9 plus base-36, negatives, base-0 detection.
- `TestConvErrors` — F4–F7.
- `TestShow` — S10–S14; assert the whole block string.
- `TestCalcWraparound` — a matrix of `{op, a, b, width, signed}` → `{result, overflow}`
  covering S15–S24 and both-sign overflow for `+ - *` at width 8 and 64.
- `TestPermRoundTrip` — for `m := 0; m < 512; m++`: `encode(decode(m)) == m`.
- `TestChar` / `TestOrd` — S31–S35, F18–F21, including a 4-byte rune (😀).
- `ExampleShow` — a doc example.

## Stretch goals

- `bits show` also prints `reversed` (`bits.Reverse64` at width) and `rotl1`
  (`bits.RotateLeft64`).
- `bits gray <n>` — binary ↔ Gray code.
- `bits ip <a.b.c.d>` — pack/unpack a dotted-quad into a `uint32` and back (previews
  `encoding/binary`).
- A `bits float <x>` that shows the IEEE-754 sign / exponent / mantissa fields using
  `math.Float64bits`.

## Self-check questions

1. `uint8(255) + uint8(1)` is `0` with no panic, but `255 + 1` as untyped constants
   won't even compile if assigned to `uint8`. Why the difference?
2. `int8(-1) >> 1` is `-1`; `uint8(255) >> 1` is `127`. Same bits going in — why
   different results?
3. `a &^ b` vs `a & ^b`: are they always equal in Go? What is the type of `^b` here?
4. Why does `bits show 0` report `bit length 0` and not `1`? What does `bits.Len64(0)`
   return?
5. In `calc`, you compute everything in `uint64` then mask. How do you detect signed
   overflow for `a * b` at width 8 without a big-integer library?
6. `bits ord €` — how many bytes is that argument, how many runes, and which of
   `len(s)` / `utf8.RuneCountInString(s)` / `[]rune(s)` tells you what you need?
