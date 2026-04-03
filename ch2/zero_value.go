package ch2

import "fmt"

func ShowZeroValue() {
	printIntZeroValue()
	printBoolZeroValue()
	printStringZeroValue()
	printPointerZeroValue()
	printInterfaceZeroValue()
	printReferenceZeroValue()
	printAggregateZeroValue()
}

// all zero values is 0
func printIntZeroValue() {
	var i int
	var i8 int8
	var i16 int16
	var i32 int32
	var i64 int64
	var u uint
	var u8 uint8
	var u16 uint16
	var u32 uint32
	var u64 uint64
	var uptr uintptr

	fmt.Printf("int:     %v\n", i)
	fmt.Printf("int8:    %v\n", i8)
	fmt.Printf("int16:   %v\n", i16)
	fmt.Printf("int32:   %v\n", i32)
	fmt.Printf("int64:   %v\n", i64)
	fmt.Printf("uint:    %v\n", u)
	fmt.Printf("uint8:   %v\n", u8)
	fmt.Printf("uint16:  %v\n", u16)
	fmt.Printf("uint32:  %v\n", u32)
	fmt.Printf("uint64:  %v\n", u64)
	fmt.Printf("uintptr: %v\n", uptr)
}

// zero value of bool is false
func printBoolZeroValue() {
	var b bool
	fmt.Printf("bool: %v\n", b)
}

// zero value of string is ""
func printStringZeroValue() {
	var s string
	fmt.Printf("string: %q\n", s)
}

// zero value of pointer is nil
func printPointerZeroValue() {
	var p *int
	fmt.Printf("*int pointer: %v\n", p)
}

// zero value of interface is nil
func printInterfaceZeroValue() {
	var i interface{}
	fmt.Printf("interface{}: %v\n", i)
}

// zero values of reference types (slice, map, channel, func) are nil
func printReferenceZeroValue() {
	var sl []int
	var m map[string]int
	var ch chan int
	var fn func()
	fmt.Printf("slice:   %v\n", sl)
	fmt.Printf("map:     %v\n", m)
	fmt.Printf("channel: %v\n", ch)
	fmt.Printf("func:    %v\n", fn == nil)
}

// zero values of aggregate types: array elements and struct fields are zeroed
func printAggregateZeroValue() {
	var arr [3]int
	type Point struct {
		X, Y int
		Name string
		Valid bool
	}
	var pt Point
	fmt.Printf("array:  %v\n", arr)
	fmt.Printf("struct: %v\n", pt)
}