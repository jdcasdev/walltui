package ui

import (
	"os"
	"testing"
)

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// renderHalfblocks is the test-facing view of renderImage: the rendered
// cells without the source image's pixel dimensions.
func renderHalfblocks(path string, cellW, cellH int) string {
	return renderImage(path, cellW, cellH).cells
}
