package main

// printf(format i8*, ...)

func main() {
	i := 1
	for {
		printf("loop iteration no %d\n", i)
		i++
		if i == 10 {
			return
		}
	}
}
