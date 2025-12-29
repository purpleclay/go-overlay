package main

import (
	"example.com/shared"
	"github.com/fatih/color"
)

func main() {
	msg := shared.Hello()
	color.Green(msg)
}
