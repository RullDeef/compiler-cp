package main

import "fmt"

const c1 = -4

const (
	c2 = 19.3
	c3 = false
)

var v1, v2 = 12, 43 + 21*2

var (
	v3 = v2
	v4 = c2
)

func main() {
	fmt.Printf("%d\n", c1)
	fmt.Printf("%f\n", c2)
	fmt.Printf("%d\n", c3)
	fmt.Printf("%d\n", v1)
	fmt.Printf("%d\n", v2)
	fmt.Printf("%d\n", v3)
	fmt.Printf("%f\n", v4)
}
