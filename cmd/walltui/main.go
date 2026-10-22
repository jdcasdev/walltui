// walltui is a keyboard-driven TUI for managing wallpapers on Linux.
package main

import (
	"fmt"
	"os"

	"github.com/jdcasdev/walltui/internal/cli"
	"github.com/jdcasdev/walltui/internal/errs"
)

// version is injected at build time via LDFLAGS (-X main.version=...).
var version = "dev"

func main() {
	if err := cli.Execute(version); err != nil {
		fmt.Fprintln(os.Stderr, "walltui:", err)
		os.Exit(errs.Code(err))
	}
}
