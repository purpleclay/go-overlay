package main

import (
	"fmt"
	"os"

	"github.com/purpleclay/go-overlay/internal/cli/govendor"
)

func main() {
	if err := govendor.Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
