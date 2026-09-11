# Project 25 — TCP Key-Value Store (mini-Redis)

| | |
|---|---|
| **Difficulty** | 5 / 5 |
| **Estimated time** | 14–18 hours |
| **Prerequisites** | [19](19-pubsub-broker.md), [22](22-storage-engine.md), [23](23-tcp-chat-server.md) |
| **Builds toward** | [26 – Reverse Proxy](26-reverse-proxy.md), [30 – Interpreter](30-interpreter.md) |

## Why this project

The capstone of the networking tier: implement a real subset of the **RESP** protocol
(the actual Redis wire format — binary-safe, well documented, and a genuinely good
protocol to have implemented once in your life), back it with persistence, add
`SUBSCRIBE`/`PUBLISH`, and ship a typed Go **client package** alongside the server. This
pulls together Project 22's storage engine, Project 23's connection-handling patterns, and
Project 19's pub/sub broker into one program.

## Go concepts you MUST use

- [ ] a real **wire protocol**: RESP2 — Simple Strings, Errors, Integers, Bulk Strings,
      Arrays — parsed and written by hand
- [ ] a concurrency-safe store (adapt Project 22's engine — see the `internal/` note
      below)
- [ ] request **pipelining**: a client may send many commands before reading any replies;
      your connection loop must handle that (read loop and write loop, or read-then-queue)
