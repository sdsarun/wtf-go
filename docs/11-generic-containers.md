# Project 11 — Generic Containers Library

| | |
|---|---|
| **Difficulty** | 3 / 5 |
| **Estimated time** | 8–12 hours |
| **Prerequisites** | [04](04-word-frequency-counter.md), [06](06-matrix-vector-library.md) |
| **Builds toward** | [17 – Cache](17-in-memory-cache.md), [27 – Rate Limiter](27-rate-limiter.md), [30 – Interpreter](30-interpreter.md) |

## Why this project

Generics done properly: type parameters, constraints and **type sets** (`~`), `comparable`
vs `cmp.Ordered`, methods on generic types, type inference at call sites, and the Go 1.23
**range-over-func** iterators (`iter.Seq`, `iter.Seq2`). You'll build the data structures
you keep re-writing, once, correctly, with benchmarks. From this project on, **every
project needs at least one benchmark.**

## Go concepts you MUST use

- [ ] type parameters on types and functions; constraints `any`, `comparable`,
      `cmp.Ordered`; a custom constraint with a **type set** using `~`
      (`type Number interface { ~int | ~int64 | ~float64 | … }`)
- [ ] methods on generic types (`func (s *Stack[T]) Push(v T)`)
- [ ] type **inference** — constructors and helpers callable without explicit type args
- [ ] `iter.Seq[T]` and `iter.Seq2[K, V]`; consuming them with `for x := range it`;
      producing them with `func(yield func(T) bool)`
- [ ] `slices.Collect`, `slices.Sorted`, `maps.Collect`, `slices.SortFunc`
- [ ] `container/heap` — implement `heap.Interface` on an unexported adapter
- [ ] `container/list` — use it once (in `List` internals or a benchmark baseline) and
      write down in the README why a generic wrapper beats it
- [ ] `//go:generate` — a local generator produces the big `Number` constraint file
- [ ] `t.Helper()` in a shared test assertion helper
- [ ] benchmarks: `b.ReportAllocs()`, `b.ResetTimer()`, sub-benchmarks with `b.Run`

## Requirements

### Types and APIs

