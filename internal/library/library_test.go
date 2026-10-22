package library

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jdcasdev/walltui/internal/errs"
)

// write creates a file (and parents) with content "x".
func write(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// testExts are the extensions the library tests accept.
var testExts = []string{".png", ".jpg", ".jpeg", ".webp"}

func names(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Rel
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestScan(t *testing.T) {
	t.Run("filters, recurses, sorts naturally", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, "10.png"))
		write(t, filepath.Join(dir, "2.png"))
		write(t, filepath.Join(dir, "1.JPG"))       // case-insensitive ext
		write(t, filepath.Join(dir, "notes.txt"))   // wrong ext
		write(t, filepath.Join(dir, ".hidden.png")) // dotfile
		write(t, filepath.Join(dir, "sub", "a-10.webp"))
		write(t, filepath.Join(dir, "sub", "a-2.webp"))
		write(t, filepath.Join(dir, ".git", "x.png")) // dot dir skipped
		if err := os.MkdirAll(filepath.Join(dir, "photo.png"), 0o700); err != nil {
			t.Fatal(err)
		} // dir named like an image

		entries, err := Scan(dir, testExts)
		if err != nil {
			t.Fatalf("Scan() = %v", err)
		}
		want := []string{"1.JPG", "2.png", "10.png", "sub/a-2.webp", "sub/a-10.webp"}
		if !equalStrings(names(entries), want) {
			t.Errorf("Scan() = %v, want %v", names(entries), want)
		}
	})

	t.Run("missing dir", func(t *testing.T) {
		_, err := Scan(filepath.Join(t.TempDir(), "nope"), testExts)
		if !errors.Is(err, errs.ErrValidation) {
			t.Errorf("Scan() = %v, want validation error", err)
		}
	})

	t.Run("unreadable folder does not hide the rest", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads every directory")
		}
		dir := t.TempDir()
		write(t, filepath.Join(dir, "ok.png"))
		locked := filepath.Join(dir, "locked")
		if err := os.Mkdir(locked, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(locked, 0o700) })

		entries, err := Scan(dir, testExts)
		if err != nil {
			t.Fatalf("Scan() = %v, want the readable entries", err)
		}
		if want := []string{"ok.png"}; !equalStrings(names(entries), want) {
			t.Errorf("Scan() = %v, want %v", names(entries), want)
		}
	})

	t.Run("symlink inside kept, outside dropped", func(t *testing.T) {
		dir := t.TempDir()
		outside := write(t, filepath.Join(t.TempDir(), "out.png"))
		real := write(t, filepath.Join(dir, "real.png"))
		if err := os.Symlink(real, filepath.Join(dir, "alias.png")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(dir, "escape.png")); err != nil {
			t.Fatal(err)
		}

		entries, err := Scan(dir, testExts)
		if err != nil {
			t.Fatalf("Scan() = %v", err)
		}
		// alias.png resolves to real.png and dedupes with it.
		want := []string{"real.png"}
		if !equalStrings(names(entries), want) {
			t.Errorf("Scan() = %v, want %v", names(entries), want)
		}
	})

	t.Run("symlinked folder is not walked into", func(t *testing.T) {
		dir := t.TempDir()
		outside := t.TempDir()
		write(t, filepath.Join(outside, "hidden.png"))
		write(t, filepath.Join(dir, "inside.png"))
		if err := os.Symlink(outside, filepath.Join(dir, "linked")); err != nil {
			t.Fatal(err)
		}

		entries, err := Scan(dir, testExts)
		if err != nil {
			t.Fatalf("Scan() = %v", err)
		}
		if want := []string{"inside.png"}; !equalStrings(names(entries), want) {
			t.Errorf("Scan() = %v, want %v", names(entries), want)
		}
	})
}

func TestResolve(t *testing.T) {
	dir := t.TempDir()
	inside := write(t, filepath.Join(dir, "wall.png"))
	write(t, filepath.Join(dir, "sub", "deep.png"))
	brother := write(t, dir+"Evil.png") // prefix sibling
	outside := write(t, filepath.Join(t.TempDir(), "out.png"))
	_insideLink := write(t, filepath.Join(dir, "link-target.png"))
	if err := os.Symlink(_insideLink, filepath.Join(dir, "link.png")); err != nil {
		t.Fatal(err)
	}

	t.Run("abs path inside", func(t *testing.T) {
		e, err := Resolve(dir, inside)
		if err != nil {
			t.Fatalf("Resolve() = %v", err)
		}
		if e.Abs != inside || e.Rel != "wall.png" {
			t.Errorf("Resolve() = %+v, want Abs=%s Rel=wall.png", e, inside)
		}
	})

	t.Run("relative input", func(t *testing.T) {
		t.Chdir(dir)
		e, err := Resolve(dir, filepath.Join("sub", "deep.png"))
		if err != nil {
			t.Fatalf("Resolve(%q) = %v", "sub/deep.png", err)
		}
		if e.Rel != "sub/deep.png" {
			t.Errorf("Rel = %q, want sub/deep.png", e.Rel)
		}
	})

	t.Run("symlink inside resolves to target", func(t *testing.T) {
		e, err := Resolve(dir, filepath.Join(dir, "link.png"))
		if err != nil {
			t.Fatalf("Resolve() = %v", err)
		}
		if e.Abs != _insideLink {
			t.Errorf("Abs = %q, want %q", e.Abs, _insideLink)
		}
	})

	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"missing", filepath.Join(dir, "ghost.png")},
		{"outside file", outside},
		{"prefix sibling", brother},
		{"the dir itself", dir},
		{"a directory inside", filepath.Join(dir, "sub")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Resolve(dir, tt.input); !errors.Is(err, errs.ErrValidation) {
				t.Errorf("Resolve(%q) = %v, want validation error", tt.input, err)
			}
		})
	}
}

func TestResolveSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	outside := write(t, filepath.Join(t.TempDir(), "out.png"))
	if err := os.Symlink(outside, filepath.Join(dir, "escape.png")); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(dir, filepath.Join(dir, "escape.png")); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("Resolve() = %v, want validation error", err)
	}
}

func TestNaturalCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int // sign
	}{
		{"2.png", "10.png", -1},
		{"10.png", "2.png", 1},
		{"a-2", "a-10", -1},
		{"a-10", "a-2", 1},
		{"a2", "a10", -1},
		{"1.png", "1.png", 0},
		{"x", "x2", -1},
		{"007", "7", 0},
		{"a", "b", -1},
		{"B", "a", -1},
	}
	for _, tt := range tests {
		got := naturalCompare(tt.a, tt.b)
		sign := 0
		switch {
		case got < 0:
			sign = -1
		case got > 0:
			sign = 1
		}
		if sign != tt.want {
			t.Errorf("naturalCompare(%q, %q) = %d, want sign %d", tt.a, tt.b, got, tt.want)
		}
	}
}
