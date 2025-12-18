package main

import (
	"fmt"
	"os"

	"github.com/purpleclay/go-overlay/internal/cli/govendor"
)

func main() {
	if err := govendor.Execute(os.Stdout); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
