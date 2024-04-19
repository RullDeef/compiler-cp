package main

import "fmt"

func main() {
	var arr [10]int
	arr[0] = 100

	var i int
	for i = 1; i < 10; i++ {
		arr[i] = i
	}

	for i = 0; i < 10; i++ {
		fmt.Printf("%d ", arr[i])
	}
	fmt.Printf("\n")
}
