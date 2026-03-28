package ch1

import "fmt"

const ROWS = 10

func SimpleRoutine() {
	ch := make(chan string)

	go func() {
		for i := 1; i <= ROWS; i++ {
			for j := 1; j <= i; j++ {
				ch <- "*"
			}
			ch <- "\n"
		}
		close(ch)
	}()

	for msg := range ch {
		fmt.Print(msg)
	}
}