- [ ] a `SUBSCRIBE` command that switches a connection into push-delivery mode (adapt
      Project 19's broker)
- [ ] a typed **client package** (`internal/redisclient` or similar) other Go programs
      (and your own tests/benchmarks) import
- [ ] `go:embed` for the `HELP` command's text
- [ ] a benchmark that hits the server over a **real socket**, with and without
      pipelining

## Background

**`internal/` strikes again.** Project 22's engine lives under
`projects/22-storage/internal/storage` and Project 19's broker under
`projects/19-pubsub/internal/broker` — neither is importable from here, by the same rule
as Project 17. Copy/adapt trimmed-down versions into this project's own `internal/`
(document what you simplified and why) rather than fighting the visibility rule.

**RESP2, the subset you need:**

| Type | Wire form | Example |
|---|---|---|
| Simple String | `+<text>\r\n` | `+OK\r\n` |
| Error | `-<text>\r\n` | `-ERR unknown command 'FOO'\r\n` |
| Integer | `:<n>\r\n` | `:42\r\n` |
| Bulk String | `$<len>\r\n<len bytes>\r\n` | `$5\r\nhello\r\n` |
| Null Bulk String | `$-1\r\n` | (missing key) |
| Array | `*<n>\r\n<n elements>` | `*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n` |
| Null Array | `*-1\r\n` | |

Clients send **every command** as an Array of Bulk Strings — even `PING` is
`*1\r\n$4\r\nPING\r\n`. Your parser only needs to read that shape from clients; your
encoder needs to write all six reply shapes above.

**TTL encoding.** The underlying engine only stores raw bytes per key. Encode a value as
`[8-byte expiry unix-nanos, 0 = never][raw value bytes]`; decode on read, treating an
expired entry as absent (lazy expiry, same idea as Project 17) — no key is physically
purged until overwritten, deleted, or (stretch) an active sweeper runs.

## Requirements

### Commands

| Command | Args | Reply | Notes |
|---|---|---|---|
| `PING` | — | `+PONG` | |
| `SET` | `key value [EX seconds]` | `+OK` | |
| `GET` | `key` | bulk or null bulk | |
| `DEL` | `key [key...]` | `:N` | N = keys actually deleted |
| `EXISTS` | `key` | `:0` / `:1` | |
| `EXPIRE` | `key seconds` | `:0` / `:1` | 0 if key missing |
| `TTL` | `key` | `:-2` (no key) / `:-1` (no expiry) / `:N` (seconds left) | |
| `INCR` / `DECR` | `key` | `:N` (new value) | value must parse as int64, else error |
| `MSET` | `k1 v1 [k2 v2 ...]` | `+OK` | even arg count required |
| `MGET` | `key [key...]` | array (null bulk per missing) | |
| `KEYS` | `pattern` | array of bulk strings | glob: `*` any run, `?` one char |
| `PUBLISH` | `topic message` | `:N` | N = subscribers reached (via the broker) |
| `SUBSCRIBE` | `topic [topic...]` | one `*3` array per topic: `subscribe topic count`, then the connection enters push mode | |
| `UNSUBSCRIBE` | `[topic...]` (none = all) | one `*3` array per topic: `unsubscribe topic count` | |
| `SAVE` | — | `+OK` | triggers the engine's snapshot |
| `HELP` | — | bulk string (the embedded help text) | |
| `QUIT` | — | `+OK`, then the connection closes | |

**Push messages** while subscribed: `*3\r\n$7\r\nmessage\r\n$<len>\r\n<topic>\r\n$<len>\r\n<data>\r\n`,
interleaved with any reply the client is still owed for other commands (a subscribed
connection may still issue `SUBSCRIBE`/`UNSUBSCRIBE`/`PING`/`QUIT`; other commands while
subscribed → error, matching real Redis).

### Errors

| Condition | Reply |
|---|---|
| unknown command | `-ERR unknown command '<name>'` |
| wrong arity | `-ERR wrong number of arguments for '<name>' command` |
| `INCR`/`DECR` on a non-integer value | `-ERR value is not an integer` |
| a command other than `SUBSCRIBE`/`UNSUBSCRIBE`/`PING`/`QUIT` while subscribed | `-ERR only (P)SUBSCRIBE / (P)UNSUBSCRIBE / PING / QUIT allowed in this context` |
| malformed protocol frame (bad `*`/`$` length, non-array command) | server sends `-ERR Protocol error: <detail>` then **closes the connection** |

### Exit codes (the binary)

| Code | Meaning |
|---|---|
| `0` | clean shutdown (SIGINT/SIGTERM, engine closed, snapshot on the way out if `-save-on-exit`) |
| `1` | bind failure, or engine failed to open (`-dir` locked/corrupt) |
| `2` | bad flags |

### Case specification — SUCCESS (via the client package against a real listener)

| # | Sequence | Assertion |
|---|---|---|
| S1 | `PING` | `PONG` |
| S2 | `SET a 1`; `GET a` | `OK`; `"1", true` |
| S3 | `GET missing` | `"", false` (null bulk) |
| S4 | `SET a 1 EX 60`; `TTL a` | `OK`; `N` with `0 < N <= 60` |
| S5 | `SET a 1`; `TTL a` | `-1` (no expiry) |
| S6 | `TTL missing` | `-2` |
| S7 | `SET a 1 EX 1`; sleep 1.2s; `GET a` | `"", false` (lazily expired) |
| S8 | `DEL a b c` where only `a`, `c` exist | `2` |
| S9 | `INCR counter` three times from missing | `1`, `2`, `3` |
| S10 | `SET x notanumber`; `INCR x` | error `-ERR value is not an integer` |
| S11 | `MSET a 1 b 2 c 3`; `MGET a b z c` | `["1","2",nil,"3"]` |
| S12 | `SET foo:1 x`; `SET foo:2 y`; `SET bar z`; `KEYS foo:*` | `["foo:1","foo:2"]` (order sorted for determinism) |
| S13 | client A subscribes to `news`; client B `PUBLISH news hi` | A's channel receives `{Topic:"news", Data:"hi"}`; `PUBLISH` returns `1` |
| S14 | `PUBLISH news hi` with zero subscribers | returns `0` |
| S15 | `SET a 1`; `SAVE`; restart the server against the same `-dir`; `GET a` | `"1", true` (persisted) |
| S16 | 100 pipelined `SET`s sent without reading replies, then read 100 replies | all `OK`, in order |
| S17 | `HELP` | non-empty bulk string containing every command name |
| S18 | `QUIT` | `OK`, then the connection is closed by the server |

### Case specification — FAILURE

| # | Sequence | Reply |
|---|---|---|
| F1 | `FROB a b` | `-ERR unknown command 'FROB'` |
| F2 | `SET a` (missing value) | `-ERR wrong number of arguments for 'SET' command` |
| F3 | `MSET a 1 b` (odd count) | `-ERR wrong number of arguments for 'MSET' command` |
| F4 | subscribed connection sends `GET a` | `-ERR only (P)SUBSCRIBE / (P)UNSUBSCRIBE / PING / QUIT allowed in this context` |
| F5 | a raw connection (not via the client lib) sends `*1\r\n$-5\r\n` (negative bulk length where not allowed) | `-ERR Protocol error: ...`, connection closed |
| F6 | a raw connection sends a non-array top-level frame, e.g. `+hello\r\n` as a "command" | `-ERR Protocol error: expected array`, connection closed |
| F7 | server started with `-dir` pointing at a directory locked by another running instance | process exits `1` with a clear message |

### Error catalogue

Reply text is exactly as shown in the tables above (`-ERR ...`). Startup errors print
`error: %v` to stderr per the usual CLI convention.

## Suggested milestones

1. RESP encoder/decoder — pure functions over a `bufio.Reader`/`io.Writer`. Test with
   hand-built byte strings, including malformed frames (F5, F6).
2. Command dispatch table (`map[string]func(store, args) Reply`) for the non-pub/sub
   commands, backed by your adapted storage engine + TTL encoding.
3. Connection loop: read a command, dispatch, write the reply, repeat; pipelining falls
   out naturally if you don't do anything synchronous-per-command beyond read→write.
4. `SUBSCRIBE`/`UNSUBSCRIBE`/`PUBLISH` wired to your adapted broker; a writer goroutine
   per subscribed connection so push messages and command replies don't race on the
   socket (reuse the "one writer goroutine, everyone else sends to a channel" shape from
   Project 23).
5. `SAVE`, startup persistence, `-dir` locking (reusing Project 22's engine semantics).
6. `go:embed` `HELP` text.
7. The client package; a benchmark with and without pipelining.
8. Full protocol + command test suite.

## Project layout

```
projects/25-kvstore/
  cmd/kvstored/main.go
  cmd/kvstore-cli/main.go        // a tiny interactive/one-shot client CLI, optional
  internal/resp/resp.go          // encode/decode RESP2
  internal/resp/*_test.go
  internal/kvengine/...          // adapted Project 22 engine (trimmed) — document the adaptation
  internal/kvengine/ttl.go       // expiry encoding
  internal/broker/...            // adapted Project 19 broker (trimmed)
  internal/server/commands.go
  internal/server/conn.go
  internal/server/server.go
  internal/server/help.txt       // //go:embed
  redisclient/client.go          // the public client package
  redisclient/*_test.go
  internal/server/*_test.go
  cmd/kvstored/main_test.go
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly.
- [ ] Pipelining (S16) works with **no explicit batching code** in the client — it's a
      natural consequence of the connection loop's shape; explain why in the README.
- [ ] `SUBSCRIBE` push messages never interleave mid-frame with a command reply on the
      wire (verify by parsing the raw byte stream in a test and asserting every frame is
      complete and well-formed).
- [ ] Persistence round-trips through a real restart (S15) in an integration test that
      actually stops and starts the server process (or the equivalent in-process
      `Open`/`Close`/`Open`).
- [ ] `go test -race ./...` clean.
- [ ] The benchmark's README table shows pipelined throughput meaningfully higher than
      unpipelined (report the real numbers).

## Test requirements

- `TestRESPEncodeDecode` — every reply type, round-trip; malformed frames (F5, F6).
- `TestCommands` — one test per command from the table, happy path + its error rows.
- `TestTTLEncoding`, `TestLazyExpiry`.
- `TestPubSub` — S13, S14, F4.
- `TestPipelining` — S16, plus a raw-byte-stream test proving out-of-order replies never
  happen.
- `TestPersistenceAcrossRestart` — S15.
- `TestDirLocking` — F7.
- `TestConcurrentClients` (`-race`, many simultaneous connections hammering the store).
- `BenchmarkThroughput` — `b.Run` unpipelined vs pipelined-100.
- `ExampleClient`.

## Stretch goals

- `EXPIRE`/active sweeper (bridge back to Project 17's background-eviction idea).
- `MULTI`/`EXEC` — a minimal transaction that queues commands and applies them atomically.
- `LPUSH`/`RPUSH`/`LRANGE` — a list type, forcing a second value encoding.
- RESP3 push-type support / protocol negotiation via `HELLO`.
- A `PSUBSCRIBE` with glob patterns (reuse `KEYS`' matcher), mirroring real Redis.
- Replication: a second instance that `SUBSCRIBE`s to a firehose of writes and applies
  them (a very small preview of distributed systems).

## Self-check questions

1. Why must **every** client command be a RESP Array of Bulk Strings, even `PING`, rather
   than letting simple commands use a Simple String on the wire? (Hint: what does a value
   containing a literal `\r\n` do to a non-length-prefixed format?)
2. Walk through why pipelining works "for free" with a simple `for { read; dispatch;
   write }` loop and requires **no** buffering of multiple commands — what property of
   TCP and `bufio.Reader` makes this true?
3. A subscribed connection needs to both *receive* pushed messages from the broker and
   *reply* to `PING`/`UNSUBSCRIBE` it sends. Why does this force a dedicated writer
   goroutine per connection (reading from a channel) rather than writing directly from
   both the broker's fan-out and the command-dispatch code?
4. Your TTL encoding puts the expiry in the first 8 bytes of the stored value. What
   happens to `INCR` on a key with a TTL — does it need to re-read and re-encode the
   expiry, or can it be lost? Which did you implement, and is that correct?
5. `KEYS foo:*` against a store with a million keys — what does a real Redis
   documentation page warn you about this command, and does your implementation share
   that problem? (You don't have to fix it — just answer honestly.)
6. F5 and F6 both close the connection after a protocol error, rather than trying to
   resynchronize. Why is "give up and disconnect" the correct response to a malformed
   frame, instead of trying to skip to the next plausible frame boundary?
7. You couldn't `import` Project 22's or Project 19's `internal` packages. Name the two
   options you had (copy vs. promote out of `internal/`) and explain, given how this repo
   is organized as many independent learning projects rather than one shipped product,
   why copying was the right call here even though it duplicates code.
