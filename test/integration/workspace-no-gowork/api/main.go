package main

import (
	"fmt"

	"example.com/shared"
	"github.com/aymanbagabas/go-udiff"
)

func main() {
	fmt.Println(shared.Hello())
	_ = udiff.Unified("a", "b", "c", "d")
}
