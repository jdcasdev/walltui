package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jdcasdev/walltui/internal/config"
	"github.com/jdcasdev/walltui/internal/library"
)

// press runs one key through the real Update loop.
func press(m Model, msg tea.KeyPressMsg) Model {
	ret, _ := m.Update(msg)
	return ret.(Model)
}

func textKey(s string) tea.KeyPressMsg {
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func TestListFilter(t *testing.T) {
	l := newTestList(t, 5, 5) // a.png, sub/b.jpg, c.png, d.jpg, e.png

	l.setQuery("B") // case-insensitive
	if len(l.matches) != 1 || l.matches[0].Rel != "sub/b.jpg" {
		t.Fatalf("filter B = %v, want [sub/b.jpg]", l.matches)
	}
	if l.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (first match)", l.cursor)
	}

	l.setQuery("png") // a.png, c.png, e.png
	if n := len(l.matches); n != 3 {
		t.Errorf("filter png = %d entries, want 3", n)
	}

	l.clearQuery()
	if n := len(l.matches); n != 5 {
		t.Errorf("clearFilter = %d entries, want 5", n)
	}
	if l.query != "" {
		t.Errorf("filter = %q after clear", l.query)
	}
}

func TestListMarksSurviveFilter(t *testing.T) {
	l := newTestList(t, 5, 5)
	l.toggleMark()      // a.png
	l.setQuery("e.png") // a.png hidden

	if n := len(l.marked()); n != 1 {
		t.Errorf("marked() = %d while filtered, want 1", n)
	}
	if cur, _ := l.current(); cur.Rel != "e.png" {
		t.Errorf("current() = %v, want the match", cur.Rel)
	}
}

func TestSearchModeKeys(t *testing.T) {
	m := testModel(true, 110, 20) // 5 entries

	m = press(m, tea.KeyPressMsg{Code: 's', Text: "s"})
	if !m.searching {
		t.Fatal("s should open search mode")
	}

	m = press(m, textKey("b")) // matches only sub/b.jpg
	if m.query != "b" || len(m.list.matches) != 1 {
		t.Fatalf("query=%q shown=%d, want b/1", m.query, len(m.list.matches))
	}
	if cur, _ := m.list.current(); cur.Rel != "sub/b.jpg" {
		t.Errorf("cursor on %v, want sub/b.jpg", cur.Rel)
	}

	// In search mode space is text, not a mark (fzf parity).
	m = press(m, tea.KeyPressMsg{Code: ' ', Text: " "})
	if m.query != "b " {
		t.Errorf("query = %q, want %q", m.query, "b ")
	}
	if n := len(m.list.marked()); n != 1 { // the pre-set a.png mark
		t.Errorf("marks = %d, space must not mark in search mode", n)
	}

	// Arrows still navigate the filtered list.
	m = press(m, tea.KeyPressMsg{Code: tea.KeyDown})
	if m.list.cursor != 0 { // only one match
		t.Errorf("cursor = %d, want 0", m.list.cursor)
	}

	// Backspace edits the query.
	m = press(m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.query != "b" {
		t.Errorf("query = %q after backspace, want b", m.query)
	}

	// esc cancels: full list, empty query.
	m = press(m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.searching || m.query != "" || len(m.list.matches) != 5 {
		t.Errorf("esc left searching=%v query=%q shown=%d", m.searching, m.query, len(m.list.matches))
	}
}

func TestSearchEscDoesNotQuit(t *testing.T) {
	m := testModel(true, 110, 20)
	m = press(m, tea.KeyPressMsg{Code: 's', Text: "s"})

	ret, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd != nil {
		t.Error("esc in search mode must not quit")
	}
	m = ret.(Model)
	if m.searching {
		t.Error("esc should leave search mode")
	}
	// Now esc quits from normal mode.
	if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc}); cmd == nil {
		t.Error("esc in normal mode should quit")
	}
}

func TestApplySelected(t *testing.T) {
	m := testModel(true, 110, 20)
	var applied []string
	m.applyFn = func(_ config.Config, e library.Entry) error {
		applied = append(applied, e.Abs)
		return nil
	}

	// Enter starts the apply and marks the model busy.
	ret, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = ret.(Model)
	if cmd == nil || !m.applying {
		t.Fatalf("enter: cmd=%v applying=%v", cmd == nil, m.applying)
	}

	// A second enter while busy is ignored (no double apply, no lock fight).
	ret, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	busy := ret.(Model)
	if cmd != nil || !busy.applying {
		t.Error("enter while applying should be a no-op")
	}

	// The apply command itself goes through the injected function.
	e, _ := m.list.current()
	msg := m.applyCmd(e)().(applyDoneMsg)
	if msg.err != nil {
		t.Fatalf("applyCmd: %v", msg.err)
	}
	if len(applied) != 1 || applied[0] != e.Abs {
		t.Errorf("applied = %v, want [%s]", applied, e.Abs)
	}

	// Completion: busy clears, current updates, nothing is announced.
	ret, tick := m.Update(msg)
	m = ret.(Model)
	if m.applying {
		t.Error("applying stuck after done")
	}
	if m.busy != "" {
		t.Errorf("busy = %q, want empty", m.busy)
	}
	if m.err != "" {
		t.Errorf("err = %q, want empty", m.err)
	}
	if m.current != e.Abs {
		t.Errorf("current = %q, want %q", m.current, e.Abs)
	}
	_ = tick
}

func TestApplyError(t *testing.T) {
	m := testModel(true, 110, 20)
	m.applying = true
	m.current = "/walls/sub/b.jpg"
	m.applyFn = func(config.Config, library.Entry) error {
		return errors.New("exit status 1")
	}

	e, _ := m.list.current()
	ret, _ := m.Update(m.applyCmd(e)())
	m = ret.(Model)
	if m.applying {
		t.Error("applying stuck after failure")
	}
	if m.current != "/walls/sub/b.jpg" {
		t.Errorf("current changed on failure: %q", m.current)
	}
	if !strings.Contains(m.err, "apply failed") {
		t.Fatalf("err = %q, want an apply failure", m.err)
	}
}

func TestEnterAppliesThenExitsSearch(t *testing.T) {
	m := testModel(true, 110, 20)
	m.applyFn = func(config.Config, library.Entry) error { return nil }

	m = press(m, tea.KeyPressMsg{Code: 's', Text: "s"})
	m = press(m, textKey("sub"))
	ret, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = ret.(Model)
	if cmd == nil {
		t.Fatal("enter in search mode should apply")
	}
	if !m.applying {
		t.Error("enter should start the apply")
	}
	if m.searching || m.query != "" || m.list.query != "" {
		t.Errorf("enter should cancel search: searching=%v query=%q filter=%q",
			m.searching, m.query, m.list.query)
	}
}
