// Package ui hosts the root tea.Model and the app-wide styles. The root is
// the only tea.Model; subcomponents (list, preview) are plain structs with
// imperative methods it calls directly.
package ui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jdcasdev/walltui/internal/backend"
	"github.com/jdcasdev/walltui/internal/config"
	"github.com/jdcasdev/walltui/internal/library"
	"github.com/jdcasdev/walltui/internal/state"
)

// Layout constants. Below narrowWidth the preview pane is dropped and the
// list takes the full width; panels are otherwise sized as proportions of
// the terminal, never as fixed cell counts.
const (
	narrowWidth = 100 // preview breakpoint
	gutter      = 2   // blank cells between the two panels
	listPercent = 44  // list share of the width
	listMin     = 30
	previewMin  = 42 // below this the preview is not worth drawing
)

// Model is the root tea.Model. Update is the only place state changes.
type Model struct {
	cfg       config.Config
	width     int
	height    int
	isDark    bool
	styles    Styles
	help      help.Model
	keymap    keymap
	list      list
	preview   preview
	spinner   spinner.Model
	backend   backend.Backend // wallpaper setter
	searching bool
	query     string
	applying  bool
	trashing  bool
	busy      string                                       // operation in flight, empty when idle
	confirm   []library.Entry                              // non-nil while the trash dialog is open
	applyFn   func(config.Config, library.Entry) error     // injected; see apply.go
	trashFn   func([]string) ([]string, map[string]string) // injected; see trash.go
	current   string                                       // canonical path of the applied wallpaper
	err       string                                       // last operation error
	status    string                                       // library scan error
}

// New returns the initial model; entries load asynchronously in Init.
func New(cfg config.Config) Model {
	b := backend.Default(cfg.StateDir)
	m := Model{
		cfg:     cfg,
		styles:  newStyles(false),
		help:    help.New(),
		keymap:  newKeymap(),
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		backend: b,
		applyFn: defaultApplyFn(b),
		trashFn: defaultTrashFn,
	}
	m.help.Styles = m.styles.helpStyles()
	m.preview.cache = newPreviewCache(16)
	return m
}

// loadedMsg carries one library scan plus the stored state.
type loadedMsg struct {
	entries []library.Entry
	current string
	err     error
}

func (m Model) loadCmd() tea.Cmd {
	return func() tea.Msg {
		entries, err := library.Scan(m.cfg.WallpaperDir, m.backend.Formats())
		// An unreadable state file behaves like an unset one.
		current, _ := state.Get(m.cfg.StateFile)
		return loadedMsg{entries: entries, current: current, err: err}
	}
}

// Init requests the terminal background and scans the library.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, m.loadCmd())
}

// Update applies a message. I/O happens in commands, never here.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.isDark = msg.IsDark()
		m.styles = newStyles(m.isDark)
		m.help.Styles = m.styles.helpStyles()
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list.setHeight(m.listRows())
		return m.syncPreview()
	case loadedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.current = msg.current
		m.list.setLibrary(msg.entries)
		m.list.setHeight(m.listRows())
		m.status = ""
		return m.syncPreview()
	case previewDoneMsg:
		return m, m.preview.install(msg)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.applying || m.trashing {
			return m, cmd // re-arm only while busy; idle spinners stop
		}
		return m, nil
	case applyDoneMsg:
		m.applying = false
		m.busy = ""
		if msg.err != nil {
			m.err = "apply failed: " + msg.err.Error()
			return m, nil
		}
		m.current = msg.entry.Abs
		return m, nil
	case trashedMsg:
		return m.handleTrashed(msg)
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.list.move(-1)
		case tea.MouseWheelDown:
			m.list.move(1)
		}
		return m.syncPreview()
	case tea.KeyPressMsg:
		next, cmd := m.updateKey(msg)
		next, sync := next.syncPreview()
		if cmd != nil {
			return next, tea.Batch(cmd, sync) // quit or apply
		}
		return next, sync
	}
	return m, nil
}

func (m Model) syncPreview() (Model, tea.Cmd) {
	w, h := m.previewDims()
	if w < 1 || h < 1 {
		return m, nil
	}
	e, ok := m.list.current()
	if !ok {
		return m, nil
	}
	return m, m.preview.request(e, w, h)
}

func (m Model) previewDims() (int, int) {
	listWidth := m.listWidth()
	previewWidth := m.width - listWidth - gutter
	if listWidth >= m.width || previewWidth < previewMin {
		return 0, 0
	}
	return previewWidth - 2, max(m.bodyHeight()-2, 0)
}

