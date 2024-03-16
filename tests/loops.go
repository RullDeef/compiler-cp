package main

func main() {
	sum := 0
	for i := 0; i < 100; i++ {
		if i == 50 {
			continue
		}
		sum = sum + i
	}
	printf("sum = %d (4900)\n", sum)
	for sum > 50 {
		sum--
		if sum > 60 {
			printf("breaking out!\n")
			break
		}
	}
}
