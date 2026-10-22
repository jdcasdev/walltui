package ui

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"testing"
)

// TestPaletteContrast is the check behind the palette: every text pair the UI
// actually draws must clear WCAG AA against the surface it sits on.
func TestPaletteContrast(t *testing.T) {
	for _, tc := range []struct {
		name   string
		isDark bool
	}{
		{"dark", true},
		{"light", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := palette(tc.isDark)
			for _, pair := range []struct {
				name   string
				fg, bg color.Color
			}{
				{"text on panel", p.text, p.panel},
				{"muted on panel", p.muted, p.panel},
				{"accent on panel", p.accent, p.panel},
				{"success on panel", p.success, p.panel},
				{"text on selected", p.selFg, p.selBg},
				{"success on selected", p.success, p.selBg},
				{"accent on selected", p.accent, p.selBg},
				{"muted on bar", p.muted, p.surface},
				{"accent on bar", p.accent, p.surface},
				{"muted on raised", p.muted, p.raised},
				{"text on raised", p.text, p.raised},
				{"success on raised", p.success, p.raised},
				{"danger on raised", p.danger, p.raised},
				{"dialog text", p.dangerFg, p.danger},
				{"accent on raised", p.accent, p.raised},
			} {
				if got := contrastRatio(pair.fg, pair.bg); got < 4.5 {
					t.Errorf("%s: %.2f:1, want >= 4.5:1", pair.name, got)
				}
			}
		})
	}
}

// TestCardDimsBackdrop checks that the body behind a card really recedes:
// the dimmed text colour replaces the bright one.
func TestCardDimsBackdrop(t *testing.T) {
	m := testModel(true, 110, 20)
	p := palette(true)

	bright := colorTriplet(p.text)
	dim := colorTriplet(blend(p.text, p.surface, dimAmount))

	if plain := m.render(); !strings.Contains(plain, bright) {
		t.Fatal("the plain view does not use the text colour")
	}

	m.busy = "applying a.png"
	out := m.render()
	if !strings.Contains(out, dim) {
		t.Error("the body behind the card is not dimmed")
	}
	if strings.Contains(out, bright) {
		t.Error("the body behind the card still uses the bright text colour")
	}
}

// colorTriplet is the RGB triple lipgloss writes for a colour.
func colorTriplet(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("%d;%d;%d", r>>8, g>>8, b>>8)
}

// TestNoFillHoles guards the rule that every span on a filled surface carries
// that surface's background: a reset followed by a bare space means a hole.
// Frame lines are exempt: they sit on the terminal surface, which has no fill.
func TestNoFillHoles(t *testing.T) {
	m := testModel(true, 110, 20)

	// List rows: cursor, chips, name, metadata and scrollbar all carry the row
	// background, whichever row style paints them.
	rows := m.list.render(m.listWidth()-2, m.listRows(), "/walls/sub/b.jpg", m.rowMeta, m.styles)
	assertNoHoles(t, "list rows", rows)
	assertNoHoles(t, "list rows selected", m.list.render(m.listWidth()-2, m.listRows(), "", m.rowMeta, newStyles(false)))

	m.searching, m.query = true, "sub"
	assertNoHoles(t, "footer", m.renderFooter())
	assertNoHoles(t, "search box", m.searchBox())
	assertNoHoles(t, "help row", m.helpText(m.help, m.keymap))
	assertNoHoles(t, "error card", m.renderError("apply failed"))
	assertNoHoles(t, "busy card", m.renderBusy())

	m.list.setLibrary(nil)
	assertNoHoles(t, "empty state", m.listBody(40, 10))
	m.list.setLibrary(testEntries(5))
	m.list.setQuery("nothing matches this")
	assertNoHoles(t, "no results", m.listBody(40, 10))

	m.list.clearQuery()
	m.confirm = m.trashTargets()
	assertNoHoles(t, "dialog", m.renderConfirm())

	w, h := m.previewDims()
	assertNoHoles(t, "placeholder", placeholder(w, h, m.styles))
}

func assertNoHoles(t *testing.T, name, s string) {
	t.Helper()
	if strings.Contains(s, "\x1b[m ") {
		t.Errorf("%s: unstyled space after a reset punches a hole in the fill", name)
	}
}

// contrastRatio is the WCAG 2.1 relative-luminance ratio, 1.0–21.0.
func contrastRatio(a, b color.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	lin := func(v uint32) float64 {
		x := float64(v) / 65535
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}
