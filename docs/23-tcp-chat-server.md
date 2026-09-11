# Project 23 — TCP Chat Server

| | |
|---|---|
| **Difficulty** | 4 / 5 |
| **Estimated time** | 9–13 hours |
| **Prerequisites** | [15](15-log-pipeline.md), [19](19-pubsub-broker.md) |
| **Builds toward** | [24 – HTTP Server From Scratch](24-http-server-scratch.md), [25 – TCP KV Store](25-kv-store.md) |

## Why this project

Your first raw TCP server: one goroutine per connection, a central **hub** goroutine that
owns all shared state (rooms, nicknames — the same actor pattern from Project 15's log
pipeline and Project 14's crawler, now for a live, long-lived protocol), read/write
**deadlines** so a silent client doesn't leak a goroutine forever, and a graceful shutdown
that drains every connection.

## Go concepts you MUST use

- [ ] `net.Listen`, `Listener.Accept` in a loop, one goroutine per `net.Conn`
- [ ] a **hub** goroutine owning `rooms map[string]map[*client]bool` and
      `nicks map[string]*client` — connection goroutines never touch these directly,
      only via channels to the hub (same actor pattern as Project 15)
- [ ] `conn.SetReadDeadline` / `SetWriteDeadline` — idle clients are disconnected, and no
      write can hang the server forever
- [ ] `bufio.Scanner` (or `Reader.ReadString('\n')`) over a `net.Conn`, with a max line
      length
- [ ] `context` for shutdown: `signal.NotifyContext`, cancel → stop accepting, notify and
      close every connection, `sync.WaitGroup` to wait for all connection goroutines to
      exit
- [ ] `net.Listener.Close()` unblocking `Accept`

## Background

**Protocol** — newline-delimited (`\n`) UTF-8 text, max **512 bytes** per line (including
the newline). On connect the server sends one line:

```
WELCOME chatd/1.0
```

The client **must** send `/nick NAME` before anything else (except `/quit`) is accepted.
On success it is placed in room `lobby`.

| Client sends | Server reply (to sender) | Broadcast (to others) |
|---|---|---|
| `/nick NAME` | `OK nick NAME` or `ERR nick_taken` / `ERR invalid_nick` | — |
| `/join ROOM` | `OK join ROOM` | `LEFT OLDROOM NICK` to old room; `JOINED ROOM NICK` to new room's other members |
| `/leave` | `OK join lobby` (back to lobby) or `ERR already_in_lobby` | `LEFT ROOM NICK` / `JOINED lobby NICK` |
| `/list` | `ROOMS room1(n1) room2(n2) ...` (sorted by name) | — |
| `/who` | `USERS nick1 nick2 ...` (current room, sorted) | — |
| `/msg NAME text` | `OK msg` or `ERR no_such_user` | `PRIVATE NICK text` to `NAME` only |
| `/quit` | `OK bye` (then the connection is closed by the server) | `LEFT ROOM NICK` |
| any other line not starting `/` | `OK msg` | `MSG ROOM NICK text` to **every** member of the room **including the sender** (delivery confirmation) |
| unknown `/word ...` | `ERR unknown_command` | — |
| anything before `/nick` (except `/quit`) | `ERR need_nick` | — |
| a line > 512 bytes | `ERR line_too_long` (connection stays open) | — |
| a blank line | ignored, no reply | — |

**Nick rules:** `^[A-Za-z][A-Za-z0-9_]{0,15}$`, unique server-wide (case-insensitive).
Changing nick after joining is **not supported** in this version (a second `/nick` while
already named → `ERR already_named`) — see stretch goals.

**Idle timeout:** if a full line hasn't arrived in **5 minutes**, the server sends
`ERR idle_timeout` and closes the connection.

**Room lifecycle:** a room is created on first `/join` and deleted the instant it becomes
empty (except `lobby`, which always exists).

## Requirements

### Functional requirements

1. `chatd [-addr :6667] [-idle 5m] [-shutdown-grace 3s]`.
2. Exactly the protocol table above, byte-for-byte reply formats.
3. The hub is the **only** place that reads or mutates `rooms`/`nicks`; connection
   goroutines send it commands over a channel and get results back over a per-command
   reply channel (or a channel embedded in the command struct).
4. On `SIGINT`/`SIGTERM`: stop `Accept`ing, send `SERVER shutting_down` to every
   connected client, wait up to `-shutdown-grace` for them to `/quit` on their own, then
   force-close remaining connections, then exit.
5. A client whose write buffer can't keep up within a bounded deadline is disconnected
   (don't let one slow reader stall broadcasts to everyone else — reuse the
   `select`+`time.After` non-blocking-send idea from Project 19).

### Exit codes

| Code | Meaning |
|---|---|
| `0` | clean shutdown |
| `1` | failed to bind `-addr` |
| `2` | bad flags |

### Concurrency requirements (graded)

- [ ] `go test -race ./...` clean with many simulated concurrent clients.
- [ ] No goroutine leak: after every client disconnects (cleanly or via idle timeout) and
      after server shutdown, `runtime.NumGoroutine()` returns to baseline.
- [ ] A client that never reads its socket (buffer fills) cannot block message delivery
      to other clients for more than the bounded deadline.
- [ ] `Accept` unblocks immediately when the listener is closed during shutdown (not
      "next connection attempt").
- [ ] Two clients racing to claim the same nick: exactly one wins; the loser gets
      `ERR nick_taken`; verified with a stress test of N goroutines racing one nick.

### Case specification — SUCCESS (via `net.Dial` against a `chatd` started on `:0`)

| # | Sequence | Assertion |
|---|---|---|
| S1 | connect | first line is `WELCOME chatd/1.0` |
| S2 | `/nick alice` | `OK nick alice` |
| S3 | `/msg` before nick set, on a fresh conn | `ERR need_nick` |
| S4 | alice: `hello room` | alice receives `MSG lobby alice hello room`; a second client bob (also in lobby) receives the same line |
| S5 | alice: `/join dev` | alice: `OK join dev`; bob (still in lobby): no message; a third client carol already in `dev`: `JOINED dev alice` |
| S6 | alice: `/list` | includes `dev(1)` and `lobby(N)` with correct counts |
| S7 | alice: `/who` (in `dev`) | `USERS alice carol` |
| S8 | bob: `/msg alice hi` | bob: `OK msg`; alice: `PRIVATE bob hi` |
| S9 | alice: `/msg ghost hi` | `ERR no_such_user` |
| S10 | bob: `/nick alice` (taken) | `ERR nick_taken` |
| S11 | alice: `/leave` from `dev` | alice: `OK join lobby`; carol (remaining in `dev`): `LEFT dev alice` |
| S12 | alice: `/quit` | `OK bye`, then the connection is closed by the server |
| S13 | idle client, no traffic for the configured idle window (test uses a short `-idle`) | `ERR idle_timeout`, connection closed |
| S14 | SIGINT the server with 2 clients connected | both receive `SERVER shutting_down`; server exits within grace + a small margin |
| S15 | a line of exactly 512 bytes (incl. `\n`) | processed normally |
| S16 | a slow-reading client (test never drains its socket) while others chat | others continue to receive messages promptly; the slow client is eventually disconnected, others unaffected |

### Case specification — FAILURE

| # | Sequence | Assertion |
|---|---|---|
| F1 | `/nick 1bob` (starts with digit) | `ERR invalid_nick` |
| F2 | `/nick alice_is_way_too_long_a_name` (>16 chars) | `ERR invalid_nick` |
| F3 | `/nick alice` then `/nick bob` | `ERR already_named` |
| F4 | `/join` with no room name | `ERR missing_argument` |
| F5 | `/leave` while already in `lobby` | `ERR already_in_lobby` |
| F6 | a 600-byte line | `ERR line_too_long`, connection **stays open** |
| F7 | `/frobnicate` | `ERR unknown_command` |
| F8 | `chatd -addr :99999` | process prints an error and exits `1` |

### Error catalogue

Reply codes are exactly the tokens shown above (`ERR need_nick`, `ERR nick_taken`, …) —
treat them as a fixed vocabulary; document all of them in the README.

## Suggested milestones

1. Protocol parser: one line → a `Command` value (pure function, no I/O — table-tested
   exhaustively against the protocol table before any networking exists).
2. `net.Listen`/`Accept` loop + per-connection goroutine that only reads lines and calls
   the parser — no shared state yet, just echo the parsed command back.
3. The hub: `rooms`, `nicks`, a `chan hubCmd` with an embedded reply channel per command;
   one `for cmd := range hub.in { ... }` loop implementing every protocol rule.
4. Wire connections to the hub for `/nick`, `/join`, `/leave`, broadcast text.
5. `/msg`, `/list`, `/who`, `/quit`.
6. Deadlines: idle read timeout; bounded write for slow clients (non-blocking send with
   a grace `time.After`, then disconnect — Project 19's `trySend` pattern, ported).
7. Graceful shutdown: `signal.NotifyContext`, broadcast + grace period + force-close +
   `WaitGroup`.
8. Concurrency + protocol tests using real `net.Dial` connections against a test server on
   `:0` (`ln.Addr()` gives you the real port).

## Project layout

```
projects/23-chatd/
  cmd/chatd/main.go
  internal/chat/protocol.go   // line -> Command, pure
  internal/chat/hub.go        // the actor: rooms, nicks, command handling
  internal/chat/conn.go       // per-connection goroutine: read loop, deadlines, write
  internal/chat/server.go     // Listen/Accept/shutdown wiring
  internal/chat/*_test.go
  cmd/chatd/main_test.go
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly against a real `net.Dial`ed connection.
- [ ] Every concurrency-requirements checkbox has a passing test.
- [ ] The protocol parser has 100% branch coverage on its own (it's pure and cheap to
      test exhaustively).
- [ ] The hub is the sole owner of `rooms`/`nicks` — grep the codebase: no other file
      touches those maps.
- [ ] `go test -race ./...` clean with ≥20 simulated concurrent clients in the stress test.

## Test requirements

- `TestParseProtocol` — every row of the protocol table as a pure unit test.
- `TestNickClaim`, `TestNickRace` (N goroutines, one nick, exactly one winner).
- `TestJoinLeaveBroadcasts`, `TestPrivateMessage`, `TestListWho`.
- `TestIdleTimeout` (short configured timeout).
- `TestLineTooLong`.
- `TestSlowClientDoesNotBlockOthers`.
- `TestGracefulShutdown`.
- `TestNoGoroutineLeak`.
- `ExampleHub` or a runnable doc example on the protocol parser.

## Stretch goals

- `/nick` changes after joining, broadcasting a `NICK old new` notice.
- TLS (`crypto/tls`, still stdlib) via `-tls-cert`/`-tls-key`.
- A simple flood-control: rate-limit lines per client (reuse Project 14's ideas).
- Message history: `/join ROOM` replays the last 20 messages (bridge to Project 19's
  `WithReplay`).
- A companion TCP **client** binary (`chatc`) with a simple terminal UI.

## Self-check questions

1. Why does the hub own `rooms`/`nicks` instead of each connection goroutine locking a
   shared `sync.Mutex`-guarded map directly? What does the channel-based design make
   trivially true that you'd have to prove by hand with a mutex (hint: think about
   `/join` needing to atomically leave one room *and* join another)?
2. `conn.SetReadDeadline(time.Now().Add(idleTimeout))` — where exactly do you call this
   (once at connect, or every time you successfully read a line)? What happens if you only
   call it once?
3. A slow client's TCP send buffer is full. What does a blocking `conn.Write` do, and how
   does that, left unguarded, let one bad client freeze the whole server (trace the actual
   call chain from the hub's broadcast to that `Write`)?
4. `ln.Close()` during shutdown — what error does a blocked `Accept()` return, and how do
   you distinguish "listener closed on purpose" from a real accept error in your loop?
5. Two clients send `/nick alice` at nearly the same instant. Where, precisely (which
   goroutine, which line of code), does the race get resolved, and why is that the only
   place it *can* be resolved correctly?
6. `bufio.Scanner`'s default max token size will silently truncate or error on a line
   over ~64KB. Your spec caps lines at 512 bytes — how do you enforce *your* limit
   explicitly rather than relying on the scanner's default, and what do you do differently
   for a line that's long but not "reading forever" (no newline ever arrives)?
7. Why must the write to a slow client use the same "nil-channel via zero timeout"
   `select` trick as Project 19's `trySend`, rather than a plain blocking `conn.Write`
   inside the hub's own goroutine? What would blocking the hub goroutine even briefly cost
   every other client?
