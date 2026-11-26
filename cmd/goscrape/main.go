package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/purpleclay/go-overlay/cmd/goscrape/internal/cli"
	xcli "github.com/purpleclay/x/cli"
)

var (
	Version   = "dev"
	BuildDate = "unknown"
)

func main() {
	version := xcli.VersionInfo{
		Version:   Version,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}

	if err := cli.Execute(version); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
