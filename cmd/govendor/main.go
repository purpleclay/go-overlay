package main

import (
	"os"
	"runtime"

	"github.com/purpleclay/go-overlay/internal/cli/govendor"
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

	// cli.Execute already renders the error to stderr via the configured
	// error handler before returning it here.
	if code, err := govendor.Execute(version, os.Args[1:]); err != nil {
		os.Exit(code)
	}
}
