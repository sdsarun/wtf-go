# Project 06 — Matrix / Vector Mini-Library

| | |
|---|---|
| **Difficulty** | 2 / 5 |
| **Estimated time** | 5–8 hours |
| **Prerequisites** | [01](01-unit-converter-cli.md)–[05](05-grade-book-csv-report.md) |
| **Builds toward** | [11 – Generic Containers](11-generic-containers.md) |

## Why this project

This is where you learn what a slice *actually is*: a pointer, a length, and a capacity.
You will store a 2-D matrix in one flat `[]float64`, which forces you to do the index math
by hand and to confront aliasing head-on (a "row view" that writes through vs a "row copy"
that doesn't). You will also use `panic`/`recover` correctly — panic for a programmer bug
(bad index), `recover` at a boundary to turn a low-level panic into a returned `error`.

It is a **library** (plus a tiny demo binary). Most of the work is in the tests.

## Go concepts you MUST use

- [ ] slice internals: `len` vs `cap`, `append` reallocation, `copy`, and a **full
      three-index slice** `s[i:j:k]` used to make a row view safe against `append`
- [ ] slice **aliasing**: one method returns a view that writes through, another returns a
      copy — with tests proving the difference
- [ ] arrays as values: a fixed `type Mat2 [2][2]float64` with value semantics, contrasted
      with the slice-backed `Matrix`
- [ ] multi-dimensional data stored in a 1-D slice (row-major index math)
- [ ] `new(T)` vs `&T{}` vs `make` — use each at least once, know why
- [ ] `clear` builtin — on the backing slice and on a map
- [ ] `panic` for out-of-range indexing (like the language itself); `recover` in a
      boundary function to return an `error` instead
- [ ] **labeled** `break` / `continue` (pivot search across two loops)
- [ ] `goto` **exactly once**, somewhere you can defend (then answer self-check 5)
- [ ] the `slices` package where it genuinely helps (`slices.Equal`, `slices.Clone`)

## Background

**Row-major storage.** A `rows×cols` matrix lives in `data []float64` of length
`rows*cols`; element `(i, j)` is `data[i*cols + j]`.

**View vs copy.** `m.RowView(i)` returns `data[i*cols : (i+1)*cols : (i+1)*cols]` — a slice
that shares the backing array, so writing to it mutates the matrix, and the third index
stops a caller's `append` from silently overwriting row `i+1`. `m.Row(i)` returns a fresh
`slices.Clone` — independent.

**Panic vs error.** Indexing out of range is a bug in the caller, so `At`/`Set` panic
(Go's own slices do). A dimension mismatch in `Add`/`Mul` is an expected runtime condition
the caller should handle, so those return `error`. To show `recover`, the internal
primitive `mulInto` panics on mismatch and the exported `Mul` recovers and returns an
`error` — a boundary pattern you'll reuse in HTTP middleware (Project 20).

## Requirements

### Package API (`internal/matrix`)

```go
type Matrix struct { /* rows, cols int; data []float64 — all unexported */ }
type Vector []float64
type Mat2 [2][2]float64

// Constructors
func New(rows, cols int) *Matrix                  // zeroed; panics if rows<1 || cols<1
func FromRows(rows [][]float64) (*Matrix, error)   // error if ragged or empty
func Parse(s string) (*Matrix, error)             // "1 2 3; 4 5 6"
func Identity(n int) *Matrix
func Zeros(r, c int) *Matrix
func Ones(r, c int) *Matrix

// Accessors
func (m *Matrix) Dims() (rows, cols int)
func (m *Matrix) At(i, j int) float64              // panics on out-of-range
func (m *Matrix) Set(i, j int, v float64)          // panics on out-of-range
func (m *Matrix) Row(i int) Vector                 // COPY
func (m *Matrix) RowView(i int) []float64          // VIEW (writes through)
func (m *Matrix) Col(j int) Vector                 // COPY (always — non-contiguous)
func (m *Matrix) Clone() *Matrix

// Element-wise / scalar (return error on shape mismatch)
func (m *Matrix) Add(n *Matrix) (*Matrix, error)
func (m *Matrix) Sub(n *Matrix) (*Matrix, error)
func (m *Matrix) Scale(f float64) *Matrix
func (m *Matrix) Mul(n *Matrix) (*Matrix, error)   // matrix product; recovers from mulInto panic
func (m *Matrix) T() *Matrix                       // transpose (new matrix)
func (m *Matrix) Reshape(rows, cols int) (*Matrix, error) // error unless rows*cols == len

// Row operations (mutate the receiver, via RowView)
func (m *Matrix) SwapRows(a, b int)
func (m *Matrix) ScaleRow(i int, f float64)
func (m *Matrix) AddScaledRow(dst, src int, f float64) // row[dst] += f*row[src]

// Linear algebra
func (m *Matrix) RREF() *Matrix                    // reduced row echelon (new matrix)
func (m *Matrix) Det() (float64, error)            // error if not square
func (m *Matrix) Inverse() (*Matrix, error)        // error if not square or singular
func (m *Matrix) Rank() int

// Vectors
func (v Vector) Dot(w Vector) (float64, error)
func (v Vector) Norm() float64
func (v Vector) Add(w Vector) (Vector, error)
func (v Vector) Scale(f float64) Vector

// Compare
func Equal(a, b *Matrix, tol float64) bool

// Fixed 2x2 (value semantics)
func (a Mat2) Mul(b Mat2) Mat2
func (a Mat2) Det() float64

// Formatting
func (m *Matrix) String() string                  // fmt.Stringer, aligned
```

### Behaviour specification (every row is a test case)

| Operation | Input | Result |
|---|---|---|
| `New(2, 3)` | — | 2×3, all zeros |
| `New(0, 3)` | — | **panics** `matrix: New: dimensions must be >= 1, got 0x3` |
| `FromRows([[1,2],[3,4]])` | — | 2×2 matrix |
| `FromRows([[1,2],[3]])` | ragged | `error: row 1 has 1 columns, want 2` |
| `FromRows(nil)` / `FromRows([][]float64{})` | empty | `error: no rows` |
| `Parse("1 2; 3 4")` | — | equals `FromRows([[1,2],[3,4]])` |
| `Parse("1 2; 3")` | ragged | `error: row 1 has 1 columns, want 2` |
| `Parse("1 x")` | bad number | `error: "x" is not a number` |
| `Identity(3)` | — | diag of 1s |
| `m.At(5, 0)` on a 2×2 | — | **panics** `matrix: index (5,0) out of range for 2x2` |
| `m.Set(-1, 0, 9)` | — | **panics** (same format) |
| `a.Add(b)` shapes 2×2 and 2×3 | — | `error: shape mismatch: 2x2 vs 2x3` |
| `a.Mul(b)` 2×3 · 3×2 | — | 2×2 product |
| `a.Mul(b)` 2×3 · 2×2 | — | `error: cannot multiply 2x3 by 2x2` (recovered from `mulInto` panic) |
| `RowView(0)` then write | write `v[0]=9` | `m.At(0,0) == 9` |
| `Row(0)` then write | write `v[0]=9` | `m.At(0,0)` unchanged |
| `append(m.RowView(0), 1, 2, 3)` | — | does **not** corrupt row 1 (three-index slice) |
| `Identity(3).Det()` | — | `1` |
| `FromRows([[1,2],[3,4]]).Det()` | — | `-2` |
| `FromRows([[1,2,3],[4,5,6]]).Det()` | non-square | `error: matrix is 2x3, not square` |
| `FromRows([[1,2],[2,4]]).Inverse()` | singular | `error: matrix is singular` |
| `FromRows([[4,7],[2,6]]).Inverse()` | — | `[[0.6,-0.7],[-0.2,0.4]]` within `1e-9` |
| `Vector{1,2,2}.Norm()` | — | `3` |
| `Vector{1,2}.Dot(Vector{3,4})` | — | `11` |
| `Vector{1,2}.Dot(Vector{3,4,5})` | — | `error: length mismatch: 2 vs 3` |
| `Mat2{{1,2},{3,4}}.Det()` | — | `-2` |
| `Mat2` passed to a func that mutates it | — | caller's value unchanged (array copy) |

### Demo binary

`matdemo "1 2 3; 4 5 6" "7 8; 9 10; 11 12"` prints the product using `Matrix.String()`.
Bad input → `error:` line, exit 1. Wrong arg count → usage, exit 2.

### Error catalogue

| Trigger | Format |
|---|---|
| New dims | `matrix: New: dimensions must be >= 1, got %dx%d` (panic value) |
| index | `matrix: index (%d,%d) out of range for %dx%d` (panic value) |
| ragged | `row %d has %d columns, want %d` |
| empty | `no rows` |
| parse number | `%q is not a number` |
| add/sub shape | `shape mismatch: %dx%d vs %dx%d` |
| mul shape | `cannot multiply %dx%d by %dx%d` |
| not square | `matrix is %dx%d, not square` |
| singular | `matrix is singular` |
| vector length | `length mismatch: %d vs %d` |
| reshape | `cannot reshape %d elements into %dx%d` |

(Exported functions return `errors.New` / `fmt.Errorf` values wrapped with a package
prefix, e.g. `fmt.Errorf("matrix.Add: %w", errShapeMismatch)`. Panics are plain strings.)

## Suggested milestones

1. `Matrix` + `New` / `FromRows` / `Parse` / `Identity` / accessors / `String`.
2. `Row` vs `RowView` (+ the three-index slice) and tests that prove aliasing behaviour.
3. `Add` / `Sub` / `Scale` / `T` / `Reshape`.
4. `mulInto` (panics) + `Mul` (recovers → error).
5. Row ops → `RREF` (labeled `break` in the pivot search) → `Rank` / `Det` / `Inverse`.
6. `Vector` methods; `Mat2` value-semantics contrast.
7. The one `goto`. The `clear` uses. `matdemo`.
8. Full test suite.

## Project layout

```
projects/06-matrix/
  cmd/matdemo/main.go
  internal/matrix/matrix.go
  internal/matrix/rowops.go
  internal/matrix/linalg.go      // RREF, Det, Inverse, Rank
  internal/matrix/vector.go
  internal/matrix/mat2.go
  internal/matrix/*_test.go
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every row of the behaviour table has a test.
- [ ] `TestAliasing` proves `RowView` writes through and `Row` does not, and that
      `append(RowView(0), ...)` cannot corrupt an adjacent row.
- [ ] `TestMat2ValueSemantics` proves passing a `Mat2` to a function does not mutate the
      caller's variable.
- [ ] `At`/`Set` panics use the exact documented string; `Mul` never panics for a shape
      mismatch (it returns an error).
- [ ] `Inverse` and `Det` agree: `m.Mul(m.Inverse())` ≈ `Identity` within `1e-9` for a
      random well-conditioned 4×4 (property test).
- [ ] `go vet` flags nothing; there is exactly one `goto` in the package.

## Test requirements

- `TestConstructors` — `New`, `FromRows` (ok + ragged + empty), `Parse` (ok + errors),
  `Identity`.
- `TestIndexPanics` — `At`/`Set` out of range, asserting the panic string via
  `recover()` in the test.
- `TestArithmetic` — `Add`/`Sub`/`Scale`/`T`/`Mul` with known small matrices + shape
  errors.
- `TestAliasing`, `TestThreeIndexSlice`.
- `TestRREF` / `TestDet` / `TestInverse` / `TestRank` — known cases incl. singular.
- `TestInverseProperty` — `m · m⁻¹ ≈ I` for a generated 4×4.
- `TestVector`, `TestMat2ValueSemantics`.
- `BenchmarkMul` — 64×64 · 64×64 (baseline for a future cache-blocking stretch).
- `ExampleMatrix_String`.

## Stretch goals

- Cache-blocked `Mul`; benchmark against the naive version.
- `container/ring`-backed circular buffer of matrices for a moving average — preview of
  Project 11.
- LU decomposition; solve `Ax = b`.
- A generic `Matrix[T Numeric]` — but do Project 11 first, then come back.
- `m.MarshalText` / `UnmarshalText` round-tripping the `Parse` format.

## Self-check questions

1. `s := make([]float64, 3, 10)` — what are `len(s)` and `cap(s)`, and what happens to the
   backing array when you `append` a 4th, 8th, 11th element?
2. `RowView(0)` returns `data[0:cols:cols]`. Drop the third index. Now a caller does
   `r := m.RowView(0); r = append(r, 99)`. What did they just overwrite, and why does
   `:cols` prevent it?
3. `m.Row(0)` returns `slices.Clone(...)`. If `Clone` didn't exist, `s2 := s1` would...
   do what? Why isn't that a copy?
4. Passing a `Mat2` (array) to `func f(m Mat2)` copies 32 bytes; passing a `*Matrix`
   copies 24 bytes (the slice header) but shares the data. Which is "pass by value" and
   why do people disagree about that phrase?
5. Show me your one `goto`. Rewrite that function without it. Which is clearer — honestly?
6. `At(5, 0)` panics; `Add` on mismatched shapes returns an error. State the rule you used
   to decide, in one sentence.
7. In `Mul`, `recover()` returns `any`. How do you turn it back into an `error`, and what
   do you do if the recovered value *isn't* one of your expected panic types?
