package backend

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func mkproc(t *testing.T, root, pid, comm string) {
	t.Helper()
	dir := filepath.Join(root, pid)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "comm"), []byte(comm+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPids(t *testing.T) {
	root := t.TempDir()
	mkproc(t, root, "123", "swaybg")
	mkproc(t, root, "137", "sleep")
	mkproc(t, root, "999", "swaybg")
	// a pid dir without a comm file: the process died mid-scan
	if err := os.MkdirAll(filepath.Join(root, "140"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "notapid"), 0o700); err != nil {
		t.Fatal(err)
	}

	pm := procManager{dir: root}
	pids := pm.pids("swaybg")
	want := []int{123, 999}
	if len(pids) != len(want) || pids[0] != want[0] || pids[1] != want[1] {
		t.Errorf("pids() = %v, want %v", pids, want)
	}
}

func TestKillPidsTermedGone(t *testing.T) {
	root := t.TempDir()
	mkproc(t, root, "500", "swaybg")

	var rec killRecorder
	pm := procManager{
		dir: root,
		kill: func(pid int, sig syscall.Signal) error {
			rec.calls = append(rec.calls, killCall{pid, sig})
			return os.RemoveAll(filepath.Join(root, "500")) // process exits on TERM
		},
		poll: 10 * time.Millisecond,
	}

	start := time.Now()
	pm.killPids([]int{500}, 500*time.Millisecond)
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Errorf("killPids took %v, want quick return once processes are gone", elapsed)
	}
	if len(rec.calls) != 1 || rec.calls[0].sig != syscall.SIGTERM {
		t.Errorf("kill calls = %v, want a single SIGTERM", rec.calls)
	}
}

func TestKillPidsEscalates(t *testing.T) {
	root := t.TempDir()
	mkproc(t, root, "500", "swaybg")

	var rec killRecorder
	pm := procManager{
		dir:  root,
		kill: func(pid int, sig syscall.Signal) error { rec.calls = append(rec.calls, killCall{pid, sig}); return nil },
		poll: 10 * time.Millisecond,
	}

	pm.killPids([]int{500}, 60*time.Millisecond)
	if len(rec.calls) < 2 {
		t.Fatalf("kill calls = %v, want TERM escalation to KILL", rec.calls)
	}
	if rec.calls[0].sig != syscall.SIGTERM || rec.calls[len(rec.calls)-1].sig != syscall.SIGKILL {
		t.Errorf("kill calls = %v, want first TERM and last KILL", rec.calls)
	}
}

func TestWaitPidsTimeout(t *testing.T) {
	root := t.TempDir()
	mkproc(t, root, "500", "swaybg")

	pm := procManager{dir: root, poll: 10 * time.Millisecond}
	if pm.waitPids([]int{500}, 50*time.Millisecond) {
		t.Error("waitPids() = true, want false (the pid is still there)")
	}
}

func TestKillIgnoresGoneProcess(t *testing.T) {
	root := t.TempDir()
	mkproc(t, root, "500", "swaybg")

	pm := procManager{
		dir: root,
		kill: func(pid int, sig syscall.Signal) error {
			return os.RemoveAll(filepath.Join(root, "500"))
		},
		poll: 10 * time.Millisecond,
	}
	pm.killPids([]int{500}, 100*time.Millisecond) // vanishing target must not error or hang
}
