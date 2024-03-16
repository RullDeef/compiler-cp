package main

func main() {
	printf("1 ")
	printf("2 ")

	goto some_label

	printf("3 ")

	goto some_label

	printf("4 ")

some_label:
	printf("5\n")
}
