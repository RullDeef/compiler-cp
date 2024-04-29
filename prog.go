package main

import "fmt"

type point struct {
	x int
	y int
}

func main() {
	var p0 point

	p0.p0.x = 0

	fmt.Printf("%d:%d\n", p0.x, p0.y)
}
