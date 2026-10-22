package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jdcasdev/walltui/internal/library"
	"github.com/jdcasdev/walltui/internal/state"
	"github.com/jdcasdev/walltui/internal/trash"
)

// trashedMsg reports one finished trash run.
type trashedMsg struct {
	removed  []string          // basenames that went
	failed   map[string]string // abs path → reason
	gone     bool              // the applied wallpaper was among the removed
	fallback string            // abs path the fallback apply produced, if any
}

func (m Model) trashTargets() []library.Entry {
	if marked := m.list.marked(); len(marked) > 0 {
		return marked
	}
	if e, ok := m.list.current(); ok {
		return []library.Entry{e}
	}
	return nil
}

func defaultTrashFn(paths []string) (removed []string, failed map[string]string) {
	return trash.Run(paths, "")
}

// trashCmd trashes off the UI goroutine. If the current wallpaper goes, the
// state is cleared and a fallback applied in the same command.
func (m Model) trashCmd(targets []library.Entry) tea.Cmd {
	fn, cfg := m.trashFn, m.cfg
	current := m.current
	return func() tea.Msg {
		paths := make([]string, len(targets))
		for i, e := range targets {
			paths[i] = e.Abs
		}
		removed, failed := fn(paths)
		msg := trashedMsg{removed: removed, failed: failed}

		for _, e := range targets {
			if e.Abs == current && failed[e.Abs] == "" {
				msg.gone = true
				_ = state.Clear(cfg.StateFile)
				if err := m.applyFn(cfg, library.Entry{}); err == nil {
					msg.fallback, _ = state.Get(cfg.StateFile)
				}
				break
			}
		}
		return msg
	}
}

func (m Model) handleTrashed(msg trashedMsg) (Model, tea.Cmd) {
	m.trashing = false
	m.busy = ""
	if msg.gone {
		m.current = msg.fallback
	}
	if reason, failed := trashError(msg); failed {
		m.err = reason
	} else if msg.gone && msg.fallback == "" {
		m.err = "trashed the current wallpaper: no fallback"
	}
	if len(msg.removed) == 0 {
		return m, nil
	}
	return m, m.loadCmd()
}

func trashError(msg trashedMsg) (string, bool) {
	if len(msg.removed) == 0 {
		reason := "gio failed"
		for _, r := range msg.failed {
			reason = r
			break
		}
		return "nothing trashed: " + reason, true
	}
	if len(msg.failed) > 0 {
		return fmt.Sprintf("trashed %d, %d failed", len(msg.removed), len(msg.failed)), true
	}
	return "", false
}

func (m Model) renderConfirm() string {
	n := len(m.confirm)
	title := fmt.Sprintf("trash %d wallpapers?", n)
	note := fmt.Sprintf("%d files move to the system trash", n)
	if n == 1 {
		title = "trash " + m.confirm[0].Rel + "?"
		note = "moves to the system trash, recoverable"
	}

	width := clampInt(m.width-4, 24, 56)
	raised := m.styles.Raised
	actions := m.styles.Action.Render("[y] Move to trash") +
		raised.Render("   ") +
		raised.Foreground(m.styles.t.muted).Render("[n] Cancel")
	body := centerIn(width-2, 5, []string{
		raised.Foreground(m.styles.t.text).Bold(true).Render(title),
		"",
		raised.Foreground(m.styles.t.muted).Render(note),
		"",
		actions,
	}, m.styles.Raised)
	return frame(width, 7, "", "", body, m.styles.BorderDanger, m.styles.Raised)
}

func overlay(base, content string, width int) string {
	baseLines := strings.Split(base, "\n")
	contentLines := strings.Split(content, "\n")
	cw := ansi.StringWidth(contentLines[0])
	left := max((width-cw)/2, 0)
	top := (len(baseLines) - len(contentLines)) / 2

	out := make([]string, len(baseLines))
	for i, bl := range baseLines {
		if i < top || i >= top+len(contentLines) {
			out[i] = bl
			continue
		}
		out[i] = ansi.Truncate(bl, left, "") +
			contentLines[i-top] +
			ansi.TruncateLeft(bl, left+cw, "")
	}
	return strings.Join(out, "\n")
}

// updateConfirmKey: y/enter trashes, esc/n cancels, everything else is swallowed
// so the dialog cannot be lost by accident.
func (m Model) updateConfirmKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "y", "enter":
		targets := m.confirm
		m.confirm = nil
		m.trashing = true
		m.busy = fmt.Sprintf("trashing %d wallpapers", len(targets))
		m.err = ""
		return m, tea.Batch(m.trashCmd(targets), m.spinner.Tick)
	case "esc", "n":
		m.confirm = nil
	}
	return m, nil
}
