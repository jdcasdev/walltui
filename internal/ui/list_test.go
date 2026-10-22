package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/jdcasdev/walltui/internal/library"
)

// testEntries builds n deterministic entries: a.png, sub/b.jpg, c.png…
// Sizes increase with the index so the metadata column has something to show.
func testEntries(n int) []library.Entry {
	names := []string{"a.png", "sub/b.jpg", "c.png", "d.jpg", "e.png", "f.png", "g.png", "h.jpg", "i.png", "j.png"}
	entries := make([]library.Entry, 0, n)
	for i, name := range names[:min(n, len(names))] {
		entries = append(entries, library.Entry{
			Abs:  "/walls/" + name,
			Rel:  name,
			Size: int64(i+1) * 1024 * 512, // 512K, 1.0M, 1.5M…
		})
	}
	return entries
}

func newTestList(t *testing.T, n, height int) *list {
	t.Helper()
	l := &list{marks: map[string]bool{}}
	l.setLibrary(testEntries(n))
	l.setHeight(height)
	return l
}

func TestListMovement(t *testing.T) {
	l := newTestList(t, 10, 3)

	for i := 0; i < 5; i++ {
		l.move(-1)
	}
	if l.cursor != 0 || l.offset != 0 {
		t.Errorf("clamped at top: cursor=%d offset=%d", l.cursor, l.offset)
	}

	l.move(1)
	if l.cursor != 1 {
		t.Errorf("cursor = %d, want 1", l.cursor)
	}

	l.end()
	if l.cursor != 9 || l.offset != 7 {
		t.Errorf("end: cursor=%d offset=%d, want 9/7", l.cursor, l.offset)
	}

	l.home()
	if l.cursor != 0 || l.offset != 0 {
		t.Errorf("home: cursor=%d offset=%d, want 0/0", l.cursor, l.offset)
	}

	l.page(1)
	if l.cursor != 2 {
		t.Errorf("page down: cursor = %d, want 2", l.cursor)
	}
	l.page(-1)
	if l.cursor != 0 {
		t.Errorf("page up: cursor = %d, want 0", l.cursor)
	}
}

func TestListClampOnShrink(t *testing.T) {
	l := newTestList(t, 10, 3)
	l.end()
	l.setLibrary(testEntries(2)) // library shrank under the cursor
	if l.cursor != 1 {
		t.Errorf("cursor = %d, want clamped to 1", l.cursor)
	}
}

func TestListMarks(t *testing.T) {
	l := newTestList(t, 3, 3)

	l.toggleMark()
	l.move(1)
	l.toggleMark()
	if n := len(l.marked()); n != 2 {
		t.Errorf("marked() = %d entries, want 2", n)
	}

	l.home()
	l.toggleMark() // unmark first
	if n := len(l.marked()); n != 1 {
		t.Errorf("marked() = %d entries after unmark, want 1", n)
	}

	// marks reset with the library
	l.setLibrary(testEntries(3))
	if n := len(l.marked()); n != 0 {
		t.Errorf("marks survived setEntries: %d", n)
	}
}

func TestListEmpty(t *testing.T) {
	l := &list{marks: map[string]bool{}}
	l.setHeight(5)
	if _, ok := l.current(); ok {
		t.Error("current() on empty list should report false")
	}
	l.toggleMark() // must not panic
	l.end()
	l.move(1)
	// five empty lines, each painted the full panel width
	lines := strings.Split(ansi.Strip(l.render(20, 5, "", nil, newStyles(true))), "\n")
	if len(lines) != 5 {
		t.Fatalf("render() = %d lines, want 5", len(lines))
	}
	for i, line := range lines {
		if line != strings.Repeat(" ", 20) {
			t.Errorf("line %d = %q, want 20 blanks", i, line)
		}
	}
}

func TestListRender(t *testing.T) {
	s := newStyles(true)
	meta := func(e library.Entry) string { return "1.2M" }

	t.Run("cursor mark current", func(t *testing.T) {
		l := newTestList(t, 3, 3)
		l.move(1)
		l.marks["/walls/c.png"] = true
		golden.RequireEqual(t, ansi.Strip(l.render(30, 3, "/walls/a.png", meta, s)))
	})

	t.Run("scrolled viewport", func(t *testing.T) {
		l := newTestList(t, 10, 3)
		l.end()
		golden.RequireEqual(t, ansi.Strip(l.render(30, 3, "", meta, s)))
	})

	t.Run("truncation", func(t *testing.T) {
		l := newTestList(t, 2, 2)
		l.marks["/walls/sub/b.jpg"] = true
		golden.RequireEqual(t, ansi.Strip(l.render(12, 2, "", nil, s)))
	})
}
