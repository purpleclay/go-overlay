package main

import (
	"fmt"

	"github.com/fatih/color"
)

//go:generate go tool stringer -type=Fruit
type Fruit int

const (
	Apple Fruit = iota
	Banana
	Cherry
)

func main() {
	color.Green("fruit: %s", Apple)
	fmt.Println(Banana)
}
