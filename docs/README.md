# Go Mastery Curriculum

A sequence of **30 build-ready projects** that take you from Go fundamentals to a
tree-walking language interpreter. Each project is a self-contained brief: full
requirements, an exact contract, an exhaustive list of every success and failure case,
and a checkable "Definition of Done". You are never asked to design the project — only to
write the Go.

The projects are ordered **easy → hard** and are engineered so that, by the time you
finish all of them, you have been *forced* to use every core language feature, every
concurrency primitive, and every essential standard-library package. See
[concept-coverage.md](concept-coverage.md) for the full traceability matrix (concept →
which project exercises it).

**Standard library only.** No third-party modules anywhere in this curriculum. When a
concept is deliberately out of scope (e.g. `database/sql`), the coverage doc says so and
points you at what to study next.

---

## How to use this

1. Do the projects **in order**. Later briefs assume you own everything earlier ones
   taught and will not re-explain it.
2. Read the whole brief before writing code. Then work milestone by milestone — each
   milestone is independently runnable and worth a commit.
3. A project is done when **every row of its Case-specification tables reproduces
   exactly** and its Definition of Done checklist is fully ticked. Don't move on early.
4. Answer the **Self-check questions** out loud or in a `NOTES.md`. If you can't, you
   built it without understanding it — go back.
5. Do the **Stretch goals** only after the core is done. They are the one place the brief
   says "design it yourself".

---

## Repository layout

One module (`learning/go`), one directory per project:

```
go.mod                     module learning/go
go.work                    (added at Project 27, to wire in the submodule)
projects/
  01-unit-converter/
    cmd/convert/main.go    entry point(s) — thin: flags, I/O, exit codes
    internal/units/...     the real logic, as importable packages
    internal/units/*_test.go
    README.md              what it is, how to run it, design notes
    Makefile
  02-bit-toolkit/
    ...
  27-ratelimit/            has its OWN go.mod (a submodule) — see that brief
```

Rules:

- **Binaries** live in `projects/NN-slug/cmd/<name>/` and stay thin. All logic goes in
  packages so it can be tested without spawning a process.
- **Libraries** (Projects 11, 27) have no `cmd/` — just packages plus `_test.go`.
- Use `internal/` for packages that must not be imported by other projects.
- Every project has its own `README.md` and `Makefile`.
- Import paths look like `learning/go/projects/01-unit-converter/internal/units`.

Create a project directory with:

```
mkdir -p projects/01-unit-converter/cmd/convert
```

No `go.mod` changes are needed until Project 27 — everything is under the root module.

---

## Toolchain cheatsheet

| Command | Use |
|---|---|
| `go run ./projects/01-unit-converter/cmd/convert 100 km mi` | run a binary |
| `go build -o bin/ ./projects/01-.../cmd/...` | build |
| `go test ./projects/01-unit-converter/...` | run that project's tests |
| `go test -race ./...` | tests with the race detector (required for every concurrent project) |
| `go test -run TestName -v ./...` | one test, verbose |
| `go test -bench . -benchmem ./...` | benchmarks + allocation stats |
| `go test -fuzz FuzzName ./...` | run a fuzz target (Ctrl-C to stop) |
| `go test -cover ./...` | coverage summary |
| `go test -coverprofile=c.out ./... && go tool cover -html=c.out` | coverage in the browser |
| `gofmt -l .` | list unformatted files (must be empty) |
| `go vet ./...` | static checks (must be clean) |
| `go doc ./projects/27-ratelimit` | render a package's docs |
| `go generate ./...` | run `//go:generate` directives |
| `go tool pprof cpu.prof` | inspect a profile |

Recommended per-project `Makefile`:

```make
.PHONY: build test race vet fmt cover clean
build:  ; go build ./...
test:   ; go test ./...
race:   ; go test -race ./...
vet:    ; go vet ./...
fmt:    ; gofmt -l .
cover:  ; go test -coverprofile=cover.out ./... && go tool cover -func=cover.out
clean:  ; rm -f cover.out *.prof
```

---

## Universal Definition of Done

**Every** project must satisfy all of this, on top of its own checklist:

- [ ] `go build ./...` succeeds from the repo root.
- [ ] `gofmt -l .` prints nothing (code is formatted).
- [ ] `go vet ./...` is clean.
- [ ] `go test ./...` passes. For any project that starts a goroutine,
      `go test -race ./...` also passes.
- [ ] Tests are **table-driven** where there is more than one case, and every row of the
      brief's Case-specification tables has a corresponding assertion.
