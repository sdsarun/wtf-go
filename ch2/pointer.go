package ch2

import "fmt"

func HowPointerWork() {
	createPointer()
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
//	Inside swap (before):       Inside swap (after):
//	┌───┐   ┌───┐               ┌───┐   ┌───┐
//	│ a │──▶│pA │               │ a │──▶│pB │  (pA now holds 0x30)
//	└───┘   └───┘               └───┘   └───┘
//	┌───┐   ┌───┐               ┌───┐   ┌───┐
//	│ b │──▶│pB │               │ b │──▶│pA │  (pB now holds 0x20)
//	└───┘   └───┘               └───┘   └───┘
func swap(a **int, b **int) {
	t := *a  // t = address pA was holding (0x20)
	*a = *b  // pA now holds address pB was holding (0x30)
	*b = t   // pB now holds the saved address (0x20)
}