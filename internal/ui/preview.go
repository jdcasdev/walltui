package ui

import (
	"fmt"
	"os"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jdcasdev/walltui/internal/library"
)

// preview is the right-hand image pane. Decoding happens in commands; the
// component only tracks request sequencing (stale results are dropped) and
// serializes decodes so fast scrolling coalesces to the latest selection.
type preview struct {
	seq     int // id of the newest request
	running bool
	pending *previewReq
	res     previewResult // rendered content of the applied request
	cache   *previewCache
}

// previewResult is one rendered image: the cells to draw and the source
// image's pixel dimensions. path keys it to the entry it belongs to.
type previewResult struct {
	path  string
	cells string
	w, h  int
}

type previewReq struct {
	entry library.Entry
	w, h  int
	seq   int
}

type previewDoneMsg struct {
	req *previewReq
	res previewResult
}

// request schedules a render at the given cell size. It returns nil while a
// decode is running; that decode picks this request up when it finishes.
func (p *preview) request(e library.Entry, w, h int) tea.Cmd {
	p.seq++
	req := &previewReq{entry: e, w: w, h: h, seq: p.seq}
	if p.running {
		p.pending = req
		return nil
	}
	p.running = true
	return previewCmd(p.cache, req)
}

func (p *preview) install(msg previewDoneMsg) tea.Cmd {
	if msg.req.seq == p.seq {
		p.res = msg.res
	}
	p.running = false
	if p.pending == nil {
		return nil
	}
	req := p.pending
	p.pending = nil
	p.running = true
	return previewCmd(p.cache, req)
}

func previewCmd(cache *previewCache, req *previewReq) tea.Cmd {
	return func() tea.Msg {
		return previewDoneMsg{req: req, res: cache.render(req.entry, req.w, req.h)}
	}
}

// render draws the pane: the half-block image, or a centered placeholder when
// none is ready, inside a framed panel titled with the entry name. The size
// comes from the list row, not from here.
func (p preview) render(width, height int, e library.Entry, s Styles) string {
	if width < 12 || height < 5 {
		return ""
	}
	innerW, innerH := width-2, height-2

	cells := ""
	if p.res.path == e.Abs {
		cells = p.res.cells
	}
	if cells == "" {
		cells = placeholder(innerW, innerH, s)
	}
	left := ""
	if e.Rel != "" {
		left = s.TitleL.Render("❯ ") +
			s.TitleR.Render(ansi.Truncate(e.Rel, max(innerW-12, 4), "…"))
	}
	return frame(width, height, left, "", cells, s.Border, s.Panel)
}

func placeholder(w, h int, s Styles) string {
	dim := s.Panel.Foreground(s.t.muted)
	return centerIn(w, h, []string{dim.Render("▌"), dim.Render("preview")}, s.Panel)
}

// previewCache is a bounded FIFO keyed by path, mtime, size, and cell dims.
// Safe for concurrent use: commands render while the UI reads.
type previewCache struct {
	mu    sync.Mutex
	cap   int
	order []string
	items map[string]previewResult
}

func newPreviewCache(capacity int) *previewCache {
	return &previewCache{cap: capacity, items: make(map[string]previewResult)}
}

func (c *previewCache) render(e library.Entry, w, h int) previewResult {
	key := cacheKey(e.Abs, w, h)

	c.mu.Lock()
	if res, ok := c.items[key]; ok {
		c.mu.Unlock()
		return res
	}
	c.mu.Unlock()

	res := renderImage(e.Abs, w, h)
	res.path = e.Abs

	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.items[key]; !exists {
		c.items[key] = res
		c.order = append(c.order, key)
		for len(c.order) > c.cap {
			delete(c.items, c.order[0])
			c.order = c.order[1:]
		}
	}
	return res
}

func cacheKey(path string, w, h int) string {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Sprintf("%s|%d|%d", path, w, h)
	}
	m := info.ModTime()
	return fmt.Sprintf("%s|%d|%d|%d|%d", path, m.UnixNano(), info.Size(), w, h)
}
