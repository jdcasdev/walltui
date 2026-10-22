// Package apply turns "a wallpaper path" into "the wallpaper is on screen,
// state updated". The resolution order is: an explicit argument must be
// valid or the apply fails loudly; the stored state falls back to the first
// available library entry so a login never ends on the default wallpaper.
package apply

import (
	"fmt"
	"time"

	"github.com/jdcasdev/walltui/internal/backend"
	"github.com/jdcasdev/walltui/internal/config"
	"github.com/jdcasdev/walltui/internal/errs"
	"github.com/jdcasdev/walltui/internal/library"
	"github.com/jdcasdev/walltui/internal/state"
)

// Options tunes the apply. The zero value is production.
type Options struct {
	Backend     backend.Backend // wallpaper setter (default backend.Default())
	LockTimeout time.Duration   // apply.lock wait (default 10s)
}

// Run applies a wallpaper and returns the entry that became current.
func Run(cfg config.Config, arg string, o Options) (library.Entry, error) {
	if err := cfg.Validate(); err != nil {
		return library.Entry{}, err
	}
	o = o.withDefaults(cfg.StateDir)

	entry, err := resolveTarget(cfg, arg, o.Backend.Formats())
	if err != nil {
		return library.Entry{}, err
	}

	lock, err := state.Acquire(cfg.LockFile, o.LockTimeout)
	if err != nil {
		return library.Entry{}, err
	}
	defer lock.Release()

	if err := o.Backend.Apply(entry.Abs, cfg.FitMode); err != nil {
		return library.Entry{}, err
	}

	if err := state.Set(cfg.StateFile, entry.Abs); err != nil {
		return library.Entry{}, err
	}

	return entry, nil
}

// resolveTarget picks the wallpaper to apply: explicit arg (loud failure,
// never substituted) > stored state (stale state falls through) > first available.
func resolveTarget(cfg config.Config, arg string, exts []string) (library.Entry, error) {
	if arg != "" {
		return library.Resolve(cfg.WallpaperDir, arg)
	}

	// An unreadable state file behaves like an unset one.
	cur, _ := state.Get(cfg.StateFile)
	if cur != "" {
		if e, err := library.Resolve(cfg.WallpaperDir, cur); err == nil {
			return e, nil
		}
	}

	entries, err := library.Scan(cfg.WallpaperDir, exts)
	if err != nil {
		return library.Entry{}, err
	}
	if len(entries) == 0 {
		return library.Entry{}, fmt.Errorf("%w: no wallpapers in %s", errs.ErrNotFound, cfg.WallpaperDir)
	}
	return entries[0], nil
}

func (o Options) withDefaults(stateDir string) Options {
	if o.Backend == nil {
		o.Backend = backend.Default(stateDir)
	}
	if o.LockTimeout == 0 {
		o.LockTimeout = state.DefaultLockTimeout
	}
	return o
}
