package main

// simple multiple return functions

func safeDivide(x, y int) (int, bool) {
	if y == 0 {
		return 0, false
	}
	return x / y, true
}

func sort3(a, b, c float64) (x float64, y float64, z float64) {
	if a < b {
		if b < c {
			x, y, z = a, b, c
		} else if a < c {
			x, y, z = a, c, b
		} else {
			x, y, z = c, a, b
		}
	} else if c < b {
		x, y, z = c, b, a
	} else if a < c {
		x, y, z = b, a, c
	} else {
		x, y, z = b, c, a
	}
	return
}

func main() {
	x, y := 100, 20
	res, ok := safeDivide(x, y)

	printf("res = %d, ok = %d\n", res, ok)
}
