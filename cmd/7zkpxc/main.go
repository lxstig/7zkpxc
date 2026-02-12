package main

import (
	"fmt"
	"os"

	"github.com/lxstig/7zkpxc/internal/app"
)

// These variables are set at build time by goreleaser via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	app.SetVersionInfo(version, commit, date)

	if err := app.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
