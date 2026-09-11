# Concept Coverage Matrix

This is the audit trail for "cover all of Go". Every row names the project(s) whose **Go
concepts you MUST use** checklist forces that concept. If you finish all 30 projects and
tick every checklist, you have used everything here.

Legend: a bare number is the primary project for that concept; `;` separates primary from
secondary appearances.

---

## 1. Declarations, types, operators

| Concept | Projects |
|---|---|
| Packages, imports, exported vs unexported names | all |
| `init()` functions | 01, 13; 07 |
| `var` / `:=` / multiple assignment / `var` blocks | all; 01–04 |
| Variable shadowing (and how `:=` causes it) | 04; 08 |
| Untyped vs typed constants | 01, 02 |
| `const` blocks, `iota`, `iota` in expressions, bit-flag `iota` | 01, 02 |
| Numeric types (`int`, `int8..64`, `uint*`, `uintptr`), conversions | 02, 06 |
| Integer overflow / wraparound; two's complement | 02 |
| Signed vs unsigned arithmetic and shifts | 02 |
| Floating point, `math` constants, `NaN`/`Inf`, rounding | 01, 02 |
| `bool`, comparison & logical operators, short-circuit eval | all; 01 |
| Bitwise operators `& | ^ &^ << >>` | 02 |
| `string`, `[]byte`, `[]rune`, conversions between them | 04, 09 |
| `rune` / code points; `unicode`, `unicode/utf8` | 04; 30 |
| Raw vs interpreted string literals; string immutability | 04, 08 |
| Zero values; relying on them | 01, 06, 07 |
| `new` vs composite literals vs `&T{}` | 06 |
| Builtins: `len`, `cap`, `make`, `append`, `copy`, `delete`, `clear`, `min`, `max` | 04, 06 |
| Blank identifier `_` | 01, 12 |
| `iota`-based `Stringer` enums | 02, 13 |

## 2. Control flow & functions

| Concept | Projects |
|---|---|
| `if` / `if` with init statement | all |
| `for` — all four forms (C-style, while, infinite, `range`) | all; 04 |
| `range` over slice / map / string / channel | 04, 14 |
| `range` over integer (Go 1.22) and over function (Go 1.23) | 11 |
| `switch` — expression form | 01 |
| `switch` — tagless / no-condition form | 01 |
| `switch` — type switch | 13; 08 |
| `switch` — `fallthrough` | 05 |
| Labeled `break` / `continue` | 06 |
| `goto` | 06 |
| Functions: parameters, multiple returns, named returns, naked `return` | 01, 08 |
| Variadic functions and `...` spread | 04; 13 |
| Closures; capturing variables; the loop-variable question (pre/post Go 1.22) | 08; 14 |
| First-class functions, function types, higher-order functions | 08, 30; 13 |
| Method values and method expressions | 05 |
| Recursion (and when to prefer iteration) | 08, 30 |
| `defer` — ordering, argument evaluation time, `defer` in loops | 07; 08 |
| `panic` / `recover` — at a boundary, in a goroutine | 08, 20; 06 |
| The functional-options pattern | 12, 27 |

## 3. Structs, methods, interfaces, generics

| Concept | Projects |
|---|---|
| Struct types, field access, comparability, tags | 05, 12 |
| Anonymous structs | 05; 20 (test tables) |
| Struct embedding and field/method promotion | 05 |
| Value vs pointer receivers — and how to choose | 05, 06 |
| Pointer semantics; aliasing; nil pointer dereference | 06 |
| Interfaces; satisfying an interface implicitly | 08, 13 |
| Interface composition (embedding interfaces) | 13; 09 (`io`) |
| The empty interface / `any` | 13 |
| Type assertions (`x.(T)`), comma-ok form | 13 |
| Type switches | 13; 08 |
| `fmt.Stringer` | 02, 05, 13 |
| `fmt.Formatter` (custom `%v` / `%+v`) | 13 |
| `error` as an interface; implementing it | 08 |
| Nil-interface vs nil-pointer-in-interface gotcha | 13 (self-check) |
| Generics: type parameters, instantiation, inference | 11, 27; 17 |
| Constraints, `constraints`-style interfaces, type sets, `~` | 11, 27 |
| `comparable`, `cmp.Ordered` | 11 |
| Generic methods / generic types with methods | 11 |
| Iterators: `iter.Seq`, `iter.Seq2`, range-over-func | 11 |
| `reflect`: `TypeOf` / `ValueOf` / `Kind`, reading struct tags, `DeepEqual`, setting fields | 12 |