- [ ] Every **exported** identifier (type, func, const, var, method) has a doc comment
      that starts with its name.
- [ ] Binaries are thin: no business logic in `main`, in `func main()` or in an
      `http.HandlerFunc` literal — it lives in a testable package.
- [ ] Errors are **handled or returned**, never ignored with `_`. A deliberate ignore has
      a one-line comment saying why.
- [ ] No naked `panic` in library code for expected conditions — return an `error`.
      `panic` is only for programmer bugs (and the two projects that explicitly practise
      `recover`).
- [ ] The project's `README.md` explains what it is, how to run it, and one paragraph of
      design notes (why the packages are split the way they are).
- [ ] From Project 11 onward: at least one **benchmark** exists for the core operation.
- [ ] Where the brief says so: a **fuzz** target exists and has run clean for ≥30s.

---

## The 30 projects

| # | Project | Difficulty | Headline concepts |
|---|---------|:---:|---|
| 01 | Unit Converter CLI | 1 | constants, `iota`, `switch`, `flag`, `strconv` |
| 02 | Number Bases & Bit Toolkit | 1 | bitwise ops, integer types, overflow, `fmt` verbs |
| 03 | Date & Duration Calculator | 1 | `time` (layouts, durations, zones) |
| 04 | Word Frequency Counter | 2 | slices, maps, runes, `bufio`, `sort`/`slices`/`maps` |
| 05 | Grade Book CSV Report | 2 | structs, methods, receivers, embedding, `encoding/csv` |
| 06 | Matrix / Vector Mini-Library | 2 | slice internals, `panic`/`recover`, labeled loops |
| 07 | Todo CLI (JSON store) | 2 | `encoding/json`, `defer`, `flag.Value`, atomic file writes |
| 08 | Expression Calculator | 3 | recursion, interface ASTs, error wrapping |
| 09 | `mini-grep` line filter | 3 | `io.Reader`/`Writer`, `bufio`, `regexp`, streaming |
| 10 | Binary File Format Parser | 3 | `encoding/binary`, `io.SectionReader`, `crc32`, fuzzing |
| 11 | Generic Containers Library | 3 | generics, constraints, iterators, `container/heap` |
| 12 | JSON Config Loader + Validator | 3 | custom (un)marshal, streaming JSON, `errors.Join`, `reflect` |
| 13 | Report / Plugin Engine | 3 | interface composition, type switches, `fmt.Formatter` |
| 14 | Concurrent Link Checker | 3 | goroutines, channels, worker pool, `context` |
| 15 | Log Aggregation Pipeline | 4 | pipelines, `select`, `signal.NotifyContext`, `atomic` |
| 16 | Bounded Blocking Queue | 4 | `sync.Cond`, channel vs condvar |
| 17 | In-Memory Cache (TTL + single-flight) | 4 | `RWMutex`, `Once`, timers, generics, benchmarks |
| 18 | Concurrent Download Manager | 4 | ranged HTTP, `io` plumbing, `sync.Pool`, resume |
| 19 | In-Process Pub/Sub Broker | 4 | nil channels, backpressure, graceful drain |
| 20 | HTTP REST API (stdlib only) | 4 | `net/http`, middleware, `log/slog`, `httptest` |
| 21 | Auth & Sessions | 4 | `crypto/*`, HMAC cookies, CSRF, PBKDF2 |
| 22 | File-Backed Storage Engine | 5 | WAL, snapshots, crash recovery, `gob`, fuzzing |
| 23 | TCP Chat Server | 4 | `net`, broadcast hub, deadlines, graceful shutdown |
| 24 | HTTP/1.1 Server From Scratch | 5 | raw TCP, hand-parsed HTTP, `html/template` |
| 25 | TCP Key-Value Store (mini-Redis) | 5 | wire protocol, persistence, client package |
| 26 | Reverse Proxy + Load Balancer | 5 | `httputil.ReverseProxy`, health checks, hot reload |
| 27 | Rate Limiter Package (own module) | 5 | modules, semver, `go.work`, `go doc`, fuzzing |
| 28 | Static Site Generator CLI | 5 | `io/fs`, templates, `go:embed`, build tags |
| 29 | Task Scheduler / Cron Daemon | 5 | cron parsing, `os/exec`, plugins, `pprof` |
| 30 | Mini Language Interpreter | 5 | lexer, Pratt parser, evaluator, closures, fuzzing |

Tiers: **1–7** fundamentals · **8–13** interfaces/errors/generics/I-O · **14–19**
concurrency · **20–25** networking & servers · **26–30** advanced / capstones.
