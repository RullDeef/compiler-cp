package main

func main() {
	defer printf("\n")
	for i := 3; i < 6; i++ {
		a := i
		printf("%d ", a)
	}
}
