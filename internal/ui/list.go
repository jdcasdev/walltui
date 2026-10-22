package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jdcasdev/walltui/internal/library"
)

// list is the wallpaper list: cursor, viewport offset, and marks. It carries
// only data and viewport state; styling is injected at render time so the
// semantic Styles stay the single source of truth.
type list struct {
	library []library.Entry // every entry in the library
	matches []library.Entry // the ones the query matches
	query   string
	cursor  int
	offset  int
	height  int
	marks   map[string]bool
}

func (l *list) setLibrary(items []library.Entry) {
	l.library = items
	l.marks = make(map[string]bool)
	l.applyQuery()
	l.clamp()
}

// setQuery narrows the visible entries to a case-insensitive substring match
// on the relative path and returns the cursor to the first match.
func (l *list) setQuery(q string) {
	l.query = q
	l.cursor, l.offset = 0, 0
	l.applyQuery()
	l.clamp()
}

func (l *list) clearQuery() {
	l.query = ""
	l.applyQuery()
	l.clamp()
}

func (l *list) applyQuery() {
	if l.query == "" {
		l.matches = l.library
		return
	}
	q := strings.ToLower(l.query)
	out := make([]library.Entry, 0, len(l.library))
	for _, e := range l.library {
		if strings.Contains(strings.ToLower(e.Rel), q) {
			out = append(out, e)
		}
	}
	l.matches = out
}

func (l *list) setHeight(h int) {
	l.height = max(h, 0)
	l.clamp()
}

func (l *list) clamp() {
	if len(l.matches) == 0 {
		l.cursor, l.offset = 0, 0
		return
	}
	l.cursor = max(0, min(l.cursor, len(l.matches)-1))
	if l.height > 0 {
		if l.cursor < l.offset {
			l.offset = l.cursor
		}
		if l.cursor >= l.offset+l.height {
			l.offset = l.cursor - l.height + 1
		}
	}
	l.offset = max(0, min(l.offset, max(0, len(l.matches)-l.height)))
}

func (l *list) move(delta int) {
	l.cursor += delta
	l.clamp()
}

func (l *list) page(dir int) {
	l.move(dir * max(l.height-1, 1))
}

func (l *list) home() { l.cursor = 0; l.clamp() }

func (l *list) end() { l.cursor = len(l.matches) - 1; l.clamp() }

func (l *list) current() (library.Entry, bool) {
	if len(l.matches) == 0 {
		return library.Entry{}, false
	}
	return l.matches[l.cursor], true
}

func (l *list) toggleMark() {
	e, ok := l.current()
	if !ok {
		return
	}
	if l.marks[e.Abs] {
		delete(l.marks, e.Abs)
	} else {
		l.marks[e.Abs] = true
	}
}

// marked returns the marked entries in library order. Marks live on the
// unfiltered library so filtering never silently unmarks anything.
func (l *list) marked() []library.Entry {
	var out []library.Entry
	for _, e := range l.library {
		if l.marks[e.Abs] {
			out = append(out, e)
		}
	}
	return out
}

// render draws exactly height lines of exactly width cells: a cursor column,
// the mark and current chips, the name, a right-aligned metadata column, and
// a scrollbar when the library overflows the viewport. meta formats that
// column for one entry; the column is sized once per render so rows line up.
func (l *list) render(width, height int, currentPath string, meta func(library.Entry) string, s Styles) string {
	bar := scrollbarCol(l.offset, height, len(l.matches))
	rowWidth := width
	if bar != nil {
		rowWidth--
	}
	metaWidth := l.metaWidth(meta, height)

	lines := make([]string, 0, max(height, 0))
	for i := l.offset; i < len(l.matches) && len(lines) < height; i++ {
		lines = append(lines,
			l.row(i, rowWidth, metaWidth, cellAt(bar, len(lines)), currentPath, meta, s))
	}
	for len(lines) < height {
		lines = append(lines, l.blank(rowWidth, cellAt(bar, len(lines)), s))
	}
	return strings.Join(lines, "\n")
}

func (l *list) metaWidth(meta func(library.Entry) string, height int) int {
	if meta == nil {
		return 0
	}
	w := 0
	for i := l.offset; i < len(l.matches) && i < l.offset+height; i++ {
		w = max(w, ansi.StringWidth(meta(l.matches[i])))
	}
	return w
}

// row lays one entry out across exactly width cells (the scrollbar cell is
// appended by render). Every span is painted from the row surface style, so
// the selection bar stays continuous from the cursor to the scrollbar.
func (l *list) row(i, width, metaWidth int, bar scrollCell, currentPath string, meta func(library.Entry) string, s Styles) string {
	e := l.matches[i]
	paint := s.Row
	if i == l.cursor {
		paint = s.RowSel
	}
	if width < 10 { // no room for cells: paint the row through
		return paint.Width(width).Render("") + l.scroll(bar, paint, s)
	}

	cursor, chip := paint.Render("  "), paint.Render("  ")
	if i == l.cursor {
		cursor = paint.Foreground(s.t.accent).Bold(true).Render("▸ ")
	}
	if l.marks[e.Abs] {
		chip = paint.Foreground(s.t.success).Bold(true).Render("✓ ")
	}

	// Right-hand cluster: a 2-cell slot for the current marker plus the
	// metadata column. Both are reserved on every row so the columns line up,
	// and both are dropped when they would squeeze the names too far.
	const fixed = 4 // cursor column + mark chip
	slot := paint.Render("  ")
	if e.Abs == currentPath {
		slot = paint.Foreground(s.t.accent).Render("● ")
	}
	text := ""
	if meta != nil {
		text = meta(e)
	}
	cell := paint.Foreground(s.t.muted).Render(
		text + strings.Repeat(" ", max(metaWidth-ansi.StringWidth(text), 0)))

	cluster := ""
	if metaWidth > 0 && width-fixed-1-(2+metaWidth) >= 8 {
		cluster = slot + cell
	} else if e.Abs == currentPath {
		cluster = slot
	}
	gap := ""
	if cluster != "" {
		gap = " "
	}
	nameWidth := max(width-fixed-ansi.StringWidth(gap)-ansi.StringWidth(cluster), 1)

	name := ansi.Truncate(e.Rel, nameWidth, "…")
	name = paint.Render(name + strings.Repeat(" ", nameWidth-ansi.StringWidth(name)))

	return cursor + chip + name + paint.Render(gap) + cluster + l.scroll(bar, paint, s)
}

func (l *list) blank(width int, bar scrollCell, s Styles) string {
	return s.Row.Width(width).Render("") + l.scroll(bar, s.Row, s)
}

// scroll paints the scrollbar cell of one row, using the row's own surface.
func (l *list) scroll(bar scrollCell, paint lipgloss.Style, s Styles) string {
	switch {
	case bar.thumb:
		return paint.Foreground(s.t.accent).Render("▌")
	case bar.track:
		return paint.Foreground(s.t.border).Render("│")
	}
	return ""
}

type scrollCell struct {
	track bool
	thumb bool
}

func cellAt(bar []scrollCell, i int) scrollCell {
	if i < 0 || i >= len(bar) {
		return scrollCell{}
	}
	return bar[i]
}
