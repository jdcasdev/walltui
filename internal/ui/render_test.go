package ui

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	xgolden "github.com/charmbracelet/x/exp/golden"
	xdraw "golang.org/x/image/draw"

	"github.com/jdcasdev/walltui/internal/library"
)

// writePNG encodes img to a temp PNG and returns its path.
func writePNG(t *testing.T, img image.Image) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "img.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return path
}

// checker returns a 2x2 image: red, green / blue, white.
func checker() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	return img
}

func TestFit(t *testing.T) {
	tests := []struct {
		name           string
		srcW, srcH     int
		availW, availH int
		wantW, wantH   int
	}{
		{"square into square", 100, 100, 10, 10, 10, 10},
		{"wide letterboxed", 200, 100, 10, 10, 10, 5},
		{"tall letterboxed", 100, 200, 10, 10, 5, 10},
		{"upscales", 4, 4, 20, 20, 20, 20},
		{"rounding clamps", 3, 2, 2, 3, 2, 1},
		{"degenerate", 0, 10, 5, 5, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h := fit(tt.srcW, tt.srcH, tt.availW, tt.availH)
			if w != tt.wantW || h != tt.wantH {
				t.Errorf("fit(%d,%d,%d,%d) = %d,%d; want %d,%d",
					tt.srcW, tt.srcH, tt.availW, tt.availH, w, h, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestRenderHalfblocksChecker(t *testing.T) {
	path := writePNG(t, checker())
	out := renderHalfblocks(path, 2, 1)

	if got := ansi.Strip(out); got != "▀▀" {
		t.Errorf("stripped = %q, want ▀▀", got)
	}
	for _, code := range []string{
		"38;2;255;0;0", "48;2;0;0;255", // red over blue
		"38;2;0;255;0", "48;2;255;255;255", // green over white
	} {
		if !strings.Contains(out, code) {
			t.Errorf("output missing color %s:\n%q", code, out)
		}
	}
}

func TestRenderHalfblocksLetterbox(t *testing.T) {
	// 8x2 source into a 4x2-cell pane: fits 4 wide, 1 cell tall, centered.
	img := image.NewRGBA(image.Rect(0, 0, 8, 2))
	path := writePNG(t, img)

	out := renderHalfblocks(path, 4, 2)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2:\n%q", len(lines), out)
	}
	if got := ansi.Strip(lines[0]); got != "▀▀▀▀" {
		t.Errorf("row 0 = %q, want ▀▀▀▀", got)
	}
	if got := ansi.Strip(lines[1]); got != "" {
		t.Errorf("row 1 = %q, want empty letterbox row", got)
	}
}

func TestRenderHalfblocksWideLetterboxSide(t *testing.T) {
	// 2x4 source into a 4x2-cell pane: 2 cells wide, 2 tall, centered with
	// one letterbox column left; rows are right-trimmed.
	img := image.NewRGBA(image.Rect(0, 0, 2, 4))
	path := writePNG(t, img)

	out := renderHalfblocks(path, 4, 2)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2:\n%q", len(lines), out)
	}
	for i, line := range lines {
		if got := ansi.Strip(line); got != " ▀▀" {
			t.Errorf("row %d = %q, want %q", i, got, " ▀▀")
		}
	}
}

func TestRenderHalfblocksErrors(t *testing.T) {
	if got := renderHalfblocks(filepath.Join(t.TempDir(), "missing.png"), 4, 2); !strings.HasPrefix(got, "cannot preview:") {
		t.Errorf("missing file = %q, want placeholder", got)
	}
	bad := filepath.Join(t.TempDir(), "bad.png")
	if err := os.WriteFile(bad, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := renderHalfblocks(bad, 4, 2); !strings.HasPrefix(got, "cannot preview:") {
		t.Errorf("corrupt file = %q, want placeholder", got)
	}
	if got := renderHalfblocks("ignored", 0, 2); got != "" {
		t.Errorf("zero width = %q, want empty", got)
	}
}

func TestRenderHalfblocksGolden(t *testing.T) {
	// 24x24 diagonal gradient, exact at 12x6 cells (no scaler rounding).
	img := image.NewRGBA(image.Rect(0, 0, 24, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 24; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 10), G: uint8(y * 10), B: 128, A: 255})
		}
	}
	path := writePNG(t, img)
	xgolden.RequireEqual(t, ansi.Strip(renderHalfblocks(path, 12, 6)))
}

func TestPreviewSerialization(t *testing.T) {
	path := writePNG(t, checker())
	p := preview{cache: newPreviewCache(4)}
	entry := library.Entry{Abs: path, Rel: "img.png"}

	// A starts decoding; B must queue behind it, not run concurrently.
	cmdA := p.request(entry, 2, 1)
	if cmdA == nil {
		t.Fatal("first request should start a decode")
	}
	if cmdB := p.request(entry, 4, 2); cmdB != nil {
		t.Error("second request while running should be pending, not a command")
	}

	// A finishes but is stale (B has the newer seq): it must not install.
	cmd2 := p.install(cmdA().(previewDoneMsg))
	if p.res.cells != "" {
		t.Errorf("stale decode was installed: %q", p.res.cells)
	}

	// The queued request takes over and installs.
	if cmd2 == nil {
		t.Fatal("pending request should have started")
	}
	p.install(cmd2().(previewDoneMsg))
	if p.res.cells == "" {
		t.Error("fresh decode was not installed")
	}
}

// TestPreviewCacheEviction checks the oldest entry goes when the cache is full.
func TestPreviewCacheEviction(t *testing.T) {
	path := writePNG(t, checker())
	e := library.Entry{Abs: path, Rel: "img.png"}
	c := newPreviewCache(2)

	c.render(e, 2, 1)
	c.render(e, 4, 2)
	c.render(e, 8, 4) // evicts the (2,1) entry

	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.items[cacheKey(path, 2, 1)]; ok {
		t.Error("oldest entry was not evicted")
	}
	if _, ok := c.items[cacheKey(path, 4, 2)]; !ok {
		t.Error("recent entry went missing")
	}
}

func BenchmarkRenderHalfblocks(b *testing.B) {
	benchmarkRender(b, xdraw.ApproxBiLinear)
}

func BenchmarkRenderHalfblocksCatmullRom(b *testing.B) {
	benchmarkRender(b, xdraw.CatmullRom)
}

// benchmarkRender renders a 1920x1080 JPEG into a 40x20-cell pane with the
// given scaler; the package default follows the faster benchmark.
func benchmarkRender(b *testing.B, s xdraw.Scaler) {
	old := scaler
	scaler = s
	defer func() { scaler = old }()

	big := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 1080; y++ {
		for x := 0; x < 1920; x++ {
			big.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	path := filepath.Join(b.TempDir(), "big.jpg")
	f, err := os.Create(path)
	if err != nil {
		b.Fatal(err)
	}
	if err := jpeg.Encode(f, big, nil); err != nil {
		b.Fatal(err)
	}
	f.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		renderHalfblocks(path, 40, 20)
	}
}
