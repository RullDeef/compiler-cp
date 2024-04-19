package main

import "fmt"

func main() {
	x, y := 10, 20

	// simple swap operations
	x, y = y, x
	x, _, y = y, x, 5

	fmt.Printf("x = %d, y = %d\n", x, y)
}
