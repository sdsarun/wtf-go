# Project 03 — Date & Duration Calculator

| | |
|---|---|
| **Difficulty** | 1 / 5 |
| **Estimated time** | 4–6 hours |
| **Prerequisites** | [01 – Unit Converter CLI](01-unit-converter-cli.md), [02 – Bit Toolkit](02-bit-toolkit.md) |
| **Builds toward** | [17 – In-Memory Cache](17-in-memory-cache.md), [29 – Task Scheduler](29-task-scheduler.md) |

## Why this project

`time` is the standard-library package people get wrong most often: the reference layout,
`Parse` vs `Format`, timezones, `Duration` arithmetic, and the fact that calendar math
(`AddDate`) is **not** the same as adding a `Duration`. This project makes you use all of
it. It also introduces an **injectable clock**, the first taste of dependency injection —
you'll need it constantly from Project 14 on.

## Go concepts you MUST use

- [ ] the reference layout `2006-01-02 15:04:05` and named layout constants; `time.Parse`,
      `time.Format`
- [ ] `time.ParseDuration`, plus a **hand-written parser** for calendar units (`y`, `mo`,
      `w`, `d`)
- [ ] `time.Duration` arithmetic; `Duration.Truncate`, `Duration.Round`; the duration
      unit constants
- [ ] `time.Time` methods: `Add`, `AddDate`, `Sub`, `Before`, `After`, `Equal`, `Compare`
- [ ] `time.Weekday`, `Time.ISOWeek`, `Time.YearDay`, `Time.Date`
- [ ] `time.Location`: `time.UTC`, `time.Local`, `time.LoadLocation`, `time.FixedZone`
- [ ] `time.Since` / `time.Until` via an injected `now func() time.Time`
- [ ] `time.Time` JSON marshalling (RFC 3339) — print results as RFC 3339
- [ ] subcommands via `flag.NewFlagSet`

## Background

**Reference layout.** Go formats and parses times by example: you write what
`Mon Jan 2 15:04:05 MST 2006` (Unix time `1136239445`, i.e. `01/02 03:04:05PM '06 -0700`)
looks like in your desired format. `"2006-01-02"` means "YYYY-MM-DD".

**Duration vs calendar.** `t.Add(24*time.Hour)` adds exactly 86400 seconds — across a DST
boundary that is not "the same wall-clock time tomorrow". `t.AddDate(0, 0, 1)` adds one
calendar day. `AddDate` also **normalizes**: `time.Date(2024, 1, 31, ...).AddDate(0, 1, 0)`
is `2024-03-02`, not "Feb 31" and not "Feb 29" — January 31 + 1 month = March 2. Your
calendar-diff algorithm must account for this.

**Days in a month:** `time.Date(y, m+1, 0, 0, 0, 0, 0, loc)` gives the last day of month
`m` — day `0` of the next month rolls back.

**Determinism:** so tests don't depend on the machine's timezone, this tool defaults to
**UTC** and takes an explicit `-now` override for anything that needs "the current time".

## Requirements

### Functional requirements

1. `when <subcommand> …`. Subcommands: `parse`, `fmt`, `add`, `sub`, `diff`, `weekday`,
   `age`, `cal`, `now`. Missing/unknown → usage, exit 2.
2. **Time argument grammar** (`<t>` everywhere): the tool tries these layouts in order and
   uses the first that parses cleanly:
   1. RFC 3339 (`2006-01-02T15:04:05Z07:00`) and RFC3339Nano
   2. `2006-01-02 15:04:05`
   3. `2006-01-02T15:04:05` (assumed UTC)
   4. `2006-01-02`
   5. `2006/01/02`
   6. `02 Jan 2006`
   7. `Jan 2, 2006`
   8. all-digits → Unix seconds (`time.Unix(n, 0).UTC()`)

   A parsed time with no explicit zone is in **UTC**.
3. **Duration argument grammar** (`<dur>` for `add` / `sub`): a sequence of
   `[-]<int><unit>` terms with no spaces, units `y mo w d h m s ms us ns`. `y`/`mo`/`w`/`d`
   are applied with `AddDate` (`w` = 7 `d`); the rest are summed into a `time.Duration`.
   The calendar part is applied **first**, then the duration part. A Go duration string
   like `1h30m` is also accepted (it has no `y/mo/w/d`).
