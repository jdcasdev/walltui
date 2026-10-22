package trash

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGio writes an executable stand-in for `gio trash -- <path>` that fails
// for any path containing reject with a message on stderr.
func fakeGio(t *testing.T, reject string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "gio")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" != trash ] || [ \"$2\" != -- ]; then echo bad invocation >&2; exit 2; fi\n" +
		"case \"$3\" in\n" +
		"  *" + reject + "*) echo 'Unable to trash: locked' >&2; exit 1;;\n" +
		"esac\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestRun(t *testing.T) {
	bin := fakeGio(t, "bad")

	removed, failed := Run([]string{"/walls/a.png", "/walls/bad.png"}, bin)

	if len(removed) != 1 || removed[0] != "a.png" {
		t.Errorf("removed = %v, want [a.png]", removed)
	}
	if len(failed) != 1 || !strings.Contains(failed["/walls/bad.png"], "Unable to trash") {
		t.Errorf("failed = %v, want bad.png with gio's reason", failed)
	}
}

func TestRunAllFail(t *testing.T) {
	bin := fakeGio(t, "png") // every path contains "png"

	removed, failed := Run([]string{"/walls/a.png"}, bin)
	if removed != nil {
		t.Errorf("removed = %v, want none", removed)
	}
	if len(failed) != 1 || failed["/walls/a.png"] == "" {
		t.Errorf("failed = %v, want one reason", failed)
	}
}

func TestRunMissingBinary(t *testing.T) {
	removed, failed := Run([]string{"/walls/a.png"}, "/nonexistent/gio")
	if removed != nil || len(failed) != 1 || !strings.Contains(failed["/walls/a.png"], "no such file") {
		t.Errorf("removed=%v failed=%v, want exec failure reason", removed, failed)
	}
}

func TestRunGioNotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // LookPath fails → bin stays empty

	removed, failed := Run([]string{"/walls/a.png"}, "")
	if removed != nil || len(failed) != 1 || failed["/walls/a.png"] != "gio not found" {
		t.Errorf("removed=%v failed=%v, want gio not found", removed, failed)
	}
}
