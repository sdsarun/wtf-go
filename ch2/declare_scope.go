package ch2

import (
	"fmt"
)

func FunctionScope() {
	// visible all inside FunctionScope
	var a = 10
	{
		// only in brance
		var b = 20
		fmt.Println(a, b)
	}
	fmt.Println(a, globalVariable)

	// only if scope
	if c := 10; c > 1 {
		fmt.Println(c)
	}
	// fmt.Println(c) // c is undefined
}

func GlobalScope() {
	fmt.Println(globalVariable)
}
