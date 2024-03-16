package main

func main() {
	sum := 0
	for i := 0; i < 100; i++ {
		sum = sum + i
	}
	printf("sum = %d (4950)\n", sum)
	for sum > 50 {
		sum--
	}
}
