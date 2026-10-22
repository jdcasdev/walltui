// Package cli runs walltui: the picker TUI, a headless apply, plus --version
// and --help.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/jdcasdev/walltui/internal/apply"
	"github.com/jdcasdev/walltui/internal/config"
	"github.com/jdcasdev/walltui/internal/errs"
	"github.com/jdcasdev/walltui/internal/ui"
)

var applyWallpaper = func(cfg config.Config, path string) (string, error) {
	entry, err := apply.Run(cfg, path, apply.Options{})
	return entry.Abs, err
}

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
	switch {
	case fs.NArg() == 0:
		return runPicker()
	case fs.Arg(0) == "apply":
		if fs.NArg() > 2 {
			return fmt.Errorf("%w: apply takes at most one path", errs.ErrUsage)
		}
		return runApply(fs.Arg(1))
	default:
		return fmt.Errorf("%w: unknown command %q", errs.ErrUsage, fs.Arg(0))
	}
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

// runApply applies a wallpaper without the TUI: the given path, or the stored
// one.
func runApply(path string) error {
	applied, err := applyWallpaper(config.Load(), path)
	if err != nil {
		return err
	}
	fmt.Println(applied)
	return nil
}
