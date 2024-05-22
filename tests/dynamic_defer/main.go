package main

import "fmt"

func hello(a int, sum *int) {
	*sum += a
	fmt.Printf("hello: %d!\n", a)
}

func printSum(sum *int) {
	fmt.Printf("sum = %d\n", *sum)
}

func main() {
	sum := 0
	defer printSum(&sum)
	for i := 10; i <= 50; i += 10 {
		defer hello(i, &sum)
	}
}
