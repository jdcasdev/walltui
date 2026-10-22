// Package cli runs walltui: the picker TUI, plus --version and --help. There
// are no subcommands, so the stdlib flag package is enough.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/jdcasdev/walltui/internal/config"
	"github.com/jdcasdev/walltui/internal/errs"
	"github.com/jdcasdev/walltui/internal/ui"
)

// Execute runs the CLI and returns the first error.
func Execute(version string) error {
	fs := flag.NewFlagSet("walltui", flag.ContinueOnError)
	showVersion := fs.Bool("version", false, "print the version and exit")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil // -h: the usage was already printed
		}
		return errs.ErrUsage // flag printed the message and the usage
	}
	if *showVersion {
		fmt.Println("walltui", version)
		return nil
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%w: unexpected argument %q", errs.ErrUsage, fs.Arg(0))
	}
	return runPicker()
}

func runPicker() error {
	if _, err := tea.NewProgram(ui.New(config.Load())).Run(); err != nil {
		if errors.Is(err, tea.ErrInterrupted) {
			return nil // Ctrl+C: a normal exit, not a failure
		}
		return fmt.Errorf("%w: running the picker: %v", errs.ErrIntegration, err)
	}
	return nil
}
