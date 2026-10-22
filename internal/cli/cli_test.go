package cli

import (
	"errors"
	"os"
	"testing"

	"github.com/jdcasdev/walltui/internal/config"
	"github.com/jdcasdev/walltui/internal/errs"
)

// withArgs runs Execute with a fake command line.
func withArgs(t *testing.T, args ...string) error {
	t.Helper()
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = append([]string{"walltui"}, args...)
	return Execute("test")
}

// TestExecuteRejectsArguments guards the CLI surface: only the apply command
// takes a positional argument, so a forgotten verb is a loud usage error
// instead of a silently ignored word.
func TestExecuteRejectsArguments(t *testing.T) {
	for _, arg := range []string{"current", "/walls/a.png", "aply"} {
		if err := withArgs(t, arg); !errors.Is(err, errs.ErrUsage) {
			t.Errorf("Execute(%q) = %v, want usage error", arg, err)
		}
	}
}

// TestExecuteApply checks the apply command reaches the apply seam, with and
// without an explicit path.
func TestExecuteApply(t *testing.T) {
	var got string
	old := applyWallpaper
	t.Cleanup(func() { applyWallpaper = old })
	applyWallpaper = func(cfg config.Config, path string) (string, error) {
		got = path
		return "/walls/a.png", nil
	}

	if err := withArgs(t, "apply"); err != nil {
		t.Errorf("Execute(apply) = %v, want nil", err)
	}
	if got != "" {
		t.Errorf("apply path = %q, want empty", got)
	}

	if err := withArgs(t, "apply", "/walls/a.png"); err != nil {
		t.Errorf("Execute(apply path) = %v, want nil", err)
	}
	if got != "/walls/a.png" {
		t.Errorf("apply path = %q, want /walls/a.png", got)
	}
}

// TestExecuteVersion checks the only flag: it prints and exits cleanly.
func TestExecuteVersion(t *testing.T) {
	if err := withArgs(t, "--version"); err != nil {
		t.Errorf("Execute(--version) = %v, want nil", err)
	}
}
