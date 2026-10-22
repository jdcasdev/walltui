// walltui is a keyboard-driven TUI for managing wallpapers on Linux.
package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/jdcasdev/walltui/internal/cli"
	"github.com/jdcasdev/walltui/internal/errs"
)

// version is injected at build time via LDFLAGS (-X main.version=...).
var version = "dev"

func main() {
	if err := cli.Execute(buildVersion()); err != nil {
		fmt.Fprintln(os.Stderr, "walltui:", err)
		os.Exit(errs.Code(err))
	}
}

// buildVersion reports the release the binary came from. Builds with ldflags
// carry it in the variable; go install leaves it unset, so the module version
// is read from the build info instead.
func buildVersion() string {
	v := version
	if v == "dev" {
		info, ok := debug.ReadBuildInfo()
		if !ok {
			return v
		}
		v = info.Main.Version
	}
	return normalizeVersion(v)
}

func normalizeVersion(v string) string {
	if v == "" || v == "(devel)" {
		return "dev"
	}
	return strings.TrimPrefix(v, "v")
}
