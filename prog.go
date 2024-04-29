package main

import "fmt"

func greet(msg string) {
	fmt.Printf("hello, %s!\n", msg)
}

func main() {
	s := "aboba"
	greet(s)
}
