// Package config resolves the wallpaper manager's runtime configuration from
// the environment: library directory, fit mode, and the XDG state layout.
// Load never touches the filesystem; Validate checks inputs on use.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/jdcasdev/walltui/internal/errs"
)

// Modes are the fit modes the backends understand.
var Modes = []string{"fill", "fit", "stretch", "center", "tile"}

// Config holds the resolved runtime configuration.
type Config struct {
	WallpaperDir string // image library root
	FitMode      string // fit mode
	StateDir     string // XDG state dir
	StateFile    string // current wallpaper path
	LockFile     string // apply lock
}

// Load resolves defaults and environment overrides. No filesystem access.
func Load() Config {
	stateDir := filepath.Join(xdgStateHome(), "walltui")
	return Config{
		WallpaperDir: envOr("WALLPAPER_DIR", filepath.Join(userHomeDir(), "Pictures", "Wallpapers")),
		FitMode:      envOr("WALLPAPER_MODE", "fill"),
		StateDir:     stateDir,
		StateFile:    filepath.Join(stateDir, "current"),
		LockFile:     filepath.Join(stateDir, "apply.lock"),
	}
}

// Validate checks the configuration and ensures the state dir exists.
func (c Config) Validate() error {
	info, err := os.Stat(c.WallpaperDir)
	switch {
	case err != nil:
		return fmt.Errorf("%w: wallpaper dir not found: %s", errs.ErrValidation, c.WallpaperDir)
	case !info.IsDir():
		return fmt.Errorf("%w: wallpaper dir is not a directory: %s", errs.ErrValidation, c.WallpaperDir)
	}
	if !slices.Contains(Modes, c.FitMode) {
		return fmt.Errorf("%w: invalid wallpaper mode: %s", errs.ErrValidation, c.FitMode)
	}
	if err := os.MkdirAll(c.StateDir, 0o700); err != nil {
		return fmt.Errorf("%w: cannot create state dir %s: %v", errs.ErrIntegration, c.StateDir, err)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func xdgStateHome() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v
	}
	return filepath.Join(userHomeDir(), ".local", "state")
}

func userHomeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "~" // surfaced as a validation error on use
}
