# Project 22 — File-Backed Storage Engine

| | |
|---|---|
| **Difficulty** | 5 / 5 |
| **Estimated time** | 12–16 hours |
| **Prerequisites** | [07](07-todo-cli.md), [10](10-binary-file-parser.md), [17](17-in-memory-cache.md) |
| **Builds toward** | [25 – TCP KV Store](25-kv-store.md) |

## Why this project

This is the mental model underneath every embedded database: an in-memory index backed
by a **write-ahead log** (WAL) for durability and **periodic snapshots** to bound recovery
time, with **crash recovery** that must tolerate a torn write (the process died mid-`fsync`)
without corrupting anything. You built the binary-framing and CRC skills in Project 10;
here you apply them to a format you control end-to-end, and you **fuzz the decoder**
because this code parses bytes that might be truncated at any point.

## Go concepts you MUST use

- [ ] `encoding/binary` for record framing (length prefix, CRC trailer)
- [ ] `encoding/gob` for the record payload (key, value, op)
- [ ] an append-only **write-ahead log**: every mutation is durably recorded before it's
      considered committed
- [ ] periodic **snapshots** that let recovery skip most of the log
- [ ] **crash recovery**: replay the WAL from the last snapshot, and stop cleanly (not
      erroring) at the first sign of a torn/incomplete final record
- [ ] file **locking**: an exclusive `LOCK` file via `os.OpenFile(..., O_CREATE|O_EXCL, ...)`
      so two processes can't open the same directory
- [ ] `bufio.Writer` batching + explicit `Sync` (fsync) — a configurable "sync every N
      writes" durability/throughput trade-off
- [ ] `io.SectionReader` when reading a specific record back out of the WAL
- [ ] `t.Cleanup` for temp-directory test fixtures
- [ ] a **fuzz target** for the WAL record decoder (`FuzzDecodeRecord`) that must never
      panic on truncated/corrupt bytes

## Background

**On-disk layout** (a directory):

```
<dir>/LOCK           empty file, existence = "in use"
<dir>/wal.log         append-only log of records since the last snapshot
<dir>/snapshot.db      the full key/value state as of the last Snapshot() call (may be absent)
```

**WAL record framing** (each record, back to back):

```
length  uint32 LE     // byte length of the gob payload that follows
payload []byte        // gob-encoded Record{Op byte; Seq uint64; Key, Value []byte}
crc     uint32 LE     // crc32.ChecksumIEEE(payload)
```

`Op`: `1` = Put, `2` = Delete (Value is empty/ignored for Delete).

**Snapshot file:** `gob`-encoded `map[string][]byte` (tombstones are simply absent —
deletes are already applied before writing a snapshot), preceded by a `uint32` length and
followed by a `crc32` trailer, same framing discipline as WAL records.

**Recovery algorithm** (`Open`):

