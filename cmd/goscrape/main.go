package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/purpleclay/go-overlay/internal/cli/goscrape"
	"github.com/purpleclay/x/cli"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func main() {
	version := cli.VersionInfo{
		Version:   Version,
		GitCommit: Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}

	if err := goscrape.Execute(version); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
