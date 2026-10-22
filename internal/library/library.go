// Package library enumerates wallpapers and validates paths. Read-only: it
// never mutates the library tree. Paths are canonical (symlinks resolved) and
// every accepted path is proven to live strictly inside the library root.
package library

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jdcasdev/walltui/internal/errs"
)

// Entry is one wallpaper. Abs is the canonical absolute path (what apply and
// trash consume); Rel is the display path relative to the library root; Size
// is the file size in bytes (0 when unknown).
type Entry struct {
	Abs  string
	Rel  string
	Size int64
}

// Scan walks dir recursively and returns every file with one of the given
// extensions, natural-sorted by relative path. Dotfiles and dot-directories
// are skipped. Symlinked files are followed one level: a link resolving inside
// the library appears under its canonical path (deduplicated); links escaping
// the tree are dropped. Symlinked folders are not walked into.
func Scan(dir string, exts []string) ([]Entry, error) {
	root, err := canonicalDir(dir)
	if err != nil {
		return nil, err
	}
	accepted := acceptedExts(exts)

	var entries []Entry
	seen := make(map[string]bool)
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err // the library root itself is unreadable
			}
			if d != nil && d.IsDir() {
				return filepath.SkipDir // one unreadable folder must not hide the rest
			}
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") && path != root {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !accepted[strings.ToLower(filepath.Ext(name))] {
			return nil
		}
		if !d.Type().IsRegular() && d.Type()&fs.ModeSymlink == 0 {
			return nil
		}
		canonical, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil
		}
		rel, ok := containment(root, canonical)
		if !ok || seen[canonical] {
			return nil
		}
		seen[canonical] = true
		var size int64
		if info, err := os.Stat(canonical); err == nil {
			size = info.Size()
		}
		entries = append(entries, Entry{Abs: canonical, Rel: rel, Size: size})
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("scan %s: %w", dir, walkErr)
	}

	slices.SortFunc(entries, func(a, b Entry) int {
		return naturalCompare(a.Rel, b.Rel)
	})
	return entries, nil
}

// Resolve validates input against the library root and returns the canonical
// entry. Any rejection is a validation error naming the reason; input outside
// the library never passes.
func Resolve(dir, input string) (Entry, error) {
	if input == "" {
		return Entry{}, fmt.Errorf("%w: path required", errs.ErrValidation)
	}

	abs, err := filepath.Abs(input)
	if err != nil {
		return Entry{}, fmt.Errorf("%w: cannot resolve path: %s", errs.ErrValidation, input)
	}

	info, err := os.Stat(abs)
	switch {
	case err != nil:
		return Entry{}, fmt.Errorf("%w: not found: %s", errs.ErrValidation, input)
	case !info.Mode().IsRegular():
		return Entry{}, fmt.Errorf("%w: not a regular file: %s", errs.ErrValidation, input)
	}

	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return Entry{}, fmt.Errorf("%w: cannot resolve path: %s", errs.ErrValidation, input)
	}

	root, err := canonicalDir(dir)
	if err != nil {
		return Entry{}, err
	}
	rel, ok := containment(root, canonical)
	if !ok {
		return Entry{}, fmt.Errorf("%w: not inside wallpaper library: %s", errs.ErrValidation, input)
	}
	var size int64
	if info, err := os.Stat(canonical); err == nil {
		size = info.Size()
	}
	return Entry{Abs: canonical, Rel: rel, Size: size}, nil
}

// containment reports whether canonical lives strictly inside the canonical
// root. The boundary-aware filepath.Rel check rejects both the directory
// itself and prefix-sibling escapes such as /walls vs /wallsEvil.
func containment(root, canonical string) (string, bool) {
	rel, err := filepath.Rel(root, canonical)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func canonicalDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("%w: cannot resolve dir: %s", errs.ErrValidation, dir)
	}
	root, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("%w: wallpaper dir not found: %s", errs.ErrValidation, dir)
	}
	return root, nil
}

func acceptedExts(exts []string) map[string]bool {
	out := make(map[string]bool, len(exts))
	for _, e := range exts {
		out[strings.ToLower(e)] = true
	}
	return out
}

// naturalCompare orders like GNU `sort -V`: embedded digit runs compare
// numerically so a-2 < a-10, everything else byte-wise.
func naturalCompare(a, b string) int {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		da, db := isDigit(a[i]), isDigit(b[j])
		switch {
		case da && db:
			zi, zj := i, j
			for i < len(a) && isDigit(a[i]) {
				i++
			}
			for j < len(b) && isDigit(b[j]) {
				j++
			}
			na := strings.TrimLeft(a[zi:i], "0")
			nb := strings.TrimLeft(b[zj:j], "0")
			if len(na) != len(nb) {
				return len(na) - len(nb)
			}
			if c := strings.Compare(na, nb); c != 0 {
				return c
			}
		default:
			if c := strings.Compare(a[i:i+1], b[j:j+1]); c != 0 {
				return c
			}
			i++
			j++
		}
	}
	switch {
	case i >= len(a) && j >= len(b):
		return 0
	case i >= len(a):
		return -1
	default:
		return 1
	}
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
