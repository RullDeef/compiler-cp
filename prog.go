package main

func main() {
	printf("1\n")
	printf("2\n")

	goto some_label

	printf("3\n")

	goto some_label

	printf("4\n")

some_label:
	printf("5\n")

}