4. **`when parse <s>`** — print which layout matched and the normalized time:
   ```
   layout  2006-01-02
   rfc3339 2024-03-09T00:00:00Z
   unix    1709942400
   ```
5. **`when fmt <t> [-layout L] [-tz Zone]`** — reformat. `-layout` accepts a Go layout
   string or one of the names `rfc3339`, `rfc1123`, `kitchen`, `date`, `datetime`,
   `unix`. `-tz` is an IANA name (`America/New_York`) or `UTC` (default) or `Local`. Print
   one line: the formatted string.
6. **`when add <t> <dur>` / `when sub <t> <dur>`** — print the resulting time as RFC 3339
   (`sub` negates the whole duration).
7. **`when diff <t1> <t2>`** — print the block:
   ```
   from       2024-01-01T00:00:00Z
   to         2024-03-10T00:00:00Z
   duration   1656h0m0s
   seconds    5961600
   breakdown  69d 0h 0m 0s
   calendar   0y 2mo 9d
   direction  forward
   ```
   `duration` is `t2.Sub(t1)` (Go's `Duration.String`), always non-negative (absolute).
   `direction` is `forward` if `t2 >= t1` else `backward`. `breakdown` splits the absolute
   duration into whole days/hours/minutes/seconds. `calendar` steps with `AddDate`: most
   years `y` such that `from.AddDate(y,0,0) <= to`, then months, then days.
8. **`when weekday <t>`** — block: `weekday`, `iso-week` (`2024-W10`), `day-of-year`.
9. **`when age <birth> [-now T]`** — calendar age as `Xy Ymo Zd` (same algorithm as
   `calendar` in `diff`), plus total days. Error if `birth` is in the future.
10. **`when cal <year> <month>`** — print an ASCII month calendar (Sunday-first), exactly
    the format shown below.
11. **`when now [-tz Zone] [-utc] [-now T]`** — print the current time (RFC 3339). `-now`
    overrides the clock (for tests / scripting).

### Exact contract — `when cal 2024 2`

```
   February 2024
Su Mo Tu We Th Fr Sa
             1  2  3
 4  5  6  7  8  9 10
11 12 13 14 15 16 17
18 19 20 21 22 23 24
25 26 27 28 29
```

Header line: 3 leading spaces, `Month YYYY`. Day cells are width-2, space-separated;
leading blank cells are 2 spaces + the separator. No trailing spaces on any line. Every
line ends with `\n`.

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | success |
| `1` | semantic error — unparseable time / duration, unknown timezone, unknown layout name, month out of range, birth date in the future |
| `2` | usage error — unknown/missing subcommand, wrong arg count, unknown flag |

### Resolution / precedence order

1. Dispatch subcommand; unknown/none → usage, exit 2.
2. Parse the subcommand's `FlagSet`; flag error → exit 2.
3. Arg-count check → exit 2.
4. Parse time/duration args (in the order they appear) → failure → exit 1.
5. Resolve `-tz` (`LoadLocation`) → failure → exit 1.
6. Domain checks (`month` 1–12, `birth <= now`) → exit 1.
7. Compute, print, exit 0.

### Case specification — SUCCESS

| # | Command | key output | exit |
|---|---------|-----------|------|
| S1 | `when parse 2024-03-09` | `layout  2006-01-02`, `rfc3339 2024-03-09T00:00:00Z` | 0 |
| S2 | `when parse 2024-03-09T12:30:00Z` | `layout  2006-01-02T15:04:05Z07:00` | 0 |
| S3 | `when parse "09 Mar 2024"` | `layout  02 Jan 2006` | 0 |
| S4 | `when parse 1709942400` | `layout  unix`, `rfc3339 2024-03-09T00:00:00Z` | 0 |
| S5 | `when fmt 2024-03-09 -layout rfc1123` | `Sat, 09 Mar 2024 00:00:00 UTC` | 0 |
| S6 | `when fmt 2024-03-09T12:00:00Z -tz America/New_York` | `2024-03-09T07:00:00-05:00` | 0 |
| S7 | `when fmt 2024-03-09 -layout kitchen` | `12:00AM` | 0 |
| S8 | `when fmt 2024-03-09 -layout unix` | `1709942400` | 0 |
| S9 | `when add 2024-01-31 1mo` | `2024-03-02T00:00:00Z` | 0 |
| S10 | `when add 2024-02-29 1y` | `2025-03-01T00:00:00Z` | 0 |
| S11 | `when add 2024-03-09 1h30m` | `2024-03-09T01:30:00Z` | 0 |
| S12 | `when add 2024-03-09 1w2d` | `2024-03-18T00:00:00Z` | 0 |
| S13 | `when add "2024-03-09 10:00:00" -1d` | `2024-03-08T10:00:00Z` | 0 |
| S14 | `when sub 2024-03-09 1w` | `2024-03-02T00:00:00Z` | 0 |
| S15 | `when diff 2024-01-01 2024-03-10` | the block above | 0 |
| S16 | `when diff 2024-03-10 2024-01-01` | same numbers, `direction  backward` | 0 |
| S17 | `when weekday 2024-03-09` | `weekday    Saturday`, `iso-week   2024-W10`, `day-of-year 69` | 0 |
| S18 | `when age 2000-06-15 -now 2024-03-09T00:00:00Z` | `age        23y 8mo 22d` | 0 |
| S19 | `when cal 2024 2` | the calendar block above | 0 |
| S20 | `when cal 2023 2` | Feb 2023 (28 days, starts Wed) | 0 |
| S21 | `when now -now 2024-03-09T12:00:00Z` | `2024-03-09T12:00:00Z` | 0 |
| S22 | `when now -now 2024-03-09T12:00:00Z -tz Asia/Tokyo` | `2024-03-09T21:00:00+09:00` | 0 |

### Case specification — FAILURE

| # | Command | stderr | exit |
|---|---------|--------|------|
| F1 | `when` | usage | 2 |
| F2 | `when frobnicate` | `error: unknown subcommand "frobnicate"` + usage | 2 |
| F3 | `when parse` | `error: expected 1 argument (time), got 0` | 2 |
| F4 | `when parse "not a date"` | `error: could not parse "not a date" as a time` | 1 |
| F5 | `when parse 2024-13-01` | `error: could not parse "2024-13-01" as a time` | 1 |
| F6 | `when fmt 2024-03-09 -tz Mars/Olympus` | `error: unknown time zone "Mars/Olympus"` | 1 |
| F7 | `when fmt 2024-03-09 -layout bogus` | `error: unknown layout name "bogus"` | 1 |
| F8 | `when add 2024-03-09 5x` | `error: invalid duration "5x": unknown unit "x"` | 1 |
| F9 | `when add 2024-03-09 ""` | `error: invalid duration "": empty` | 1 |
| F10 | `when diff 2024-03-09` | `error: expected 2 arguments (t1 t2), got 1` | 2 |
| F11 | `when cal 2024 13` | `error: month must be between 1 and 12, got 13` | 1 |
| F12 | `when cal 2024 abc` | `error: "abc" is not a valid month` | 1 |
| F13 | `when age 2999-01-01 -now 2024-03-09T00:00:00Z` | `error: birth date 2999-01-01 is in the future` | 1 |

### Error catalogue

| Trigger | Format |
|---|---|
| unknown subcommand | `error: unknown subcommand %q` |
| arg count | `error: expected N argument(s) (<names>), got M` |
| unparseable time | `error: could not parse %q as a time` |
| unknown zone | `error: unknown time zone %q` |
| unknown layout | `error: unknown layout name %q` |
| bad duration unit | `error: invalid duration %q: unknown unit %q` |
| empty duration | `error: invalid duration %q: empty` |
| month range | `error: month must be between 1 and 12, got %d` |
| bad month | `error: %q is not a valid month` |
| future birth | `error: birth date %s is in the future` |

## Suggested milestones

1. `internal/whenlib`: `ParseTime(string) (time.Time, layoutName string, error)` trying
   the layout list; `parse` subcommand.
2. `ParseDelta(string) (Delta, error)` where `Delta{Years, Months, Days int; Dur time.Duration}`;
   `Delta.ApplyTo(t) time.Time`; `add` / `sub`.
3. `fmt` with the layout-name map and `-tz`.
4. `Diff(a, b time.Time) DiffResult` — absolute duration, day/h/m/s breakdown, and the
   `AddDate`-stepping calendar breakdown. `diff`, `age` (calendar breakdown reused).
5. `weekday`, `now` (with injected `Clock`).
6. `cal` — build the grid from `time.Date(y, m, 1, ...)` weekday and last-day trick.
7. Tests, including DST cases (`America/New_York` across March).

## Project layout

```
projects/03-date-calc/
  cmd/when/main.go
  internal/whenlib/parse.go      // ParseTime, layout table
  internal/whenlib/delta.go      // Delta, ParseDelta, ApplyTo
  internal/whenlib/diff.go       // Diff, calendar breakdown, age
  internal/whenlib/cal.go        // month grid
  internal/whenlib/clock.go      // type Clock func() time.Time
  internal/whenlib/*_test.go
  cmd/when/main_test.go
  README.md
  Makefile
```

The `Clock` is passed into whatever needs "now"; production wires `time.Now`, tests pass a
fixed function. `main` reads `-now` and, if set, builds a fixed clock.

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly, **regardless of the machine's `TZ`** (run the test
      suite with `TZ=America/Los_Angeles go test ./...` and `TZ=Asia/Kolkata go test ./...`).
