package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jdcasdev/walltui/internal/apply"
	"github.com/jdcasdev/walltui/internal/backend"
	"github.com/jdcasdev/walltui/internal/config"
	"github.com/jdcasdev/walltui/internal/library"
)

type applyDoneMsg struct {
	entry library.Entry
	err   error
}

// defaultApplyFn performs the actual apply. Tests inject a fake so no test ever
// touches the wallpaper tool or the state files.
func defaultApplyFn(b backend.Backend) func(config.Config, library.Entry) error {
	return func(cfg config.Config, e library.Entry) error {
		_, err := apply.Run(cfg, e.Abs, apply.Options{Backend: b})
		return err
	}
}

// applyCmd applies one entry off the UI goroutine.
func (m Model) applyCmd(e library.Entry) tea.Cmd {
	fn, cfg := m.applyFn, m.cfg
	return func() tea.Msg {
		return applyDoneMsg{entry: e, err: fn(cfg, e)}
	}
}

func (m Model) applySelected() (Model, tea.Cmd) {
	e, ok := m.list.current()
	if !ok || m.applying || m.trashing {
		return m, nil
	}
	m.applying = true
	m.busy = "applying " + e.Rel
	m.err = ""
	return m, tea.Batch(m.applyCmd(e), m.spinner.Tick)
}
