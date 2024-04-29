package main

import "fmt"

func main() {
	a := int64(0xFFFFFFF)
	b := 3.1415926535
	bfp := float32(b) + float32(0.1)

	fmt.Printf("%ld\n%d\n", a, int8(a))
	fmt.Printf("%d\n", int(bfp))
	fmt.Printf("%.9lf\n", b)
}
