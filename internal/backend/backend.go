// Package backend applies wallpapers through a desktop tool.
// One implementation per tool.
package backend

import "path/filepath"

// Backend sets a wallpaper.
type Backend interface {
	// Name identifies the tool behind the backend.
	Name() string
	// Formats are the image extensions the tool can draw, dot included.
	Formats() []string
	// Apply sets path as the wallpaper, scaled by mode. It fails loudly:
	// a missing tool or a wallpaper that never went up is an error.
	Apply(path, mode string) error
}

// Default returns the backend used when none is chosen.
func Default(stateDir string) Backend {
	if stateDir == "" {
		return Swaybg{}
	}
	return Swaybg{PidFile: filepath.Join(stateDir, "swaybg.pid")}
}
