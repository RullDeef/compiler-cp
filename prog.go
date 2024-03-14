package main

// printf(format i8*, ...)

func main() {
	for i := 0; i < 5; i = i + 1 {
		printf("loop iteration no %d\n", i)
	}
}
