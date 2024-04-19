package main

import "fmt"

func main() {
	sum := 0
	for i := 0; i < 100; i++ {
		if i == 50 {
			continue
		}
		sum = sum + i
	}
	fmt.Printf("sum = %d\n", sum)
	for sum > 50 {
		sum--
		if sum > 60 {
			fmt.Printf("breaking out!\n")
			break
		}
	}
}
