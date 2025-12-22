package main

import (
	"os"

	"github.com/purpleclay/go-overlay/internal/cli/govendor"
)

func main() {
	if err := govendor.Execute(); err != nil {
		os.Exit(1)
	}
}
