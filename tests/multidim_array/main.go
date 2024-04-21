package main

import "fmt"

func main() {
	var arr [5][7]int

	// fill matrix in zig-zag
	row, col := 0, 0
	for e := 0; e < 5*7; e++ {
		arr[row][col] = e
		if (row+col)%2 == 0 {
			row--
			col++
			if col >= 7 {
				col = 6
				row += 2
			} else if row == -1 {
				row = 0
			}
		} else {
			row++
			col--
			if row >= 5 {
				row = 4
				col += 2
			} else if col == -1 {
				col = 0
			}
		}
	}

	// random spice
	arr[2][3] = 404

	// output matrix out
	for i := 0; i < 5; i++ {
		for j := 0; j < 7; j++ {
			if j > 0 {
				fmt.Printf(" ")
			}
			fmt.Printf("%d", arr[i][j])
		}
		fmt.Printf("\n")
	}
}