## 4. Slices, maps, arrays, ordering

| Concept | Projects |
|---|---|
| Array types; arrays as values; array vs slice | 06 |
| Slice headers: pointer/len/cap; `append` growth; reallocation | 04, 06 |
| Slice aliasing bugs; full three-index slice `a[i:j:k]` | 06 |
| `copy` semantics | 06 |
| Multi-dimensional slices | 06 |
| Maps: creation, comma-ok read, `delete`, non-deterministic iteration | 04, 17 |
| The `map[T]struct{}` set idiom | 04, 13 |
| `sort.Slice`, `sort.Interface`, `sort.Stable` | 04, 05, 13 |
| `slices` package (`Sort`, `SortFunc`, `Contains`, `BinarySearch`, `Insert`, `Delete`, `Clone`, `Equal`, `Compact`) | 04, 06, 11 |
| `maps` package (`Clone`, `Equal`, `Keys`, `Values` iterators) | 04, 17 |
| `cmp` package (`Compare`, `Or`, `Ordered`) | 04, 11 |
| `container/heap` | 11 |
| `container/list` | 11 |
| `container/ring` (optional / stretch) | 11 (stretch) |

## 5. Errors

| Concept | Projects |
|---|---|
| The `error` interface; returning errors; `errors.New` | 01, 07 |
| Sentinel errors and `errors.Is` | 07 |
| Custom error types and `errors.As` | 08 |
| Wrapping with `fmt.Errorf("...: %w", err)`; `errors.Unwrap` | 08 |
| `errors.Join` / multi-error aggregation | 12, 18 |
| Deciding: sentinel vs typed vs opaque errors | 08 (self-check) |
| `panic`/`recover` as a controlled boundary mechanism | 08, 20 |

## 6. Concurrency

| Concept | Projects |
|---|---|
| Goroutines; the `go` statement; goroutine lifetime | 14 |
| Goroutine leaks — causing, detecting, preventing | 15, 19 |
| Unbuffered vs buffered channels; blocking semantics | 14, 16 |
| Directional channel types (`chan<-`, `<-chan`) | 14 |
| `close`, receiving from a closed channel, `range` over a channel | 14, 19 |
| Nil channels and disabling a `select` case | 19 |
| `select`; `select` with `default`; `select` + `time.After` | 15, 19 |
| The worker-pool pattern | 14 |
| The pipeline pattern (fan-out / fan-in) | 15 |
| The single-flight pattern | 17 |
| Backpressure and drop policies | 19 |
| `sync.Mutex`, `sync.RWMutex` | 17; 23 |
| `sync.WaitGroup` | 14 |
| `sync.Once` | 17 |
| `sync.Cond` | 16 |
| `sync.Pool` | 18 |
| `sync.Map` — and why a plain map+mutex is usually better | 17, 19 |
| `sync/atomic` (counters, `atomic.Value`, `atomic.Pointer`) | 15, 17 |
| `context`: `WithCancel`, `WithTimeout`, `WithDeadline` | 14 |
| `context`: `WithValue` and context keys | 14, 20 |
| `context` cancellation propagation through a call tree | 15 |
| `signal.NotifyContext` / `os/signal` graceful shutdown | 15, 23 |
| The race detector (`-race`) | 14 onward (all concurrent projects) |
| Deadlock and starvation reasoning | 16 |

## 7. Standard library — I/O, text, os

