package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jdcasdev/walltui/internal/errs"
)

func TestLoadDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("WALLPAPER_DIR", "")
	t.Setenv("WALLPAPER_MODE", "")

	c := Load()
	if want := filepath.Join(home, "Pictures", "Wallpapers"); c.WallpaperDir != want {
		t.Errorf("WallpaperDir = %q, want %q", c.WallpaperDir, want)
	}
	if c.FitMode != "fill" {
		t.Errorf("Mode = %q, want fill", c.FitMode)
	}
	if want := filepath.Join(home, ".local", "state", "walltui"); c.StateDir != want {
		t.Errorf("StateDir = %q, want %q", c.StateDir, want)
	}
	if want := filepath.Join(c.StateDir, "current"); c.StateFile != want {
		t.Errorf("StateFile = %q, want %q", c.StateFile, want)
	}
	if want := filepath.Join(c.StateDir, "apply.lock"); c.LockFile != want {
		t.Errorf("LockFile = %q, want %q", c.LockFile, want)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("WALLPAPER_DIR", "/custom/walls")
	t.Setenv("WALLPAPER_MODE", "fit")
	t.Setenv("XDG_STATE_HOME", "/custom/state")

	c := Load()
	if c.WallpaperDir != "/custom/walls" {
		t.Errorf("WallpaperDir = %q, want /custom/walls", c.WallpaperDir)
	}
	if c.FitMode != "fit" {
		t.Errorf("Mode = %q, want fit", c.FitMode)
	}
	if want := filepath.Join("/custom/state", "walltui"); c.StateDir != want {
		t.Errorf("StateDir = %q, want %q", c.StateDir, want)
	}
}

func TestValidate(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("WALLPAPER_DIR", "")
	t.Setenv("WALLPAPER_MODE", "")

	t.Run("missing wallpaper dir", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		err := Load().Validate()
		if !errors.Is(err, errs.ErrValidation) {
			t.Errorf("Validate() = %v, want validation error", err)
		}
	})

	t.Run("dir is a file", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		c := Load()
		if err := os.MkdirAll(filepath.Dir(c.WallpaperDir), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(c.WallpaperDir, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := c.Validate(); !errors.Is(err, errs.ErrValidation) {
			t.Errorf("Validate() = %v, want validation error", err)
		}
	})

	t.Run("bad mode", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		c := Load()
		if err := os.MkdirAll(c.WallpaperDir, 0o700); err != nil {
			t.Fatal(err)
		}
		c.FitMode = "tilee"
		if err := c.Validate(); !errors.Is(err, errs.ErrValidation) {
			t.Errorf("Validate() = %v, want validation error", err)
		}
	})

	t.Run("ok creates state dir", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		c := Load()
		if err := os.MkdirAll(c.WallpaperDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
		info, err := os.Stat(c.StateDir)
		if err != nil || !info.IsDir() {
			t.Errorf("state dir %q was not created", c.StateDir)
		}
	})
}
