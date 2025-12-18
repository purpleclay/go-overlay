package main

import (
	"fmt"
	"os"

	"github.com/purpleclay/go-overlay/internal/cli/goscrape"
)

func main() {
	if err := goscrape.Execute(os.Stdout); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
