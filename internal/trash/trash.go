// Package trash moves library files to the FreeDesktop trash via gio: one
// gio call per path, outcomes reported per path. Files are never deleted —
// the trash keeps them recoverable.
package trash

import (
	"os/exec"
	"path"
	"strings"
)

// Run trashes each path and reports outcomes: the basenames that went, and
// the reason for each that didn't. bin overrides the gio binary (tests).
func Run(paths []string, bin string) (removed []string, failed map[string]string) {
	if bin == "" {
		bin, _ = exec.LookPath("gio")
	}
	failed = make(map[string]string)

	for _, p := range paths {
		if bin == "" {
			failed[p] = "gio not found"
			continue
		}
		if out, err := exec.Command(bin, "trash", "--", p).CombinedOutput(); err != nil {
			failed[p] = reason(string(out), err)
			continue
		}
		removed = append(removed, path.Base(p))
	}
	return removed, failed
}

// reason prefers gio's own first line over the bare exit error.
func reason(out string, err error) string {
	if s := strings.TrimSpace(strings.SplitN(out, "\n", 2)[0]); s != "" {
		return s
	}
	return err.Error()
}
