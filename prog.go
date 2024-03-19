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

	a, b, c := sort3(40.9, 21.0, 167.0)

	printf("res = %d, ok = %d\n", res, ok)
	printf("a = %f, b = %f, c = %f\n", a, b, c)

	// в общем, почему-то не выставляются дополнительные звездочки при использовании
	// выходных параметров.

	// %51 = load double, double* %4
	// %52 = load double, double* %5
	// %53 = load double, double* %3
	// %54 = load double, double** %0
	// store double %51, double %54
	// %55 = load double, double** %1
	// store double %52, double %55
	// %56 = load double, double** %2
	// store double %53, double %56

	// вместо

	// %51 = load double, double* %4
	// %52 = load double, double* %5
	// %53 = load double, double* %3
	// %54 = load double*, double** %0
	// store double %51, double* %54
	// %55 = load double*, double** %1
	// store double %52, double* %55
	// %56 = load double*, double** %2
	// store double %53, double* %56
}
