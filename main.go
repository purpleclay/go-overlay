package main

import (
	"fmt"
	"go-scrape/cmd"
	"os"
)

func main() {
	if err := cmd.Execute(os.Stdout); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
