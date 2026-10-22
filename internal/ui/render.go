package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"strings"

	// Image format decoders. png/jpeg/gif from the stdlib; webp and bmp are
	// decode-only in golang.org/x/image.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	"charm.land/lipgloss/v2"
	xdraw "golang.org/x/image/draw"
)

// scaler trades quality for speed. ApproxBiLinear keeps preview latency low
// while scrolling; see BenchmarkRenderHalfblocks in render_test.go for the numbers.
var scaler xdraw.Scaler = xdraw.ApproxBiLinear

// halfBlock renders two vertically stacked pixels per cell: fg = top pixel,
// bg = bottom pixel.
const halfBlock = "▀"

// renderImage decodes path, aspect-fits it into cellW x cellH cells and renders
// it as half-block runs. It never returns an error: failures become a placeholder.
func renderImage(path string, cellW, cellH int) previewResult {
	if cellW < 1 || cellH < 1 {
		return previewResult{}
	}

	img, err := decode(path)
	if err != nil {
		return previewResult{cells: fmt.Sprintf("cannot preview: %v", err)}
	}

	b := img.Bounds()
	pw, ph := fit(b.Dx(), b.Dy(), cellW, cellH*2)
	if pw == 0 {
		return previewResult{cells: "cannot preview: empty image"}
	}

	dst := image.NewRGBA(image.Rect(0, 0, pw, ph))
	scaler.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)

	return previewResult{cells: compose(dst, pw, ph, cellW, cellH), w: b.Dx(), h: b.Dy()}
}

func decode(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func fit(srcW, srcH, availW, availH int) (int, int) {
	if srcW <= 0 || srcH <= 0 || availW < 1 || availH < 1 {
		return 0, 0
	}
	scale := math.Min(float64(availW)/float64(srcW), float64(availH)/float64(srcH))
	w := max(1, min(int(math.Round(float64(srcW)*scale)), availW))
	h := max(1, min(int(math.Round(float64(srcH)*scale)), availH))
	return w, h
}

func compose(src *image.RGBA, pw, ph, cellW, cellH int) string {
	offX := (cellW - pw) / 2
	offY := (cellH*2 - ph) / 2

	lines := make([]string, cellH)
	for cy := 0; cy < cellH; cy++ {
		topY := cy*2 - offY
		var row strings.Builder
		var runLen int
		var runTop, runBot color.RGBA

		flush := func() {
			if runLen > 0 {
				row.WriteString(halfBlockRun(runTop, runBot, runLen))
				runLen = 0
			}
		}

		for cx := 0; cx < cellW; cx++ {
			px := cx - offX
			by := topY + 1
			inTop := px >= 0 && px < pw && topY >= 0 && topY < ph
			inBot := px >= 0 && px < pw && by >= 0 && by < ph
			if !inTop && !inBot {
				flush()
				row.WriteByte(' ')
				continue
			}
			var top, bot color.RGBA
			if inTop {
				top = src.RGBAAt(px, topY)
			}
			if inBot {
				bot = src.RGBAAt(px, by)
			}
			// Duplicate the present pixel when the image ends mid-cell.
			if !inBot {
				bot = top
			}
			if !inTop {
				top = bot
			}
			if runLen > 0 && (top != runTop || bot != runBot) {
				flush()
			}
			runTop, runBot = top, bot
			runLen++
		}
		flush()
		lines[cy] = strings.TrimRight(row.String(), " ")
	}
	return strings.Join(lines, "\n")
}

var cellStyle = lipgloss.NewStyle()

func halfBlockRun(top, bot color.RGBA, n int) string {
	style := cellStyle.
		Foreground(top).
		Background(bot)
	return style.Render(strings.Repeat(halfBlock, n))
}
