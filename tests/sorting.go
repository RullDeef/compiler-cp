package main

func main() {
	var arr [1024]int
	var size, index uint

	defer printf("Спасибо, досвидания\n")

	printf("Введите размер массива (не больше 10): ")
	scanf("%u", &size)

	if size > 10 {
		printf("Дурак? Написано же, не больше 10\n")
		return
	}

	for index = 0; index < size; index++ {
		printf("Введите %u-е число: ", index+1)
		scanf("%d", &arr[index])
	}

	var i, j uint
	for i = 1; i < size; i++ {
		for j = 0; j < i; j++ {
			if arr[j] > arr[i] {
				tmp := arr[i]
				arr[i] = arr[j]
				arr[j] = tmp
			}
		}
	}

	for index = 0; index < size; index++ {
		printf("%d ", arr[index])
	}
	printf("\n")
}
