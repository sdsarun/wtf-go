# Project 07 — Todo CLI (JSON store, atomic writes)

| | |
|---|---|
| **Difficulty** | 2 / 5 |
| **Estimated time** | 5–7 hours |
| **Prerequisites** | [01](01-unit-converter-cli.md)–[06](06-matrix-vector-library.md) |
| **Builds toward** | [12 – JSON Config Loader](12-json-config-loader.md), [22 – Storage Engine](22-storage-engine.md) |

## Why this project

A persistent CLI: it reads and rewrites a JSON file every run. That forces `encoding/json`
with struct tags and custom text marshalling, safe file replacement (write-temp +
`os.Rename`, never truncate-in-place), `defer` for cleanup (including the classic
argument-evaluation gotcha), a **custom `flag.Value`**, and **sentinel errors** checked
with `errors.Is`.

## Go concepts you MUST use

- [ ] `encoding/json` — struct tags, `omitempty`, `json.MarshalIndent`, `Unmarshal`
- [ ] a type implementing `encoding.TextMarshaler` / `TextUnmarshaler` (`Priority`),
      reused by **both** `encoding/json` and `flag.TextVar`
- [ ] a **custom `flag.Value`** (`String()` + `Set()`) for the repeatable `-tag` flag
- [ ] subcommands via `flag.NewFlagSet`
- [ ] `defer` — closing files, removing the temp file on failure; explain the
      argument-evaluation timing in a self-check
- [ ] `os` — `UserHomeDir`, `Getenv`, `MkdirAll`, `CreateTemp`, `Rename`, `Chmod`,
      `FileMode(0o600)`, `os.Stat`, `errors.Is(err, os.ErrNotExist)`
- [ ] **sentinel errors** (`ErrNotFound`, `ErrEmptyText`) and `errors.Is`
- [ ] `slices.DeleteFunc` / `slices.IndexFunc` for list mutation
- [ ] an injected `now func() time.Time`

## Background

**Atomic replace.** If you `os.Create` the real file and the process dies mid-write, the
user's data is gone. Instead: write a sibling temp file, `Sync` it, `Chmod` to `0600`,
then `os.Rename` it over the target — `rename(2)` is atomic on the same filesystem. A
`defer os.Remove(tmp)` cleans up if you bail before the rename.

**File location** (first that applies): `-file` flag › `$TODO_FILE` › `$HOME/.todo.json`.

**Missing file is not an error** — it means "empty list". The file is created on the first
mutation.

## Requirements

### Functional requirements

1. `todo <subcommand> …`. Subcommands: `add`, `list`, `show`, `done`, `undone`, `edit`,
   `rm`, `clear-done`. Missing/unknown → usage, exit 2.
2. **`add <text...>`** — the text is all remaining positional args joined with spaces;
   empty → error. Flags: `-due DATE`, `-priority low|med|high` (default `med`),
   `-tag NAME` (repeatable; also accepts `-tag a,b`). Assigns `id = next_id`, increments
   `next_id`, sets `created = now()`. Prints `added #<id>`.
3. **`list`** — pending items only unless `-all`. Filters: `-tag NAME`, `-priority P`,
   `-due-before DATE`, `-done` (only done). Sort `-sort priority|due|created|id`
   (default `priority`: priority desc, then due asc with "no due" last, then id asc).
   `-asc` reverses the primary key. Output is a `tabwriter` table (columns below); no
   rows → print `(no matching items)`.
4. **`show <id>`** — full detail block for one item.
5. **`done <id>` / `undone <id>`** — toggle `done`; `done` sets `completed = now()`,
   `undone` clears it. Already in that state → no-op, still exit 0, print
   `#<id> already done` / `already pending`.
6. **`edit <id>`** — any of `-text`, `-due` (`-due ""` clears it), `-priority`, `-tag`
   (replaces the tag set; `-tag ""` clears). At least one flag required.
7. **`rm <id>`** — delete; prints `removed #<id>`.
8. **`clear-done`** — remove all done items; prints `removed N item(s)`.
9. Every mutating command rewrites the file atomically. `list` / `show` never write.
10. DATE grammar: `2006-01-02`, `2006-01-02T15:04:05Z07:00`, `today`, `tomorrow`,
    `+Nd` (N days from `now()`), or empty (only where clearing is allowed). Stored as
    RFC 3339 UTC.

### JSON file schema

```json
{
  "next_id": 4,
  "items": [
    {
      "id": 1,
      "text": "buy milk",
      "done": false,
      "priority": "high",
      "tags": ["errand"],
      "due": "2024-03-10T00:00:00Z",
      "created": "2024-03-01T09:00:00Z",
      "completed": null
    }
  ]
}
```

`tags`, `due`, `completed` use `omitempty` (well — `completed` is `*time.Time`, so it's
`null` when unset and omitted only if you tag it `,omitempty` on the pointer; either is
acceptable, document your choice). Files are written with `MarshalIndent(v, "", "  ")` and
a trailing newline.

