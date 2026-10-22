package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestFrame checks the box math: exactly width x height cells, whatever the
// content and titles are, and "" (not a broken frame) when there is no room.
func TestFrame(t *testing.T) {
	s := newStyles(true)
	long := strings.Repeat("content ", 40)

	for _, size := range []struct{ w, h int }{{20, 5}, {12, 3}, {60, 2}, {80, 12}, {2, 2}, {20, 1}} {
		out := frame(size.w, size.h, s.TitleL.Render("❯ Wallpapers"), "142", long, s.Border, s.Panel)
		if out == "" {
			if size.w >= 6 && size.h >= 2 {
				t.Errorf("frame(%d,%d) = \"\", want a box", size.w, size.h)
			}
			continue
		}
		lines := strings.Split(ansi.Strip(out), "\n")
		if len(lines) != size.h {
			t.Errorf("frame(%d,%d) = %d lines, want %d", size.w, size.h, len(lines), size.h)
		}
		for i, line := range lines {
			if got := ansi.StringWidth(line); got != size.w {
				t.Errorf("frame(%d,%d) line %d = %d cells, want %d: %q", size.w, size.h, i, got, size.w, line)
			}
		}
	}
}

func TestHumanSize(t *testing.T) {
	for _, tc := range []struct {
		n    int64
		want string
	}{
		{0, "0B"}, {512, "512B"}, {52 * 1024, "52K"}, {512 * 1024, "512K"},
		{1024 * 1024, "1.0M"}, {int64(2.5 * 1024 * 1024), "2.5M"},
		{13_317_000, "13M"}, {3 << 30, "3.0G"},
	} {
		if got := humanSize(tc.n); got != tc.want {
			t.Errorf("humanSize(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// TestTailCut: the search cursor sits at the end, so the tail is what stays.
func TestTailCut(t *testing.T) {
	for _, tc := range []struct {
		in    string
		width int
		want  string
	}{
		{"short", 10, "short"},
		{"abcdefgh", 5, "…efgh"},
		{"", 5, ""},
	} {
		if got := tailCut(tc.in, tc.width); got != tc.want {
			t.Errorf("tailCut(%q,%d) = %q, want %q", tc.in, tc.width, got, tc.want)
		}
	}
}
