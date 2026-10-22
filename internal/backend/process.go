package backend

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// procManager finds processes by their comm name and signals them by pid.
// procDir and kill are injectable so tests can drive a fake procfs without
// touching real processes; production uses /proc and unix.Kill.
type procManager struct {
	dir  string
	kill func(pid int, sig syscall.Signal) error
	poll time.Duration
}

func unixKill(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}

// pids lists matching processes. Races with process exit are normal: a comm
// file vanishing mid-scan just means the process died.
func (pm procManager) pids(name string) []int {
	entries, err := os.ReadDir(pm.dir)
	if err != nil {
		return nil
	}
	var pids []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || !e.IsDir() {
			continue
		}
		comm, err := os.ReadFile(filepath.Join(pm.dir, e.Name(), "comm"))
		if err != nil {
			continue // the process exited mid-scan
		}
		if strings.TrimSpace(string(comm)) == name {
			pids = append(pids, pid)
		}
	}
	return pids
}

// retire kills pid, but only while it still runs a process with that name:
// a recycled pid must never be killed. A pid of 0 means "nothing to retire".
func (pm procManager) retire(name string, pid int, timeout time.Duration) {
	if pid <= 0 {
		return
	}
	for _, p := range pm.pids(name) {
		if p == pid {
			pm.killPids([]int{pid}, timeout)
			return
		}
	}
}

// killPids SIGTERMs every given pid, escalating to SIGKILL, and waits up to
// timeout for each stage. Like `pkill ... || true`, a process surviving the
// escalation cannot block the apply.
func (pm procManager) killPids(pids []int, timeout time.Duration) {
	if len(pids) == 0 {
		return
	}
	signalAll(pm.kill, pids, syscall.SIGTERM)
	if pm.waitPids(pids, timeout) {
		return
	}
	signalAll(pm.kill, pids, syscall.SIGKILL)
	pm.waitPids(pids, timeout/2)
}

func (pm procManager) waitPids(pids []int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if !pm.anyAlive(pids) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(pm.poll)
	}
}

func (pm procManager) anyAlive(pids []int) bool {
	for _, pid := range pids {
		if _, err := os.Stat(filepath.Join(pm.dir, strconv.Itoa(pid))); err == nil {
			return true
		}
	}
	return false
}

func signalAll(kill func(int, syscall.Signal) error, pids []int, sig syscall.Signal) {
	for _, pid := range pids {
		_ = kill(pid, sig) // an already-gone process is success
	}
}