### Exact contract — `list` table

`todo list` with items #1 (buy milk, high, due 2024-03-10, tag errand),
#3 (write report, med, no due):

```
ID  P  DUE         TAGS    TEXT
1   H  2024-03-10  errand  buy milk
3   M  -           -       write report
```

`P` is `H`/`M`/`L`. `DUE` is the date part only, or `-`. `TAGS` is comma-joined or `-`.
With `-all`, a leading `[x] ` / `[ ] ` prefix is added to `TEXT`.

### Exact contract — `show 1`

```
#1  buy milk
priority   high
status     pending
due        2024-03-10
tags       errand
created    2024-03-01T09:00:00Z
completed  -
```

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | success (including harmless no-ops) |
| `1` | item not found; empty text; bad date / priority; corrupt todo file; unwritable location |
| `2` | usage error — unknown/missing subcommand, wrong args, unknown flag, `edit` with no flags |

### Resolution / precedence order

1. Dispatch subcommand → unknown/none → usage, exit 2.
2. Parse the subcommand `FlagSet` → flag error → exit 2.
3. Resolve the file path (`-file` › `$TODO_FILE` › `$HOME/.todo.json`).
4. Load: missing file → empty store; present but invalid JSON → exit 1
   (`error: <path>: invalid todo file: <err>`).
5. Validate args (`id` numeric, text non-empty, date parses, priority valid, `edit` has
   ≥1 flag) → exit 1 or 2 per the table.
6. Apply the operation (`ErrNotFound` → exit 1).
7. If mutating: write atomically (failure → exit 1). Print the confirmation line. Exit 0.

### Case specification — SUCCESS

(each run uses `-file $T` pointing at a fresh temp file, and an injected `now` =
`2024-03-09T12:00:00Z`)

