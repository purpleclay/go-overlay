package main

import (
	"os"

	"github.com/purpleclay/go-overlay/internal/cli/govendor"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func main() {
	build := govendor.NewBuildDetails(Version, Commit, BuildDate)

	if err := govendor.Execute(build); err != nil {
		os.Exit(1)
	}
}