- [ ] The calendar breakdown is verified against hand-computed cases including
      end-of-month (`2024-01-31` +1mo) and leap day (`2024-02-29` +1y).
- [ ] `cal` output has no trailing spaces and matches byte-for-byte.
- [ ] Nothing calls `time.Now()` outside `main` (grep for it).

## Test requirements

- `TestParseTime` — S1–S4, F4–F5, plus each layout in the list at least once.
- `TestParseDelta` — S9–S14 deltas, F8–F9, negative terms, mixed `1y2mo10d12h`.
- `TestApplyToNormalization` — `2024-01-31 +1mo → 2024-03-02`, `2024-02-29 +1y → 2025-03-01`.
- `TestDiff` — S15–S16; assert the whole block; add a same-instant case (all zeros).
- `TestCalendarBreakdown` — table of `{from, to} → {y, mo, d}` with month-boundary cases.
- `TestCal` — S19–S20 and a month that starts on Sunday and one that needs 6 rows.
- `TestFmtTimezone` — S6 and a DST-transition timestamp; run under two `TZ` env values.
- `TestClockInjection` — `now` and `age` with a fixed clock.
- `ExampleDiff`.

## Stretch goals

- `when biz-days <t1> <t2>` — count weekdays (Mon–Fri) between two dates.
- `when countdown <t> -now T` — "3 days, 4 hours from now" / "2 hours ago".
- `when tz <t> <zone1> <zone2> ...` — a table of the same instant in several zones.
- Accept `now`, `today`, `yesterday`, `tomorrow` as time arguments.
- `when iso <YYYY-Www-D>` — parse an ISO week date.

## Self-check questions

1. Why is the reference layout `2006-01-02 15:04:05` and not `2020-01-02` or
   `YYYY-MM-DD`? What is special about that specific instant?
2. `t.Add(24 * time.Hour)` and `t.AddDate(0, 0, 1)` give different results on exactly two
   days a year in `America/New_York`. Which days, and which call is "one calendar day"?
3. `time.Date(2024, 2, 30, 0, 0, 0, 0, time.UTC)` does not error. What does it return, and
   how does that behaviour help you compute "days in a month"?
4. Your tests pass on your machine but a teammate in India sees failures. What did you
   forget, and why does defaulting to `time.UTC` instead of `time.Local` fix it?
5. `time.Parse("2006-01-02", "2024-03-09")` returns a time with which location, and what
   is its `Zone()`?
6. What is a "monotonic clock reading" on a `time.Time`, when does Go strip it, and why
   does `time.Since` care?
7. In `diff`, why compute the calendar breakdown by stepping with `AddDate` rather than
   dividing the `Duration` by "30 days"?
