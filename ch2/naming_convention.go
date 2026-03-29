package ch2

import "fmt"

/*
GO NAMING CHEAT SHEET (for talking)

1. Use short and clear names
2. Avoid snake_case
3. Avoid very long names
4. Use context to reduce name length
5. Use camelCase for variables/functions
6. Use lowercase for package/file
*/

// =======================
// GOOD EXAMPLES
// =======================

// function name: short + verb
func createH1() string {
	return "<h1>Hello</h1>"
}

func GoodNames() {
	// variable: short, clear, use context
	h1 := createH1()

	fmt.Println("Good:", h1)
}

// =======================
// BAD EXAMPLES
// =======================

func BadNames() {
	// ❌ too long + redundant + Java style
	var HTMLHeadingElementStringValue = createH1()

	// ❌ snake_case (not Go style)
	var html_heading = createH1()

	// ❌ unclear meaning
	var x = createH1()

	fmt.Println("Bad:", HTMLHeadingElementStringValue, html_heading, x)
}