func (m Model) listWidth() int {
	if m.width < narrowWidth {
		return m.width
	}
	return clampInt(m.width*listPercent/100, listMin, m.width-previewMin-gutter)
}

func (m Model) updateKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	m.err = "" // any key dismisses the error card
	if m.confirm != nil {
		return m.updateConfirmKey(msg)
	}
	if m.searching {
		return m.updateSearchKey(msg)
	}
	switch {
	case key.Matches(msg, m.keymap.quit):
		return m, tea.Quit
	case key.Matches(msg, m.keymap.up):
		m.list.move(-1)
	case key.Matches(msg, m.keymap.down):
		m.list.move(1)
	case key.Matches(msg, m.keymap.pageUp):
		m.list.page(-1)
	case key.Matches(msg, m.keymap.pgDn):
		m.list.page(1)
	case key.Matches(msg, m.keymap.home):
		m.list.home()
	case key.Matches(msg, m.keymap.end):
		m.list.end()
	case key.Matches(msg, m.keymap.mark):
		m.list.toggleMark()
	case key.Matches(msg, m.keymap.trash):
		if !m.applying && !m.trashing {
			if targets := m.trashTargets(); len(targets) > 0 {
				m.confirm = targets
			}
		}
	case key.Matches(msg, m.keymap.search):
		m.searching = true
	case key.Matches(msg, m.keymap.apply):
		return m.applySelected()
	}
	return m, nil
}

func (m Model) updateSearchKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		return m.exitSearch(), nil
	case "enter":
		next, cmd := m.applySelected()
		return next.exitSearch(), cmd
	case "backspace":
		if r := []rune(m.query); len(r) > 0 {
			m.query = string(r[:len(r)-1])
			m.list.setQuery(m.query)
		}
	case "up":
		m.list.move(-1)
	case "down":
		m.list.move(1)
	case "pgup":
		m.list.page(-1)
	case "pgdown":
		m.list.page(1)
	case "home":
		m.list.home()
	case "end":
		m.list.end()
	default:
		if t := msg.Text; t != "" {
			m.query += t
			m.list.setQuery(m.query)
		}
	}
	return m, nil
}

func (m Model) exitSearch() Model {
	m.searching = false
	m.query = ""
	m.list.clearQuery()
	return m
}

// View declares the terminal features on every return path.
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m Model) bodyHeight() int {
	return max(m.height-1, 0)
}

func (m Model) listRows() int {
	return max(m.bodyHeight()-2, 0)
}

func (m Model) render() string {
	if m.width == 0 {
		return m.styles.Muted.Render("starting…")
	}
	// The card is built with the bright styles, the body behind it dimmed.
	card := m.card()
	if card != "" {
		m.styles = m.styles.dimmed()
		m.help.Styles = m.styles.helpStyles()
	}
	body := m.renderBody()
	if card != "" {
		body = overlay(body, card, m.width)
	}
	return strings.Join([]string{
		body,
		m.renderFooter(),
	}, "\n")
}

func (m Model) card() string {
	switch {
	case m.confirm != nil:
		return m.renderConfirm()
	case m.busy != "":
		return m.renderBusy()
	case m.err != "":
		return m.renderError(m.err)
	case m.status != "":
		return m.renderError(m.status)
	}
	return ""
}

func (m Model) renderError(text string) string {
	return m.renderCard("✗ "+text, m.styles.Raised.Foreground(m.styles.t.danger).Bold(true))
}

func (m Model) renderBusy() string {
	return m.renderCard(m.spinner.View()+" "+m.busy, m.styles.Raised.Foreground(m.styles.t.accent).Bold(true))
}

func (m Model) renderCard(text string, style lipgloss.Style) string {
	width := clampInt(m.width-4, 24, 56)
	body := centerIn(width-2, 1, []string{style.Render(text)}, m.styles.Raised)
	return frame(width, 3, "", "", body, m.styles.Border, m.styles.Raised)
}

func (m Model) renderFooter() string {
	left := m.styles.Meta.Render(m.cfg.FitMode)
	keys := help.KeyMap(m.keymap)
	if m.searching {
		left = m.searchBox()
		keys = searchKeys{}
	}

	right := ""
	switch {
	case m.applying:
		right = m.styles.ChipBusy.Render("applying…")
	case m.trashing:
		right = m.styles.ChipBusy.Render("trashing…")
	default:
		h := m.help
		h.SetWidth(max(m.width-ansi.StringWidth(left)-4, 10))
		right = m.helpText(h, keys)
	}
	return gapLine(left, right, m.width, m.styles.Bar)
}

