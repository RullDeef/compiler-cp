package main

func main() {
	x, y := 10, 20

	// simple swap operations
	x, y = y, x
	x, _, y = y, x, 5

	printf("x = %d, y = %d\n", x, y)
}
