package ui

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/jdcasdev/walltui/internal/config"
)

// testModel builds a deterministic model with an injected library.
func testModel(isDark bool, w, h int) Model {
	cfg := config.Config{
		WallpaperDir: "/walls",
		FitMode:      "fill",
		StateFile:    "/state/current",
		LockFile:     "/state/apply.lock",
	}
	m := New(cfg)
	m.isDark = isDark
	m.styles = newStyles(isDark)
	m.help.Styles = m.styles.helpStyles()
	m.width, m.height = w, h
	m.help.SetWidth(w)

	m.list.setLibrary(testEntries(5))
	m.list.setHeight(m.listRows())
	m.list.marks["/walls/a.png"] = true
	m.current = "/walls/sub/b.jpg"
	return m
}

func TestView(t *testing.T) {
	for _, tt := range []struct {
		name   string
		isDark bool
		w, h   int
	}{
		{"dark", true, 110, 20},
		{"light", false, 110, 20},
		{"narrow", true, 80, 12},
		{"narrowlight", false, 80, 12},
		{"status", true, 110, 20},
		{"empty", true, 110, 20},
		{"noresults", true, 110, 20},
		{"search", true, 110, 20},
		{"busy", true, 110, 20},
		{"error", true, 110, 20},
		{"confirm", true, 110, 20},
		{"confirmmulti", true, 110, 20},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := testModel(tt.isDark, tt.w, tt.h)
			switch tt.name {
			case "status":
				m.status = "permission denied: /walls"
			case "empty":
				m.list.setLibrary(nil)
			case "noresults":
				m.searching = true
				m.query = "zz"
				m.list.setQuery("zz")
			case "search":
				m.searching = true
				m.query = "sub"
				m.list.setQuery("sub")
			case "busy":
				m.applying = true
				m.busy = "applying sub/b.jpg"
				m.spinner, _ = m.spinner.Update(m.spinner.Tick()) // advance a frame
			case "error":
				m.err = "apply failed: swaybg died"
			case "confirm":
				m.confirm = m.trashTargets()
			case "confirmmulti":
				m.list.marks["/walls/a.png"] = true
				m.list.marks["/walls/c.png"] = true
				m.confirm = m.trashTargets()
			}
			golden.RequireEqual(t, ansi.Strip(m.View().Content))
		})
	}
}

// TestKeyboard drives the real event loop: navigation, marking, and quit.
func TestKeyboard(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.png", "b.png", "c.png"} {
		writeFile(t, filepath.Join(dir, name))
	}
	m := New(config.Config{WallpaperDir: dir, FitMode: "fill"})

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(110, 20),
		teatest.WithProgramOptions(tea.WithColorProfile(colorprofile.ASCII)),
	)
	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool { return bytes.Contains(b, []byte("a.png")) },
		teatest.WithDuration(5*time.Second),
	)

	tm.Type("j") // b.png
	tm.Type(" ") // mark b.png
	tm.Type("G") // last: c.png
	// The mark shows up as the ✓ chip on the row.
	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool { return bytes.Contains(b, []byte("✓")) },
		teatest.WithDuration(5*time.Second),
	)

	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	final := tm.FinalModel(t).(Model) // blocks until the program exits
	if final.list.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (last entry)", final.list.cursor)
	}
	if n := len(final.list.marked()); n != 1 {
		t.Errorf("marked = %d, want 1", n)
	}
}

// TestKeyDismissesError: the error card must not stay on screen forever.
func TestKeyDismissesError(t *testing.T) {
	m := testModel(true, 110, 20)
	m.err = "apply failed: swaybg died"

	m = press(m, tea.KeyPressMsg{Code: 'j'})
	if m.err != "" {
		t.Errorf("err = %q, want cleared by a key press", m.err)
	}
}

// TestSearchLoop drives search through the real event loop: open, type,
// see the filtered count, cancel, and quit. It never sends enter (that would
// apply for real — apply paths are covered by injected-fn unit tests).
func TestSearchLoop(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.png", "b.png", "c.png"} {
		writeFile(t, filepath.Join(dir, name))
	}
	m := New(config.Config{WallpaperDir: dir, FitMode: "fill"})

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(110, 20),
		teatest.WithProgramOptions(tea.WithColorProfile(colorprofile.ASCII)),
	)
	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool { return bytes.Contains(b, []byte("a.png")) },
		teatest.WithDuration(5*time.Second),
	)

	tm.Type("s") // search mode
	tm.Type("b") // filter: b.png only
	tm.Type(" ") // space is text in search mode
	// The footer's search-mode hint is rewritten in full when the mode opens:
	// stable regardless of how the keystroke frames coalesce. (The key glyph
	// carries bold, so only the description is contiguous in the byte stream.)
	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool { return bytes.Contains(b, []byte("back to select")) },
		teatest.WithDuration(5*time.Second),
	)

	// esc cancels; it is queued before ctrl+c, so FinalModel sees the
	// cancelled state.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEsc})
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	final := tm.FinalModel(t).(Model)
	if final.searching || final.query != "" || final.list.query != "" {
		t.Errorf("left over: searching=%v query=%q filter=%q",
			final.searching, final.query, final.list.query)
	}
}

// TestTrashLoop opens the trash dialog through the real event loop, cancels
// twice, and quits. The destructive y path uses injected fakes in
// trash_test.go — never gio on the real library.
func TestTrashLoop(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.png", "b.png", "c.png"} {
		writeFile(t, filepath.Join(dir, name))
	}
	m := New(config.Config{WallpaperDir: dir, FitMode: "fill"})

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(110, 20),
		teatest.WithProgramOptions(tea.WithColorProfile(colorprofile.ASCII)),
	)
	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool { return bytes.Contains(b, []byte("a.png")) },
		teatest.WithDuration(5*time.Second),
	)

	tm.Type("d") // dialog on the cursor entry
	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool { return bytes.Contains(b, []byte("trash a.png?")) },
		teatest.WithDuration(5*time.Second),
	)

	tm.Type("n")                               // cancel
	tm.Type("d")                               // reopen
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEsc}) // cancel again
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	final := tm.FinalModel(t).(Model)
	if final.confirm != nil || final.trashing {
		t.Errorf("left over: confirm=%v trashing=%v", final.confirm, final.trashing)
	}
	if n := len(final.list.marked()); n != 0 {
		t.Errorf("marks = %d, want 0", n)
	}
}
