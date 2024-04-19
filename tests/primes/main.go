package main

import "fmt"

func prime(x int) bool {
	for n := 2; n < x/2; n++ {
		if x%n == 0 {
			return false
		}
	}
	return true
}

func getNextPrime(x int) int {
	x = x + 1
	for !prime(x) {
		x = x + 1
	}
	return x
}

func printFirstPrimes(n int) {
	num := 0
	for i := 1; i <= n; i++ {
		num = getNextPrime(num)
		fmt.Printf("%d ", num)
	}
	fmt.Printf("\n")
}

func main() {
	printFirstPrimes(20)
}