1. Take the `LOCK` (fail fast if already held).
2. If `snapshot.db` exists and its CRC checks out, load it into the in-memory map.
   If its CRC is **bad**, that's a hard error — a corrupt snapshot is not recoverable
   (there's nothing to fall back to).
3. Open `wal.log`; read records one at a time. For each: check the length is plausible
   (not absurd, not beyond remaining file size), read the payload + CRC, verify the CRC.
   - **Full match:** apply the record (`Put`/`Delete`) to the in-memory map, continue.
   - **Anything else** (short read on length, short read on payload, CRC mismatch): this
     is the **tail of a torn write** — stop replaying here, keep everything decoded so
     far, and **truncate the WAL file to the last known-good offset** (so future appends
     don't leave garbage in the middle of the file). This is not an error.
4. Ready for use.

**Idempotency matters here.** If a crash happens between writing a new `snapshot.db` and
truncating `wal.log`, the next `Open` replays WAL records that are *already* reflected in
the snapshot. That's fine — `Put`/`Delete` are idempotent, so replaying them again is a
no-op in effect. (See self-check 5.)

## Requirements

### API

```go
type Engine struct { /* unexported */ }

func Open(dir string, opts ...Option) (*Engine, error)
func WithSyncEvery(n int) Option    // fsync after every n writes; default 1 (every write)

func (e *Engine) Get(key []byte) ([]byte, bool)
func (e *Engine) Put(key, value []byte) error
func (e *Engine) Delete(key []byte) error
func (e *Engine) Len() int
func (e *Engine) Snapshot() error   // atomic: write snapshot.tmp, fsync, rename, THEN truncate WAL
func (e *Engine) Close() error      // flush, sync, release LOCK

var ErrLocked = errors.New("storage: directory is locked by another process")
var ErrClosed = errors.New("storage: engine is closed")
var ErrCorruptSnapshot = errors.New("storage: snapshot is corrupt")
```

`Get`/`Put`/`Delete`/`Len` are safe for concurrent use (`RWMutex`: `RLock` for `Get`,
full `Lock` for mutations — writes must also append to the WAL in order, so they're
serialized anyway).

### Behaviour specification

| Scenario | Expectation |
|---|---|
| `Open` a fresh empty directory | succeeds, `Len()==0` |
| `Put(a,1)`, `Get(a)` | `1, true` |
| `Put(a,1)`, `Delete(a)`, `Get(a)` | `nil, false` |
| `Put(a,1)`, `Close`, `Open` again | `Get(a)` → `1, true` (WAL replay) |
| `Put(a,1)`, `Snapshot()`, `Close`, `Open` | `Get(a)` → `1, true`, and the WAL is now empty (no records to replay) |
| `Put(a,1)`, `Snapshot()`, `Put(b,2)`, `Close`, `Open` | both `a` and `b` present (snapshot + WAL replay combine) |
| `Open` a directory with a truncated WAL (last record cut mid-payload) | opens successfully; every record **before** the truncation point is present; the WAL file on disk is now shorter (truncated to the valid prefix) |
| `Open` a directory with a WAL record whose CRC is wrong (bit flip) but full length | same as above — stop at that record, truncate |
| `Open` a directory whose `snapshot.db` CRC is wrong | `ErrCorruptSnapshot` |
| `Open` the same directory twice concurrently | second `Open` returns `ErrLocked` |
| `Open`, `Close`, `Open` again | second `Open` succeeds (lock released) |
| any call on a closed `Engine` | returns `ErrClosed` |
| `WithSyncEvery(5)`; 12 `Put`s; kill the process (simulated: don't call `Close`) before the 13th | on reopen, at least the first 10 puts (2 fsync batches) are guaranteed present; the last ≤4 may or may not be, and if partially written the tail-truncation rule applies — **never** a corrupt state |

### Crash-recovery fixtures (you must build these, byte-exact, under `testdata/`)

1. `clean/` — a normal directory: 5 puts, 1 delete, no snapshot.
2. `snapshotted/` — 5 puts, a `Snapshot()`, 2 more puts.
3. `torn_midlength/` — a WAL truncated inside the 4-byte length prefix of the last record.
4. `torn_midpayload/` — truncated inside the gob payload.
5. `torn_midcrc/` — payload complete, truncated inside the 4-byte CRC trailer.
6. `bad_crc/` — a full record whose CRC has been hand-corrupted (flip one byte).
7. `corrupt_snapshot/` — a `snapshot.db` with a flipped byte.

Write a small `testdata/gen_test.go` (or a `//go:build ignore` generator) that *builds*
these programmatically by opening a real `Engine`, doing operations, closing it, then
truncating/corrupting specific bytes — don't hand-craft the gob bytes by hand.

### The CLI (`kvstore`) — a thin wrapper for manual testing

| Command | Behaviour | Exit |
|---|---|---|
| `kvstore -dir D put KEY VALUE` | `Put`, print nothing | `0` / `1` on error |
| `kvstore -dir D get KEY` | print the value, or `(not found)` | `0` |
| `kvstore -dir D del KEY` | `Delete` | `0` / `1` |
| `kvstore -dir D snapshot` | `Snapshot()` | `0` / `1` |
| `kvstore -dir D dump` | print every key/value, sorted by key | `0` |
| (bad usage) | usage to stderr | `2` |

## Suggested milestones

1. Record framing: `encodeRecord`/`decodeRecord` (binary length + gob + crc32); round-trip
   tests; `FuzzDecodeRecord` from day one (feed it garbage as you build).
2. In-memory engine (no persistence): `Put`/`Get`/`Delete`/`Len` behind `RWMutex`.
3. WAL append on every mutation; `bufio.Writer` + `WithSyncEvery`.
4. WAL replay on `Open` (happy path — a clean, complete log).
5. Snapshot write (atomic temp+rename, like Project 07) + WAL truncation after.
6. Tail-truncation recovery for torn writes — this is the hard part; build the fixtures
   first, then make recovery pass against them one by one.
7. `LOCK` file.
8. `kvstore` CLI. Full test suite.

## Project layout

```
projects/22-storage/
  cmd/kvstore/main.go
  internal/storage/record.go     // encode/decode, crc
  internal/storage/wal.go        // append, replay, tail-truncate
  internal/storage/snapshot.go   // write/load, atomic rename
  internal/storage/engine.go     // Open/Put/Get/Delete/Snapshot/Close, locking
  internal/storage/record_fuzz_test.go
  internal/storage/*_test.go
  testdata/gen_test.go           // builds the crash-recovery fixtures
  testdata/{clean,snapshotted,torn_midlength,...}/
  cmd/kvstore/main_test.go
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every behaviour-table row is a test; every crash-recovery fixture has a dedicated
      test asserting the exact final key set after `Open`.
- [ ] `FuzzDecodeRecord` runs ≥5 minutes clean (never panics on any byte slice).
- [ ] `go test -race ./...` clean (concurrent `Get`/`Put`/`Delete` stress test included).
- [ ] After recovering from a torn WAL, the on-disk WAL file is provably shorter
      (truncated) — a subsequent `Open` of the *already-recovered* directory replays
      cleanly with no further truncation (idempotent recovery).
- [ ] A kill-and-reopen property test: run a random sequence of `Put`/`Delete`/
      `Snapshot` ops against a **real OS file**, "crash" by copying the directory at a
      random byte offset into the WAL mid-write (simulate via truncating a copy), reopen,
      and assert the resulting state is a **prefix** of the true op sequence (never a
      state that couldn't have existed at any real point in time).
- [ ] `Snapshot()` is safe to call concurrently with in-flight `Put`s (assert with `-race`
      and a correctness check afterward).

## Test requirements

- `TestRecordRoundTrip`, `FuzzDecodeRecord`.
- `TestBasicOps`, `TestPersistAcrossReopen`, `TestSnapshotThenReopen`.
- `TestTornWAL_*` — one test per fixture (midlength, midpayload, midcrc, bad_crc).
- `TestCorruptSnapshot`.
- `TestLocking` — second `Open` fails while first is open; succeeds after `Close`.
- `TestClosedEngineErrors`.
- `TestConcurrentOps` (`-race`).
- `TestCrashRecoveryProperty` (the kill-and-reopen property test above).
- `TestSnapshotConcurrentWithPuts`.
- `ExampleEngine`.

## Stretch goals

- Compact the WAL by merging with the snapshot in the background (a real "compaction"
  cycle) instead of only at explicit `Snapshot()` calls.
- Range scans: `Engine.Range(prefix []byte) iter.Seq2[[]byte, []byte]` (bridge to
  Project 11's iterators).
- A second on-disk index (a sparse offset index into the WAL) so `Get` for a key not yet
  snapshotted doesn't require replaying from the start on cold cache.
- Multi-version: keep the last N values per key with timestamps.
- Replace `gob` with your own `encoding/binary` format for the payload too, and compare
  size/throughput.

## Self-check questions

1. Recovery hits a WAL record with a bad CRC. Why is "stop and truncate" the right move
   instead of "return an error and refuse to open" — think about what state the process
   was actually in when it died.
2. Your fixture `torn_midcrc/` has a **complete, valid** gob payload but a truncated CRC
   trailer. Should that record's mutation be applied? Walk through why "the payload
   looked fine" isn't enough — what exactly makes a record trustworthy?
3. `Snapshot()` writes `snapshot.tmp`, fsyncs, renames over `snapshot.db`, *then*
   truncates the WAL. What could go wrong if you truncated the WAL **before** confirming
   the snapshot rename succeeded?
4. Two goroutines call `Put` concurrently. Your WAL append must preserve *some* total
   order (it's a single file). Where exactly does that ordering get enforced — the
   `RWMutex`, the `bufio.Writer`, or the OS file position? What breaks if you used
   `RLock` for `Put`?
5. Explain, precisely, why replaying an already-snapshotted `Put(a, 1)` a second time
   from a stale WAL tail is safe, but why that would **not** be safe if your engine
   supported an "increment counter" operation instead of only `Put`/`Delete`.
6. `os.OpenFile(lockPath, O_CREATE|O_EXCL, 0o600)` — what does `O_EXCL` guarantee, and why
   is a plain `os.Stat` check for the lock file's existence (then create it) **not**
   equivalent (race it in your head with two processes starting at the same instant)?
7. Your fuzzer will likely find that a record claiming `length = 4_000_000_000` on a
   50-byte file causes a problem if not handled. What's the fix, and where does it belong
   relative to `io.ReadFull`?
