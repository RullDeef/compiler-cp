package main

import "fmt"

type point struct {
	x int
	y int
}

func main() {
	p0 := point{
		y: -3,
		x: 12,
	}

	fmt.Printf("%d:%d\n", p0.x, p0.y)

	arr := [3]bool{
		2: true,
		1: false,
		0: true,
	}

	for i := 0; i < 3; i++ {
		if i > 0 {
			fmt.Printf(" ")
		}
		if arr[i] {
			fmt.Printf("true")
		} else {
			fmt.Printf("false")
		}
	}
	fmt.Printf("\n")

	pArr := [10]point{
		3: {x: 8, y: -12},
		{y: 5},
		{x: -2},
	}

	for i := 0; i < 10; i++ {
		if i > 0 {
			fmt.Printf(", ")
		}
		fmt.Printf("{%d:%d}", pArr[i].x, pArr[i].y)
	}
	fmt.Printf("\n")
}
