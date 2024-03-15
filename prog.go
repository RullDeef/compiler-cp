package main

func fib(n int) int {
	if n <= 1 {
		return n
	} else {
		return fib(n-1) + fib(n-2)
	}
}

var fib5, fib10 int = fib(5), fib(10)

func main() {
	printf("fib5 = %d\n", fib5)
	printf("fib10 = %d\n", fib10)
}
