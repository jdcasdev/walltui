package ui

import (
	"image/color"

	"charm.land/bubbles/v2/help"
	"charm.land/lipgloss/v2"
)

// tokens is the raw palette for one terminal background. newStyles is its
// only consumer and TestPaletteContrast checks every pair the UI draws.
type tokens struct {
	isDark   bool
	surface  color.Color
	panel    color.Color
	raised   color.Color
	border   color.Color
	text     color.Color
	muted    color.Color
	accent   color.Color
	selBg    color.Color
	selFg    color.Color
	success  color.Color
	danger   color.Color
	dangerFg color.Color
}

// palette resolves the tokens for a terminal background. The light value always
// comes first: lipgloss.LightDark is func(light, dark).
func palette(isDark bool) tokens {
	c := lipgloss.LightDark(isDark)
	hex := lipgloss.Color
	return tokens{
		isDark:   isDark,
		surface:  c(hex("#f2f4f7"), hex("#0d0f13")),
		panel:    c(hex("#ffffff"), hex("#14171d")),
		raised:   c(hex("#eceff4"), hex("#1e232c")),
		border:   c(hex("#d5dae2"), hex("#2b3140")),
		text:     c(hex("#1f2328"), hex("#d8dbe2")),
		muted:    c(hex("#5f6b7f"), hex("#8a93a3")),
		accent:   c(hex("#3556d1"), hex("#7da2ff")),
		selBg:    c(hex("#dbe4ff"), hex("#2a3550")),
		selFg:    c(hex("#16233f"), hex("#eef2ff")),
		success:  c(hex("#176f30"), hex("#6fd18a")),
		danger:   c(hex("#cf222e"), hex("#ff7b72")),
		dangerFg: c(hex("#ffffff"), hex("#20100e")),
	}
}

// dimAmount is how far the backdrop blends toward the surface.
const dimAmount = 0.55

// dimmed blends the palette toward the surface: the backdrop behind a card.
// The chip foregrounds stay put, so a dimmed chip is never dark on dark.
func (t tokens) dimmed() tokens {
	t.panel = blend(t.panel, t.surface, dimAmount)
	t.raised = blend(t.raised, t.surface, dimAmount)
	t.border = blend(t.border, t.surface, dimAmount)
	t.text = blend(t.text, t.surface, dimAmount)
	t.muted = blend(t.muted, t.surface, dimAmount)
	t.accent = blend(t.accent, t.surface, dimAmount)
	t.selBg = blend(t.selBg, t.surface, dimAmount)
	t.selFg = blend(t.selFg, t.surface, dimAmount)
	t.success = blend(t.success, t.surface, dimAmount)
	t.danger = blend(t.danger, t.surface, dimAmount)
	return t
}

// blend mixes c toward t by f (0 = c, 1 = t).
func blend(c, t color.Color, f float64) color.Color {
	r, g, b, _ := c.RGBA()
	tr, tg, tb, _ := t.RGBA()
	mix := func(x, to uint32) uint32 {
		return uint32(float64(x) + (float64(to)-float64(x))*f)
	}
	return color.RGBA{
		R: uint8(mix(r, tr) >> 8),
		G: uint8(mix(g, tg) >> 8),
		B: uint8(mix(b, tb) >> 8),
		A: 0xff,
	}
}

// Styles holds every style in the app, built once from the terminal
// background. Components receive it on Render; render code never holds a
// color literal.
//
// Spans are grouped by the surface they sit on. Lip Gloss resets between
// inline runs, so a foreground-only span drawn on a filled surface would drop
// that fill for its own cells: text on a bar, a panel, a row or the raised
// dialog is therefore always painted from the matching surface style, with
// t supplying the color.
type Styles struct {
	IsDark bool
	t      tokens

	// Chrome bars, background included.
	Bar   lipgloss.Style // footer fill
	Title lipgloss.Style // accent, on the bar
	Meta  lipgloss.Style // bar text

	// Foreground only: text drawn on the terminal surface, and frame lines.
	Muted        lipgloss.Style
	Border       lipgloss.Style
	BorderDanger lipgloss.Style
	TitleL       lipgloss.Style // panel title, left
	TitleR       lipgloss.Style // panel title, right

	// Row and surface painters, background included.
	Row, RowSel   lipgloss.Style
	Panel, Raised lipgloss.Style

	// Chips carry their own background.
	ChipBusy, Action lipgloss.Style
}

func newStyles(isDark bool) Styles {
	return stylesFrom(palette(isDark))
}

func stylesFrom(p tokens) Styles {
	bar := lipgloss.NewStyle().Background(p.surface)
	chip := func(bg, fg color.Color) lipgloss.Style {
		return lipgloss.NewStyle().Background(bg).Foreground(fg).Padding(0, 1)
	}
	return Styles{
		IsDark: p.isDark,
		t:      p,

		Bar:   bar,
		Title: bar.Foreground(p.accent).Bold(true),
		Meta:  bar.Foreground(p.muted),

		Muted:        lipgloss.NewStyle().Foreground(p.muted),
		Border:       lipgloss.NewStyle().Foreground(p.border),
		BorderDanger: lipgloss.NewStyle().Foreground(p.danger),
		TitleL:       lipgloss.NewStyle().Foreground(p.accent).Bold(true),
		TitleR:       lipgloss.NewStyle().Foreground(p.muted),

		Row:    lipgloss.NewStyle().Foreground(p.text).Background(p.panel),
		RowSel: lipgloss.NewStyle().Foreground(p.selFg).Background(p.selBg).Bold(true),
		Panel:  lipgloss.NewStyle().Background(p.panel),
		Raised: lipgloss.NewStyle().Background(p.raised),

		ChipBusy: chip(p.raised, p.accent),
		Action:   chip(p.danger, p.dangerFg).Bold(true),
	}
}

func (s Styles) dimmed() Styles {
	p := palette(s.IsDark)
	p = p.dimmed()
	return stylesFrom(p)
}

// helpStyles is the bubbles help styling for the footer bar: every part carries
// the bar background so the fill stays continuous between items.
func (s Styles) helpStyles() help.Styles {
	return help.Styles{
		Ellipsis:       s.Meta,
		ShortKey:       s.Title,
		ShortDesc:      s.Meta,
		ShortSeparator: s.Meta,
		FullKey:        s.Title,
		FullDesc:       s.Meta,
		FullSeparator:  s.Meta,
	}
}
