package main

import "fmt"

func main() {
	var arr [1024]int
	var size, index uint

	defer fmt.Printf("Спасибо, досвидания\n")

	fmt.Printf("Введите размер массива (не больше 10): ")
	fmt.Scanf("%u", &size)
	fmt.Printf("\n")

	if size > 10 {
		fmt.Printf("Дурак? Написано же, не больше 10\n")
		return
	}

	for index = 0; index < size; index++ {
		fmt.Printf("Введите %u-е число: ", index+1)
		fmt.Scanf("%d", &arr[index])
	}
	fmt.Printf("\n")

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
		fmt.Printf("%d ", arr[index])
	}
	fmt.Printf("\n")
}
