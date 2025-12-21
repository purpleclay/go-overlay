package main

import (
	"fmt"

	"github.com/aymanbagabas/go-udiff"
	"github.com/charmbracelet/x/ansi"
)

func main() {
	_ = udiff.Unified("a", "b", "hello", "world")
	fmt.Println(ansi.Strip("hello"))
}
