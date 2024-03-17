package main

func minPtr(a, b *int) *int {
	if *a < *b {
		return a
	} else {
		return b
	}
}

func main() {
	num, num2 := 120, 52
	num2 = 53
	*(&num) = num + 1
	var ptr *int
	ptr = &num
	*ptr = 13
	*minPtr(&num, &num2) = 0
	printf("num = %d, num2 = %d\n", num, num2)
}
