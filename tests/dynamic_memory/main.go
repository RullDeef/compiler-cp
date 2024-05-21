package main

import "fmt"

type vec struct {
	x float64
	y float64
}

func newVec(x, y float64) *vec {
	res := &vec{}
	res.x = x
	res.y = y
	return res
}

func vecSum(v1, v2 *vec) *vec {
	return &vec{
		x: v1.x + v2.x,
		y: v1.y + v2.y,
	}
}

func main() {
	v1 := newVec(20.0, 31.0)
	v2 := vec{13.0, 71.2}

	v3 := vecSum(v1, &v2)
	fmt.Printf("v3 = {%f, %f}\n", v3.x, v3.y)
}