| Package / area | Projects |
|---|---|
| `fmt` — verbs `%v %+v %#v %T %q %b %o %x %c %e %g`, width/precision, `Sprintf`/`Fprintf`/`Errorf` | 02, 05, 13 |
| `strconv` — `Parse*`/`Format*`/`Quote`, base handling | 01, 02 |
| `strings` — search, split, replace, `Builder`, `NewReader`, `NewReplacer` | 04, 09 |
| `bytes` — `Buffer`, `Reader`, byte-slice helpers | 09, 10 |
| `unicode`, `unicode/utf8` — classification, decoding | 04 |
| `io` — `Reader`/`Writer`/`Closer`/`Seeker`, `Copy`/`CopyN`, `Pipe`, `MultiWriter`/`MultiReader`, `LimitReader`, `SectionReader`, `ReaderAt`/`WriterAt`, `Discard`, `EOF` | 09, 10, 18 |
| `bufio` — `Scanner` (+ custom `SplitFunc`), `Reader` (`Peek`, `ReadString`), `Writer` (`Flush`) | 04, 09, 24 |
| `os` — `Args`, `Getenv`/`Environ`, `Open`/`Create`/`OpenFile`, `FileMode`, `MkdirAll`, `Rename`, `Stat`, `Exit`, `Stdin`/`Stdout`/`Stderr` | 07, 09 |
| `os/exec` — `CommandContext`, `StdinPipe`/`StdoutPipe`, `ExitError`, `Run`/`Start`/`Wait` | 29; 28 |
| `os/signal`, `signal.NotifyContext` | 15, 23 |
| `flag` — `String`/`Int`/`Bool`/`Var`, custom `flag.Value`, `Usage`, `Args`, subcommands via `flag.NewFlagSet` | 01, 07, 09 |
| `time` — reference layout, `Parse`/`Format`/`ParseDuration`, `Duration` math, `Add`/`AddDate`/`Sub`, `Truncate`/`Round`, `Weekday`, `Location`/`LoadLocation`, `Timer`/`Ticker`/`AfterFunc`/`After`, `Since`/`Until`, monotonic clock | 03; 17, 29 |
| `math`, `math/rand/v2` | 02; 14, 17 |
| `path`, `path/filepath` — `Join`, `Ext`, `Base`, `Dir`, `Walk`/`WalkDir`, `Rel`, `Glob` | 28 |
| `io/fs`, `embed` — `fs.FS`, `fs.WalkDir`, `embed.FS`, `//go:embed` | 28; 25 |
| `log` vs `log/slog` — handlers (`TextHandler`/`JSONHandler`), levels, `With`, `Group`, attrs | 20; 29 |
| `regexp` — compile, groups, named captures, `FindAllStringSubmatch`, `ReplaceAllString(Func)` | 09 |
| `text/tabwriter` | 05 |

## 8. Standard library — encoding, hashing, crypto

| Package | Projects |
|---|---|
| `encoding/json` — struct tags, `omitempty`, `Marshal`/`Unmarshal`, `Encoder`/`Decoder` (streaming), `json.RawMessage`, custom `MarshalJSON`/`UnmarshalJSON`, `json.Number` | 07, 12, 20 |
| `encoding/csv` | 05 |
| `encoding/xml` | 12; 28 (RSS) |
| `encoding/gob` | 22 |
| `encoding/binary` — `ByteOrder`, `Read`/`Write`, `varint`, `Uvarint` | 10; 22 |
| `encoding/hex`, `encoding/base64` | 10; 21 |
| `hash`, `hash/crc32`, `hash/fnv` | 10 |
| `crypto/rand` | 21; (already in your scratch `main.go`) |
| `crypto/sha256`, `crypto/hmac`, `crypto/subtle` | 21 |
| `crypto/pbkdf2` (stdlib since Go 1.24) for password hashing | 21 |

## 9. Standard library — networking & web

| Package / area | Projects |
|---|---|
| `net/http` server — `Handler`/`HandlerFunc`, `ServeMux` method+path patterns, `http.Server` with timeouts, `Server.Shutdown` | 20 |
| `net/http` middleware — wrapping handlers, a recovery middleware, request-scoped context | 20 |
| `net/http` client — `http.Client` with timeout, `Request`/`Response`, `Range` requests, connection reuse | 14, 18 |
| `net/url` — `Parse`, `URL` fields, `Values`, query encoding | 14, 26 |
| `net` — `Listen`/`Dial`, `net.Conn`, `TCPConn`, `SetDeadline`/`SetReadDeadline` | 23, 24, 25 |
| `net/textproto` — reading MIME-style headers (optional in 24) | 24 |
| `net/http/httputil` — `ReverseProxy`, `DumpRequest` | 26 |
| `net/http/httptest` — `Server`, `ResponseRecorder`, `NewRequest` | 20 |
| `net/http/pprof` — registering the profiling endpoints | 29 |
| `mime`, `mime/multipart` — file uploads, `multipart.Reader`/`Writer` | 20 |
| `html/template` — auto-escaping, `{{define}}`/`{{template}}`/`{{block}}`, `FuncMap`, `ParseFS` | 24, 28 |
| `text/template` — the same engine without escaping; when each is correct | 28 |
| `expvar` — published metrics | 26, 29 |

