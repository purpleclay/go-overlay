package main

import (
	"fmt"

	"github.com/fatih/color"
)

func Add(a, b int) int {
	return a + b
}

func main() {
	color.Green("%d", Add(1, 2))
	fmt.Println(Add(3, 4))
}
