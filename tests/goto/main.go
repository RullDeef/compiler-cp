package main

import "fmt"

func main() {
	fmt.Printf("1 ")
	fmt.Printf("2 ")
	goto some_label2

lbl3:
	fmt.Printf("3 ")
	goto some_label

some_label2:
	fmt.Printf("4 ")
	goto lbl3

some_label:
	fmt.Printf("5\n")
}