## 10. Testing, tooling, modules, build

| Concept | Projects |
|---|---|
| `testing` basics, `t.Errorf`/`t.Fatalf` | all |
| Table-driven tests | all (from 01) |
| Subtests, `t.Run`, subtest naming | 11; all from 11 |
| `t.Parallel` | 17 |
| `t.Cleanup` | 22 |
| `t.Helper` | 11 |
| `TestMain` / `testing.M` (suite setup/teardown) | 20 |
| Golden files / `testdata/` directories | 22; 24 |
| `httptest`-based handler tests | 20 |
| Benchmarks — `testing.B`, `b.N`, `b.ReportAllocs`, `b.ResetTimer`, sub-benchmarks | 11 onward |
| Fuzzing — `testing.F`, `f.Add`, seed corpus, `go test -fuzz` | 10, 22, 27, 30 |
| `Example` functions / testable examples; output comments | 27 |
| Doc comments, package doc (`doc.go`), `go doc` | 27; all (exported symbols) |
| Coverage — `-cover`, `-coverprofile`, `go tool cover` | README + spot use |
| Profiling — `-cpuprofile`/`-memprofile`, `runtime/pprof`, `net/http/pprof`, `go tool pprof` | 29, 30 |
| `runtime` — `NumGoroutine`, `GOMAXPROCS`, `ReadMemStats`, `Gosched` | 29 |
| Modules — `go.mod`, `go.sum`, `require`, `replace`, `exclude`, semantic import versioning | 27 |
| Multi-module workspaces — `go.work`, `go work use` | 27 |
| Semver tags, `v0`/`v1` rules, `/v2` import paths (concept) | 27 |
| Package design — `cmd/`, `internal/`, minimal public API surface | 20, 25, 27 |
| Build constraints — `//go:build` lines, `_GOOS`/`_GOARCH` filename suffixes | 28, 29 |
| `//go:generate` | 11, 30 |
| `//go:embed` | 25, 28 |
| `go vet`, `gofmt` / `go fmt`, `go fix` (awareness) | README + all |

---

## Deliberately NOT covered — and what to do next

| Left out | Why | Study next |
|---|---|---|
| `database/sql` + a SQL driver | Every driver (`mattn/go-sqlite3`, `modernc.org/sqlite`, `lib/pq`, `jackc/pgx`) is a third-party module; this curriculum is stdlib-only. Project 22 teaches persistence one level down (write-ahead log, snapshots, crash recovery), which is the mental model `database/sql` sits on top of. | After Project 22, re-implement Project 20's store on `database/sql` + `modernc.org/sqlite` (pure Go, no cgo). Learn `sql.DB` pooling, `Tx`, `context` methods, `sql.Null*`, prepared statements. |
| `cgo` | Rarely needed, big footgun, not "learning Go" so much as "learning the FFI boundary". | Read the cgo docs; try wrapping one small C function once. |
| `unsafe`, `reflect` field-offset tricks | Almost never correct in application code. | Read the `unsafe` docs and the strict rules for `unsafe.Pointer` conversions. |
| Go assembly, `//go:linkname`, `plugin` | Niche / discouraged. | Skim the Go asm guide for curiosity only. |
| Third-party ecosystem — `chi`/`gin`, `sqlx`/`gorm`, `cobra`/`urfave-cli`, `testify`, `zap`/`zerolog`, `golang.org/x/sync/errgroup`, `golang.org/x/time/rate` | You will appreciate them far more having built the stdlib versions yourself (Projects 20, 7/1, 20, all tests, 20/29, 15, 27). | Re-skin Project 20 with `chi` + `sqlx`, Project 1/7 with `cobra`, and compare your Project 27 to `golang.org/x/time/rate`. |
| Generics-heavy functional libraries (`samber/lo` etc.) | The language gives you enough; over-abstraction is a common beginner trap. | — |
