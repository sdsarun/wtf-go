package ch2

import "fmt"

const (
	_ = 1 << (iota * 10) // = 1 << 10
	KB
	MB
	GB
)

func ConstantsValues() {
	fmt.Println(KB)
	fmt.Println(MB)
	fmt.Println(GB)
}
