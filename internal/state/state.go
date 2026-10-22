// Package state persists the current wallpaper path and serializes apply
// through a file lock. The on-disk layout is <state>/current (atomic write,
// mode 0600), <state>/apply.lock (flock) and <state>/swaybg.pid.
package state

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jdcasdev/walltui/internal/errs"
)

// Get returns the stored wallpaper path; ("", nil) when unset. A missing
// state is a normal condition, not an error.
func Get(stateFile string) (string, error) {
	data, err := os.ReadFile(stateFile)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("%w: cannot read state file %s: %v", errs.ErrIntegration, stateFile, err)
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}

// Set atomically stores the canonical path: temp file + rename so readers
// never observe a partial write.
func Set(stateFile, canonical string) error {
	if canonical == "" {
		return fmt.Errorf("%w: empty wallpaper path", errs.ErrValidation)
	}
	dir := filepath.Dir(stateFile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("%w: cannot create state dir %s: %v", errs.ErrIntegration, dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".current.")
	if err != nil {
		return fmt.Errorf("%w: cannot create temp state file: %v", errs.ErrIntegration, err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("%w: cannot chmod state file: %v", errs.ErrIntegration, err)
	}
	if _, err := tmp.WriteString(canonical + "\n"); err != nil {
		tmp.Close()
		return fmt.Errorf("%w: cannot write state file: %v", errs.ErrIntegration, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("%w: cannot close state file: %v", errs.ErrIntegration, err)
	}
	if err := os.Rename(tmpName, stateFile); err != nil {
		return fmt.Errorf("%w: cannot replace state file: %v", errs.ErrIntegration, err)
	}
	committed = true
	return nil
}

func Clear(stateFile string) error {
	if err := os.Remove(stateFile); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: cannot remove state file %s: %v", errs.ErrIntegration, stateFile, err)
	}
	return nil
}
