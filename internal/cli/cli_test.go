package cli

import (
	"errors"
	"os"
	"testing"

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

// TestExecuteRejectsArguments guards the CLI surface: the picker takes no
// positional arguments, so a forgotten verb is a loud usage error instead of
// a silently ignored word.
func TestExecuteRejectsArguments(t *testing.T) {
	for _, arg := range []string{"apply", "current", "/walls/a.png"} {
		if err := withArgs(t, arg); !errors.Is(err, errs.ErrUsage) {
			t.Errorf("Execute(%q) = %v, want usage error", arg, err)
		}
	}
}

// TestExecuteVersion checks the only flag: it prints and exits cleanly.
func TestExecuteVersion(t *testing.T) {
	if err := withArgs(t, "--version"); err != nil {
		t.Errorf("Execute(--version) = %v, want nil", err)
	}
}
