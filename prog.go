package main

func bar(a int) int {
	printf("a!\n")
	return a
}

func main() {
	var a int = 5
	_, a, _ = bar(a), 3, 1230
	printf("a = %d\n", a)
}
