package main

import "fmt"

type vec2d struct {
	x int
	y int
}

func dist(p1, p2 vec2d) vec2d {
	return vec2d{
		x: p2.x - p1.x,
		y: p2.y - p1.y,
	}
}

func main() {
	p1 := vec2d{5, 4}
	p2 := vec2d{6, 1}
	d := dist(p1, p2)
	fmt.Printf("{%d %d}\n", d.x, d.y)
}