All live under `internal/containers`. All have: `Len() int`, an `All()` iterator, a
`String()` (for small ones), and a zero value that is usable (or a documented "must use
the constructor").

**`Stack[T any]`** — LIFO, slice-backed.
`New[T]() *Stack[T]` · `Push(v T)` · `Pop() (T, bool)` · `Peek() (T, bool)` ·
`All() iter.Seq[T]` (top → bottom).

**`Queue[T any]`** — FIFO, **ring-buffer** backed (amortized O(1), no per-op alloc after
warm-up).
`Enqueue(v T)` · `Dequeue() (T, bool)` · `Peek() (T, bool)` · `All() iter.Seq[T]` (front → back).

**`Deque[T any]`** — ring buffer, both ends.
`PushFront/PushBack/PopFront/PopBack/Front/Back` · `At(i int) T` (panics OOB) · `All()`.

**`RingBuffer[T any]`** — fixed capacity `n`; `Push` overwrites the oldest when full.
`NewRing[T](n int) *RingBuffer[T]` · `Push(v T) (evicted T, didEvict bool)` · `All()` (oldest → newest).

**`List[T any]`** — doubly linked.
`PushFront/PushBack` return `*Node[T]` · `Remove(*Node[T])` · `MoveToFront(*Node[T])` ·
`Front()/Back() *Node[T]` · `All()`. (This one powers Project 17's LRU.)

**`Set[T comparable]`** — hash set.
`Add/Remove/Contains` · `Union/Intersect/Difference(other) *Set[T]` (new set) ·
`IsSubsetOf(other) bool` · `All() iter.Seq[T]` (unspecified order) ·
`Collect(iter.Seq[T]) *Set[T]`.

**`SortedSet[T cmp.Ordered]`** — ordered, backed by a sorted slice (binary search insert)
or a simple BST — your choice, document the complexity.
`Add/Remove/Contains` · `Min()/Max() (T, bool)` · `Range(lo, hi T) iter.Seq[T]` (inclusive) ·
`All()` (ascending).

**`OrderedMap[K comparable, V any]`** — map + insertion-order list.
`Set(k, v)` · `Get(k) (V, bool)` · `Delete(k) bool` · `Keys() iter.Seq[K]` (insertion order) ·
`All() iter.Seq2[K, V]` (insertion order). Re-`Set`ting an existing key updates the value
but **keeps** its original position.

**`PriorityQueue[T any]`** — min-heap by a supplied `less func(a, b T) bool`.
`NewPQ[T](less func(a, b T) bool) *PriorityQueue[T]` · `Push(v T)` · `Pop() (T, bool)` ·
`Peek() (T, bool)` · `Len()`. Implemented on top of `container/heap`.

**Free functions** (generic, in `containers`):
`Map[T, U any](iter.Seq[T], func(T) U) iter.Seq[U]` ·
`Filter[T any](iter.Seq[T], func(T) bool) iter.Seq[T]` ·
`Reduce[T, A any](iter.Seq[T], A, func(A, T) A) A` ·
`Chunk[T any](iter.Seq[T], n int) iter.Seq[[]T]`.
`Map`/`Filter`/`Chunk` are **lazy** (they return iterators, pull nothing until ranged).

### Behaviour specification (each row is a test)

| Structure | Scenario | Expectation |
|---|---|---|
| `Stack` | push 1,2,3; pop | `3, true` then `2, true` then `1, true` then `_, false` |
| `Stack` | `All()` after push 1,2,3 | yields `3, 2, 1` |
| `Stack` | zero value `var s Stack[int]; s.Push(1)` | works (no constructor needed) — or document it panics; be consistent |
| `Queue` | enqueue 1..5, dequeue 3, enqueue 6,7 | dequeue yields `4,5,6,7` |
| `Queue` | enqueue/dequeue 1e6 times alternating | **zero allocations per op** after warm-up (`BenchmarkQueueSteadyState`) |
| `Deque` | pushFront 1, pushBack 2, pushFront 3 | `All()` → `3,1,2`; `At(0)==3` |
| `Deque` | `At(99)` on len 3 | panics `deque: index 99 out of range [0,3)` |
| `RingBuffer` | cap 3; push 1,2,3,4 | push(4) returns `(1, true)`; `All()` → `2,3,4` |
| `List` | pushBack a,b,c; remove(b) | `All()` → `a,c`; removing again is a no-op-safe? document (Remove twice is a bug → may panic) |
| `List` | `MoveToFront(node_c)` | `All()` → `c,a` |
| `Set` | `{1,2,3}.Intersect({2,3,4})` | `{2,3}`; originals unchanged |
| `Set` | `{1,2}.IsSubsetOf({1,2,3})` | `true`; `{1,4}` → `false` |
| `SortedSet` | add 5,1,3,2,4 | `All()` → `1,2,3,4,5`; `Min()` → `1,true` |
| `SortedSet` | `Range(2, 4)` | yields `2,3,4` |
| `OrderedMap` | set a,b,c then re-set a | `Keys()` → `a,b,c` (a keeps position); `Get(a)` → new value |
| `OrderedMap` | delete b | `All()` → `(a,_),(c,_)` |
| `PriorityQueue` | push 5,1,3 with `less = <` | pop → `1,3,5` |
| `PriorityQueue` | push structs, `less` by field | min-by-field ordering |
| `Map`/`Filter` | `Filter(Map(seq(1..10), sq), even)` | lazy: pull 3 → only 3 `sq` calls (`TestLaziness` with a counter) |
| `Reduce` | `Reduce(seq(1..5), 0, +)` | `15` |
| `Chunk` | `Chunk(seq(1..7), 3)` | `[1,2,3] [4,5,6] [7]` |
| iterators | `break` out of `range s.All()` early | iterator's `yield` returns `false`; no goroutine leaked, no panic |

### Error / panic policy

- Removal from an empty structure via the `(T, bool)` API → `zero, false`, never panic.
- Index-based access (`Deque.At`) out of range → **panic** with the documented string
  (it's a caller bug).
- `NewRing(0)` or negative → **panic** `ring: capacity must be >= 1`.
- `NewPQ(nil)` → **panic** `pq: less func must not be nil`.

## `//go:generate`

`internal/containers/number.go` starts with:

```go
//go:generate go run ./internal/gen -out number_gen.go
```

`internal/gen/main.go` writes `number_gen.go` containing
`type Number interface { ~int | ~int8 | … | ~float64 }` and a `type Real`, `type Integer`,
etc. Commit both the generator and its output; `go generate ./...` must reproduce
`number_gen.go` byte-for-byte (add a `TestGeneratedUpToDate` that runs the generator to a
buffer and diffs).

## Suggested milestones

1. `assert` test helper (`t.Helper()`), then `Stack`, `Queue` (ring), their iterators.
2. `Deque`, `RingBuffer`.
3. `Set`, `SortedSet`.
4. `OrderedMap` (map + intrusive linked list of entries).
5. `List` + `container/list` benchmark baseline.
6. `PriorityQueue` on `container/heap`.
7. `Map/Filter/Reduce/Chunk` lazy iterator combinators + `TestLaziness`.
8. `//go:generate` for `Number`.
9. Benchmarks: `b.Run` sub-benchmarks per structure; `ReportAllocs`.

## Project layout

```
projects/11-containers/
  internal/containers/stack.go queue.go deque.go ring.go list.go
  internal/containers/set.go sortedset.go orderedmap.go pq.go
  internal/containers/iter.go        // Map/Filter/Reduce/Chunk
  internal/containers/number.go      // //go:generate directive + hand parts
  internal/containers/number_gen.go  // generated
  internal/gen/main.go               // the generator
  internal/containers/*_test.go
  internal/containers/bench_test.go
  README.md
  Makefile
```

No `cmd/` — this is a pure library. (Add a `cmd/demo` only if you want a runnable
showcase.)

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every behaviour-table row has a test.
- [ ] `go generate ./...` reproduces `number_gen.go` exactly (`TestGeneratedUpToDate`).
- [ ] `BenchmarkQueueSteadyState` and `BenchmarkStackPushPop` report **0 allocs/op** in
      steady state.
- [ ] `TestLaziness` proves `Map`/`Filter` do no work until pulled and stop early on
      `break`.
- [ ] `TestIteratorEarlyBreak` — breaking out of every `All()` loop is safe (no panic,
      `yield` observed returning false).
- [ ] Every exported generic type/function has a doc comment stating its complexity
      (O(1) amortized, O(log n), etc.).
- [ ] `go vet ./...` clean; the package builds with `GOEXPERIMENT` unset (no bleeding-edge
      features beyond 1.23 iterators).

## Test requirements

- One `Test<Type>` per structure covering its table rows, plus empty-structure and
  single-element edge cases.
- `TestSetAlgebra` — union/intersect/difference/subset with de Morgan sanity checks.
- `TestOrderedMapOrdering` — insertion order survives update and delete.
- `TestPriorityQueueStress` — push 10k random, pop all, assert sorted.
- `TestIterCombinators` — `Map`, `Filter`, `Reduce`, `Chunk`, composition, laziness.
- `TestGeneratedUpToDate`.
- `bench_test.go` — `BenchmarkQueueSteadyState`, `BenchmarkStackPushPop`,
  `BenchmarkListVsContainerList`, `BenchmarkSetContains`,
  `BenchmarkPriorityQueuePushPop`, each with `b.Run` size variants (`1e2`, `1e4`, `1e6`).
- `ExampleOrderedMap`, `ExampleFilter`.

## Stretch goals

- `container/ring`-backed `RingBuffer` variant; benchmark against your slice version.
- A generic `BTreeMap[K cmp.Ordered, V any]` (order 32) — real O(log n), cache-friendly.
- `Seq2` combinators (`MapValues`, `FilterKeys`).
- A `Pool[T any]` thin wrapper over `sync.Pool` (typed `Get`/`Put`) — you'll want it in
  Project 18.
- `TrySend`-style bounded channel adapter for `Queue` — bridge to Project 16/19.

## Self-check questions

1. `type Number interface { ~int | ~float64 }` — why the `~`? What breaks for a caller who
   has `type Celsius float64` if you write `int | float64` instead?
2. `comparable` vs `cmp.Ordered` — `Set[T]` needs one, `SortedSet[T]` the other. Which,
   and why can't `Set` use `cmp.Ordered` and `SortedSet` use `comparable`?
3. Your `Queue.All()` returns `iter.Seq[T]`, which is `func(yield func(T) bool)`. When the
   caller does `for x := range q.All() { if x == target { break } }`, what value does
   `yield` return on the `break`, and what must your producer do when it sees that?
4. `slices.Collect(q.All())` — walk through how the compiler turns your `func(yield ...)`
   and the `for range` into ordinary calls. Is a goroutine involved? (It's not — why do
   people think it is?)
5. `NewPQ` takes `less func(a, b T) bool` instead of constraining `T` to `cmp.Ordered`.
   Name two concrete `T`s where the closure approach is the only option.
6. `Queue` is a ring buffer. Draw the `head`, `tail`, `len`, `cap` after: enqueue 3,
   dequeue 1, enqueue 2 more, with `cap == 4`. When does it grow, and how do you copy on
   grow when the data wraps?
7. `container/list.List` is not generic — its `Element.Value` is `any`. What are the two
   concrete costs (one runtime, one at the type level) of that vs your `List[T]`?
8. `Map[T, U any](seq, f)` returns an `iter.Seq[U]` immediately, having called `f` zero
   times. Where does `f` actually run, and what keeps it from running during
   `Map(...)` itself?
