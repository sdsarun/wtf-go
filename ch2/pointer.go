package ch2

import "fmt"

func HowPointerWork() {
	createPointer()
}

// NewVsMake demonstrates when to use new() vs make().
//
// new(T)
//   - Allocates zeroed memory for any type T
//   - Returns a *T (pointer to the zero value)
//   - Use when you need a pointer to a value type (int, struct, array, etc.)
//
// make(T, ...)
//   - Only works with slice, map, and channel
//   - Allocates AND initialises the internal structure (len, cap, backing array, etc.)
//   - Returns the type itself (not a pointer) because these are already reference types
//   - Use when you need a ready-to-use slice, map, or channel
func NewVsMake() {
	// ── new ──────────────────────────────────────────────────────────────────
	// Allocates a zeroed int and gives back a pointer to it.
	//  Memory:  [ *int ] ──▶ [ 0 ]
	pi := new(int)
	*pi = 42
	fmt.Printf("new(int)    → pointer %p, value %d\n", pi, *pi)

	// Allocates a zeroed struct and gives back a pointer to it.
	//  Memory:  [ *Point ] ──▶ [ X:0, Y:0 ]
	type Point struct{ X, Y int }
	pp := new(Point)
	pp.X, pp.Y = 3, 4
	fmt.Printf("new(Point)  → pointer %p, value %+v\n", pp, *pp)

	// ── make ─────────────────────────────────────────────────────────────────
	// make([]T, len, cap) — creates a slice with backing array ready to use.
	//  Memory:  [ slice header: ptr | len=3 | cap=5 ] ──▶ [ 0, 0, 0, _, _ ]
	// new([]int) would give *[]int pointing to a nil slice header — not useful.
	sl := make([]int, 3, 5)
	sl[0] = 10
	fmt.Printf("make([]int) → len=%d cap=%d value=%v\n", len(sl), cap(sl), sl)

	// make(map[K]V) — initialises the hash table; without this the map is nil
	//  and any write would panic.
	//  new(map[string]int) gives a *map pointing to nil — writing would panic.
	m := make(map[string]int)
	m["a"] = 1
	fmt.Printf("make(map)   → %v\n", m)

	// make(chan T, buf) — initialises the channel with an optional buffer.
	//  new(chan int) gives a *chan pointing to nil — sending would block forever.
	ch := make(chan int, 1)
	ch <- 99
	fmt.Printf("make(chan)  → received %d\n", <-ch)

	// ── quick decision guide ──────────────────────────────────────────────────
	// ┌────────────────────────┬───────────────────────────────────────────────┐
	// │ You need …             │ Use                                           │
	// ├────────────────────────┼───────────────────────────────────────────────┤
	// │ pointer to int/struct  │ new(T)   → returns *T                         │
	// │ ready-to-use slice     │ make([]T, len, cap)                           │
	// │ ready-to-use map       │ make(map[K]V)                                 │
	// │ ready-to-use channel   │ make(chan T, buf)                              │
	// └────────────────────────┴───────────────────────────────────────────────┘
}

func createPointer() {
	var pA, pB = new(int), new(int)
	*pA = 10
	*pB = 20

	// Memory layout before swap:
	//
	//  Stack                Heap
	//  ┌──────────┐         ┌──────────┐
	//  │ pA  0x20 │────────▶│  10      │ 0x20
	//  └──────────┘         └──────────┘
	//  ┌──────────┐         ┌──────────┐
	//  │ pB  0x30 │────────▶│  20      │ 0x30
	//  └──────────┘         └──────────┘

	fmt.Println("=== Before Swap ===")
	fmt.Printf("pA var lives at : %p\n", &pA)
	fmt.Printf("pA points to    : %p  value: %d\n", pA, *pA)
	fmt.Printf("pB var lives at : %p\n", &pB)
	fmt.Printf("pB points to    : %p  value: %d\n", pB, *pB)

	swap(&pA, &pB)

	// Memory layout after swap:
	// The heap values are untouched. Only the addresses stored in pA/pB are exchanged.
	//
	//  Stack                Heap
	//  ┌──────────┐         ┌──────────┐
	//  │ pA  0x30 │────────▶│  20      │ 0x30
	//  └──────────┘         └──────────┘
	//  ┌──────────┐         ┌──────────┐
	//  │ pB  0x20 │────────▶│  10      │ 0x20
	//  └──────────┘         └──────────┘

	fmt.Println("=== After Swap ===")
	fmt.Printf("pA var lives at : %p  (same)\n", &pA)
	fmt.Printf("pA points to    : %p  value: %d\n", pA, *pA)
	fmt.Printf("pB var lives at : %p  (same)\n", &pB)
	fmt.Printf("pB points to    : %p  value: %d\n", pB, *pB)
}

// swap exchanges the addresses stored in two pointer variables.
// It receives **int so it can mutate the caller's pointer variables.
//
//	Inside swap (before):
//	┌───┐   ┌──────────┐         ┌──────────┐
//	│ a │──▶│ pA = 0x20│────────▶│  10      │ 0x20
//	└───┘   └──────────┘         └──────────┘
//	┌───┐   ┌──────────┐         ┌──────────┐
//	│ b │──▶│ pB = 0x30│────────▶│  20      │ 0x30
//	└───┘   └──────────┘         └──────────┘
//
//	Inside swap (after):
//	a and b still point to the same pA/pB locations — only the stored address changes.
//	┌───┐   ┌──────────┐         ┌──────────┐
//	│ a │──▶│ pA = 0x30│────────▶│  20      │ 0x30
//	└───┘   └──────────┘         └──────────┘
//	┌───┐   ┌──────────┐         ┌──────────┐
//	│ b │──▶│ pB = 0x20│────────▶│  10      │ 0x20
//	└───┘   └──────────┘         └──────────┘
func swap(a **int, b **int) {
	// Initial state:
	//  a ──▶ [ pA = 0x20 ] ──▶ [ 10 ]
	//  b ──▶ [ pB = 0x30 ] ──▶ [ 20 ]

	t := *a
	// t now holds the address pA was storing (0x20)
	//  t        = 0x20
	//  a ──▶ [ pA = 0x20 ] ──▶ [ 10 ]
	//  b ──▶ [ pB = 0x30 ] ──▶ [ 20 ]

	*a = *b
	// pA now stores the address pB was storing (0x30)
	//  t        = 0x20
	//  a ──▶ [ pA = 0x30 ] ──▶ [ 20 ]
	//  b ──▶ [ pB = 0x30 ] ──▶ [ 20 ]

	*b = t
	// pB now stores the saved address (0x20)
	//  t        = 0x20
	//  a ──▶ [ pA = 0x30 ] ──▶ [ 20 ]
	//  b ──▶ [ pB = 0x20 ] ──▶ [ 10 ]
}

