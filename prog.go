package main

func main() {
	for i := 0; i < 6; i++ {
		if i == 4 {
			continue
		}
		printf("i = %d\n", i)
	}
}
