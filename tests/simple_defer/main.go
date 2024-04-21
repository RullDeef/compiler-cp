package main

import "fmt"

func foo(val int) {
	defer bar(val)
	fmt.Printf("foo -> %d\n", val)
}

func bar(val int) {
	defer fmt.Printf("bar -> %d\n", val)

	for i := -5; i < 15; i++ {
		val += i
	}
}

func main() {
	defer fmt.Printf("defer no.1\n")
	defer fmt.Printf("defer no.2\n")
	foo(123)
	defer fmt.Printf("defer no.3\n")
	defer fmt.Printf("defer no.4\n")
}
