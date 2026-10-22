package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// frame renders body inside a rounded box of exactly width x height cells.
// Content lines are cut to the inner width and padded with fill, so the panel
// background stays continuous. It returns "" when the box is too small.
func frame(width, height int, left, right, body string, border, fill lipgloss.Style) string {
	inner := width - 2
	if inner < 4 || height < 2 {
		return ""
	}

	out := make([]string, 0, height)
	out = append(out, border.Render("╭")+edge(inner, left, right, border)+border.Render("╮"))

	lines := strings.Split(body, "\n")
	for len(lines) < height-2 {
		lines = append(lines, "")
	}
	for _, l := range lines[:height-2] {
		out = append(out, border.Render("│")+
			fill.Width(inner).Render(ansi.Truncate(l, inner, "…"))+
			border.Render("│"))
	}
	out = append(out, border.Render("╰")+
		border.Render(strings.Repeat("─", inner))+
		border.Render("╯"))
	return strings.Join(out, "\n")
}

func edge(inner int, left, right string, border lipgloss.Style) string {
	dash := border.Render("─")
	var l, r string
	if left != "" {
		l = dash + " " + left + " "
	}
	if right != "" {
		r = " " + right + " " + dash
	}

	gap := inner - ansi.StringWidth(l) - ansi.StringWidth(r)
	if gap < 0 { // titles do not fit side by side: drop the right one
		r = ""
		gap = inner - ansi.StringWidth(l)
	}
	if gap < 0 { // still too long: cut the left one down
		l = ansi.Truncate(l, max(inner-1, 0), "…")
		gap = inner - ansi.StringWidth(l)
	}
	return l + border.Render(strings.Repeat("─", max(gap, 0))) + r
}

func scrollbarCol(offset, height, total int) []scrollCell {
	if height < 1 || total <= height {
		return nil
	}
	thumb := max(1, height*height/total)
	top := offset * (height - thumb) / max(total-height, 1)

	out := make([]scrollCell, height)
	for i := range out {
		if i >= top && i < top+thumb {
			out[i] = scrollCell{thumb: true}
			continue
		}
		out[i] = scrollCell{track: true}
	}
	return out
}

// centerIn centers pre-styled lines in a block of exactly w x h cells, filled
// with fill so the panel background stays continuous.
func centerIn(w, h int, lines []string, fill lipgloss.Style) string {
	if w < 1 || h < 1 {
		return ""
	}
	out := make([]string, 0, h)
	blank := fill.Render(strings.Repeat(" ", w))
	for i := 0; i < (h-len(lines))/2; i++ {
		out = append(out, blank)
	}
	for _, l := range lines {
		out = append(out, fill.Width(w).Align(lipgloss.Center).Render(ansi.Truncate(l, w, "…")))
	}
	for len(out) < h {
		out = append(out, blank)
	}
	return strings.Join(out, "\n")
}

// humanSize formats a byte count the way file managers do: 512B, 512K, 1.2M.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit && exp < 3; v /= unit {
		div *= unit
		exp++
	}
	v := float64(n) / float64(div)
	if v >= 10 { // 512K, not 512.0K
		return fmt.Sprintf("%.0f%c", v, "KMGT"[exp])
	}
	return fmt.Sprintf("%.1f%c", v, "KMGT"[exp])
}

func gutterBlock(block string, n int) string {
	lines := strings.Split(block, "\n")
	pad := strings.Repeat(" ", n)
	for i, l := range lines {
		lines[i] = l + pad
	}
	return strings.Join(lines, "\n")
}

func clampInt(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	return max(lo, min(v, hi))
}
