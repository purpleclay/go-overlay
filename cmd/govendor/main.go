package main

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/purpleclay/go-overlay/cmd/govendor/internal/cli"
	xcli "github.com/purpleclay/x/cli"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	version := xcli.VersionInfo{
		Version:   Version,
		GitCommit: Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}

	// cli.Execute already renders the error to stderr via the configured
	// error handler before returning it here.
	if code, err := cli.Execute(ctx, version, os.Args[1:]); err != nil {
		os.Exit(code)
	}
}
