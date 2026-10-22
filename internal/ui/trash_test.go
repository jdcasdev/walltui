package ui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jdcasdev/walltui/internal/config"
	"github.com/jdcasdev/walltui/internal/library"
	"github.com/jdcasdev/walltui/internal/state"
)

// fakeTrash returns a trashFn that claims every path was trashed.
func fakeTrash(paths []string) ([]string, map[string]string) {
	var removed []string
	for _, p := range paths {
		removed = append(removed, filepath.Base(p))
	}
	return removed, map[string]string{}
}

func TestTrashTargets(t *testing.T) {
	m := testModel(true, 110, 20) // entries: a.png …, cursor on a.png

	if got := m.trashTargets(); len(got) != 1 || got[0].Rel != "a.png" {
		t.Errorf("cursor targets = %v, want [a.png]", got)
	}

	m.list.marks["/walls/c.png"] = true
	m.list.marks["/walls/sub/b.jpg"] = true
	got := m.trashTargets()
	if len(got) != 3 || got[0].Rel != "a.png" || got[1].Rel != "sub/b.jpg" || got[2].Rel != "c.png" {
		t.Errorf("marked targets = %v, want the three marks in library order", got)
	}

	empty := testModel(true, 110, 20)
	empty.list.setLibrary(nil)
	if got := empty.trashTargets(); got != nil {
		t.Errorf("empty library targets = %v, want nil", got)
	}
}

func TestTrashConfirmKeys(t *testing.T) {
	m := testModel(true, 110, 20)
	m.trashFn = fakeTrash

	// d opens the dialog on the cursor entry. (Update may return a preview
	// sync cmd after every key — only state matters here.)
	ret, _ := m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = ret.(Model)
	if len(m.confirm) != 1 || m.confirm[0].Rel != "a.png" {
		t.Fatalf("d: confirm=%v", m.confirm)
	}

	// Unrelated keys are swallowed: dialog stays, list does not move.
	ret, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = ret.(Model)
	if len(m.confirm) != 1 || m.list.cursor != 0 {
		t.Error("j should be swallowed while the dialog is open")
	}

	// n cancels.
	ret, _ = m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	m = ret.(Model)
	if m.confirm != nil {
		t.Error("n should cancel the dialog")
	}

	// d again with marks: the dialog targets every mark (testModel pre-marks
	// a.png, so three entries qualify).
	m.list.marks["/walls/c.png"] = true
	m.list.marks["/walls/sub/b.jpg"] = true
	ret, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = ret.(Model)
	if len(m.confirm) != 3 {
		t.Fatalf("d with marks: confirm = %v, want 3 targets", m.confirm)
	}

	// y confirms: dialog closes, trashing starts.
	ret, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	m = ret.(Model)
	if cmd == nil || m.confirm != nil || !m.trashing {
		t.Fatalf("y: cmd=%v confirm=%v trashing=%v", cmd == nil, m.confirm, m.trashing)
	}
}

func TestTrashBusyGuards(t *testing.T) {
	m := testModel(true, 110, 20)
	m.applying = true

	ret, _ := m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if ret.(Model).confirm != nil {
		t.Error("d while applying should be ignored")
	}

	m = testModel(true, 110, 20)
	m.trashing = true
	ret, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if ret.(Model).applying {
		t.Error("enter while trashing should be ignored")
	}
}

func TestTrashRunFlow(t *testing.T) {
	m := testModel(true, 110, 20)
	m.trashFn = fakeTrash

	targets := m.trashTargets()
	msg := m.trashCmd(targets)().(trashedMsg)
	if len(msg.removed) != 1 || msg.removed[0] != "a.png" || msg.gone {
		t.Fatalf("trashCmd: %+v", msg)
	}

	ret, _ := m.Update(msg)
	m = ret.(Model)
	if m.trashing {
		t.Error("trashing stuck after done")
	}
	if m.busy != "" || m.err != "" {
		t.Errorf("busy = %q, err = %q; want a clean finish", m.busy, m.err)
	}
}

func TestTrashNothing(t *testing.T) {
	m := testModel(true, 110, 20)
	m.trashing = true

	ret, _ := m.Update(trashedMsg{failed: map[string]string{"/walls/a.png": "gio not found"}})
	m = ret.(Model)
	if !strings.HasPrefix(m.err, "nothing trashed: ") {
		t.Errorf("err = %q, want nothing-trashed failure", m.err)
	}
}

func TestTrashCurrentTriggersFallback(t *testing.T) {
	m := testModel(true, 110, 20)
	m.cfg.StateFile = filepath.Join(t.TempDir(), "current")
	m.current = "/walls/a.png" // the cursor entry is the applied wallpaper
	m.trashFn = fakeTrash

	applied := false
	m.applyFn = func(cfg config.Config, _ library.Entry) error {
		applied = true
		return state.Set(cfg.StateFile, "/walls/z.png") // fallback picks z.png
	}

	e, _ := m.list.current()
	msg := m.trashCmd([]library.Entry{e})().(trashedMsg)
	if !msg.gone || msg.fallback != "/walls/z.png" {
		t.Fatalf("trashCmd: gone=%v fallback=%q, want gone with z.png", msg.gone, msg.fallback)
	}

	ret, _ := m.Update(msg)
	m = ret.(Model)
	if !applied {
		t.Error("fallback apply not invoked")
	}
	if m.current != "/walls/z.png" {
		t.Errorf("current = %q, want fallback /walls/z.png", m.current)
	}
	if m.err != "" {
		t.Errorf("err = %q, want empty after a successful fallback", m.err)
	}
}

func TestTrashCurrentNoFallback(t *testing.T) {
	m := testModel(true, 110, 20)
	m.cfg.StateFile = filepath.Join(t.TempDir(), "current")
	m.current = "/walls/a.png"
	m.trashFn = fakeTrash
	m.applyFn = func(config.Config, library.Entry) error { return errors.New("no wallpapers left") }

	e, _ := m.list.current()
	msg := m.trashCmd([]library.Entry{e})().(trashedMsg)
	if !msg.gone || msg.fallback != "" {
		t.Fatalf("trashCmd: gone=%v fallback=%q", msg.gone, msg.fallback)
	}

	ret, _ := m.Update(msg)
	m = ret.(Model)
	if m.current != "" {
		t.Errorf("current = %q, want cleared", m.current)
	}
	if !strings.Contains(m.err, "no fallback") {
		t.Errorf("err = %q, want the no-fallback notice", m.err)
	}
}
