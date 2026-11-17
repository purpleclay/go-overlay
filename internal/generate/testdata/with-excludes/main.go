package main

import (
	"fmt"

	"github.com/aymanbagabas/go-udiff"
	"github.com/charmbracelet/x/ansi"
)

func main() {
	fmt.Println(udiff.Unified("", "", "", ""))
	fmt.Println(ansi.Strip(""))
}