| # | Commands (in sequence) | final stdout of last cmd | exit |
|---|---|---|---|
| S1 | `todo add buy milk` | `added #1` | 0 |
| S2 | `todo add "write report" -priority low` | `added #2` | 0 |
| S3 | `todo add ship it -due tomorrow -tag work -tag urgent` | `added #3` | 0 |
| S4 | after S1–S3: `todo list` | 3-row table, sorted high→low priority (all `med`? no: #1 med, #2 low, #3 med) → #1, #3, #2 | 0 |
| S5 | `todo list -tag work` | only #3 | 0 |
| S6 | `todo list -priority low` | only #2 | 0 |
| S7 | `todo done 1` | `done #1` | 0 |
| S8 | after S7: `todo list` | #3, #2 (pending only) | 0 |
| S9 | after S7: `todo list -all` | 3 rows, #1 prefixed `[x]` | 0 |
| S10 | `todo done 1` again | `#1 already done` | 0 |
| S11 | `todo undone 1` | `undone #1` | 0 |
| S12 | `todo show 3` | the detail block (due = 2024-03-10) | 0 |
| S13 | `todo edit 2 -priority high -text "write the report"` | `updated #2` | 0 |
| S14 | `todo edit 3 -due ""` | `updated #3` (due cleared) | 0 |
| S15 | `todo rm 2` | `removed #2` | 0 |
| S16 | `todo add another; todo done 3; todo clear-done` | `removed 1 item(s)` | 0 |
| S17 | `todo list` on a fresh file | `(no matching items)` | 0 |
| S18 | `todo add x -due +3d` then `todo show <id>` | due = `2024-03-12` | 0 |
| S19 | `TODO_FILE=$T todo add via env` (no `-file`) | `added #1`, and `$T` now exists with mode `0600` | 0 |

### Case specification — FAILURE

| # | Command | stderr | exit |
|---|---|---|---|
| F1 | `todo` | usage | 2 |
| F2 | `todo frob` | `error: unknown subcommand "frob"` + usage | 2 |
| F3 | `todo add` | `error: todo text must not be empty` | 1 |
| F4 | `todo add x -priority urgent` | `error: -priority must be low, med, or high` | 1 |
| F5 | `todo add x -due "next week"` | `error: could not parse "next week" as a date` | 1 |
| F6 | `todo done 99` | `error: no item with id 99` | 1 |
| F7 | `todo done abc` | `error: id must be a positive integer, got "abc"` | 1 |
| F8 | `todo edit 1` (no flags) | `error: edit needs at least one of -text, -due, -priority, -tag` | 2 |
| F9 | `todo show 1` with a corrupt file | `error: <path>: invalid todo file: <json error>` | 1 |
| F10 | `todo add x -file /root/nope/.todo.json` (unwritable) | `error: <path>: <mkdir/rename error>` | 1 |

### Error catalogue

| Trigger | Format |
|---|---|
| unknown subcommand | `error: unknown subcommand %q` |
| empty text | `error: todo text must not be empty` |
| priority | `error: -priority must be low, med, or high` |
| date parse | `error: could not parse %q as a date` |
| not found | `error: no item with id %d` |
| bad id | `error: id must be a positive integer, got %q` |
| edit no flags | `error: edit needs at least one of -text, -due, -priority, -tag` |
| corrupt file | `error: %s: invalid todo file: %v` |
| write failure | `error: %s: %v` |

## Suggested milestones

1. `internal/todo`: `Item`, `Priority` (`MarshalText`/`UnmarshalText`, `String`),
   `Store{NextID int; Items []Item}`.
2. `LoadStore(path)` (missing → empty, invalid → wrapped error), `SaveStore(path, s)`
   (atomic temp+rename, `0600`, `defer os.Remove`).
3. Date parsing helper with the injected clock.
4. Operations on `Store`: `Add`, `Find` (returns `ErrNotFound`), `SetDone`, `Edit`,
   `Remove`, `ClearDone`.
5. Filtering + the sort modes; the `tabwriter` renderer; the `show` block.
6. `tagList` custom `flag.Value`; wire each subcommand's `FlagSet`.
7. `main` dispatch + path resolution + exit codes.
8. Golden CLI test driving sequences of commands against a temp file.

## Project layout

```
projects/07-todo/
  cmd/todo/main.go
  internal/todo/model.go     // Item, Priority (+ Text marshalling), Store
  internal/todo/store.go     // LoadStore / SaveStore (atomic)
  internal/todo/ops.go       // Add/Find/SetDone/Edit/Remove/ClearDone, ErrNotFound
  internal/todo/date.go      // parseDate(now, s)
  internal/todo/render.go    // list table + show block
  internal/todo/flagtypes.go // tagList flag.Value
  internal/todo/*_test.go
  cmd/todo/main_test.go
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly.
- [ ] After a mutation, the on-disk file is valid JSON, ends with `\n`, and has mode
      `0600` (`TestFilePermissions`).
- [ ] A simulated write failure (temp dir made read-only mid-test) leaves the **original**
      file untouched (`TestAtomicWriteFailureKeepsOldData`).
- [ ] `ErrNotFound` is a package-level sentinel; callers use `errors.Is`, not string
      matching.
- [ ] `Priority` marshals to/from `"low"`/`"med"`/`"high"` in JSON and is accepted by
      `flag.TextVar`.
- [ ] Nothing calls `time.Now()` outside `main`.

## Test requirements

- `TestPriorityText` — JSON round-trip + `flag.TextVar` accept/reject.
- `TestLoadStore` — missing file → empty; valid file → parsed; invalid JSON → wrapped
  error (`errors.Is` a package `ErrCorrupt`).
- `TestSaveAtomic` — writes temp then renames; concurrent readers never see a partial
  file (write a big store in a loop while reading).
- `TestAtomicWriteFailureKeepsOldData`.
- `TestOps` — add/find/done/edit/remove/clear-done incl. `ErrNotFound`.
- `TestSortModes` — priority/due/created/id, `-asc`, "no due" ordering.
- `TestDateParse` — `today`, `tomorrow`, `+3d`, RFC3339, bad string.
- `TestRenderList` / `TestRenderShow` — golden strings.
- `TestCLISequence` — S1–S19 as command sequences.
- `ExamplePriority_String`.

## Stretch goals

- `todo` with no subcommand prints the pending list (like `list`).
- `-format json` on `list` / `show`.
- File locking: an `O_CREATE|O_EXCL` lock file so two `todo` processes can't interleave
  writes (preview of Project 22).
- `todo undo` — keep the previous file version as `.todo.json.bak` and restore it.
- Recurring todos (`-every 1w`): on `done`, spawn the next occurrence.

## Self-check questions

1. `defer f.Close()` vs `defer func() { f.Close() }()` — for the temp file, does it
   matter? Now consider `defer os.Remove(tmp.Name())` where `tmp` is reassigned later —
   when is `tmp.Name()` evaluated?
2. Your `add` does truncate-write instead of temp+rename. Describe the exact sequence of
   events that loses the user's data, and why `rename` avoids it.
3. `Priority` implements `MarshalText`, not `MarshalJSON`. Why does `encoding/json` still
   emit `"high"` and not `{}`?
4. `-tag work -tag urgent` — how does your `flag.Value.Set` get called, how many times,
   and where do you store the accumulating slice?
5. `errors.Is(err, ErrNotFound)` vs `err == ErrNotFound` vs `strings.Contains(err.Error(),
   "not found")` — which survive `fmt.Errorf("op: %w", ErrNotFound)`, and which is correct?
6. You load the file, mutate the struct, and on save the disk is full. What state is the
   user's todo file in, and did your `defer os.Remove` help or hurt?
7. `completed *time.Time` vs `completed time.Time` with `omitempty` — what does each emit
   when the task isn't done, and which round-trips cleanly?
