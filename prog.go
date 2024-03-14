package main

// printf(format i8*, ...)

func main() {
	a := 10
	printf("a = %d\n", a)
	if 1 > 0 {
		a = 20
		printf("a = %d\n", a)
	}
	printf("a = %d\n", a)
}
