package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/laurenschristian/ibkrctl/internal/cli"
)

var version = "dev"

func main() {
	if version == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			version = bi.Main.Version
		}
	}
	cli.Version = version
	if err := cli.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", cli.RedactError(err))
		os.Exit(1)
	}
}