// helpText is the bubbles help row with its bare key/description separators
// re-painted: the bubble emits an unstyled space between runs, which would
// punch a hole in the footer fill after each reset.
func (m Model) helpText(h help.Model, keys help.KeyMap) string {
	space := "\x1b[m" + m.styles.Bar.Render(" ")
	return strings.ReplaceAll(h.View(keys), "\x1b[m ", space)
}

// searchBox is the modal search input: a filled box with a prompt and a block
// cursor. The query keeps its tail, where the cursor sits.
func (m Model) searchBox() string {
	s := m.styles.Raised
	return s.Padding(0, 1).Render(
		s.Foreground(m.styles.t.accent).Bold(true).Render("❯ ") +
			s.Foreground(m.styles.t.text).Render(tailCut(m.query, max(m.width/3, 8))) +
			s.Foreground(m.styles.t.accent).Render("▌"))
}

func (m Model) renderBody() string {
	height := m.bodyHeight()
	if height < 1 {
		return ""
	}

	listWidth := m.listWidth()
	previewWidth := m.width - listWidth - gutter
	if listWidth >= m.width || previewWidth < previewMin {
		return m.listPanel(m.width, height)
	}

	e, _ := m.list.current()
	return lipgloss.JoinHorizontal(lipgloss.Top,
		gutterBlock(m.listPanel(listWidth, height), gutter),
		m.preview.render(previewWidth, height, e, m.styles),
	)
}

func (m Model) listPanel(width, height int) string {
	total := len(m.list.library)
	right := strconv.Itoa(total)
	if m.list.query != "" {
		right = fmt.Sprintf("%d/%d", len(m.list.matches), total)
	}
	left := m.styles.TitleL.Render("❯ ") + m.styles.TitleR.Render("Wallpapers")

	body := m.listBody(max(width-2, 0), max(height-2, 0))
	if box := frame(width, height, left, right, body, m.styles.Border, m.styles.Panel); box != "" {
		return box
	}
	return m.list.render(width, height, m.current, m.rowMeta, m.styles)
}

func (m Model) listBody(innerWidth, innerHeight int) string {
	if innerWidth < 1 || innerHeight < 1 {
		return ""
	}
	switch {
	case len(m.list.library) == 0:
		return centerIn(innerWidth, innerHeight, []string{
			m.styles.Panel.Foreground(m.styles.t.text).Render("no wallpapers"),
			m.styles.Panel.Foreground(m.styles.t.muted).Render(m.cfg.WallpaperDir),
			"",
			m.styles.Panel.Foreground(m.styles.t.muted).Render("add images to the folder and reopen"),
		}, m.styles.Panel)
	case len(m.list.matches) == 0:
		return centerIn(innerWidth, innerHeight, []string{
			m.styles.Panel.Foreground(m.styles.t.text).Render("no results for " + strconv.Quote(m.list.query)),
			m.styles.Panel.Foreground(m.styles.t.muted).Render("esc clears the search"),
		}, m.styles.Panel)
	}
	return m.list.render(innerWidth, innerHeight, m.current, m.rowMeta, m.styles)
}

func (m Model) rowMeta(e library.Entry) string {
	var parts []string
	if p := m.preview.res; p.path == e.Abs && p.w > 0 {
		parts = append(parts, fmt.Sprintf("%d×%d", p.w, p.h))
	}
	if e.Size > 0 {
		parts = append(parts, humanSize(e.Size))
	}
	return strings.Join(parts, " · ")
}

// gapLine lays left and right out on one row of exactly width cells, filling
// the gap with fill so a bar background stays continuous.
func gapLine(left, right string, width int, fill lipgloss.Style) string {
	if width < 1 {
		return ""
	}
	lw := ansi.StringWidth(left)
	if lw >= width {
		return ansi.Truncate(left, width, "…")
	}
	right = ansi.Truncate(right, max(width-lw-1, 0), "…")
	return left + fill.Render(strings.Repeat(" ", max(width-lw-ansi.StringWidth(right), 1))) + right
}

// tailCut keeps the end of s, which is where the block cursor sits.
func tailCut(s string, width int) string {
	if ansi.StringWidth(s) <= width {
		return s
	}
	return "…" + ansi.TruncateLeft(s, ansi.StringWidth(s)-width+1, "")
}
