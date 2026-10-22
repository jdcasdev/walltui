package backend

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jdcasdev/walltui/internal/errs"
	"github.com/jdcasdev/walltui/internal/state"
)

// testEnv bundles a fake swaybg on PATH, an empty fake procfs, and recorders.
type testEnv struct {
	t       *testing.T
	log     string // swaybg argument log
	pids    string // swaybg pid log
	procDir string
	pidFile string // where the backend records the wallpaper process
	kills   *killRecorder
}

type killCall struct {
	pid int
	sig syscall.Signal
}

type killRecorder struct{ calls []killCall }

func (k *killRecorder) kill(pid int, sig syscall.Signal) error {
	k.calls = append(k.calls, killCall{pid, sig})
	return nil
}

// newEnv builds the test environment. scriptBody overrides the fake swaybg
// body (default: log args, record pid, sleep).
func newEnv(t *testing.T, scriptBody string) *testEnv {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	procDir := filepath.Join(root, "proc")
	for _, d := range []string{bin, procDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	env := &testEnv{
		t:       t,
		log:     filepath.Join(root, "swaybg.log"),
		pids:    filepath.Join(root, "swaybg.pids"),
		procDir: procDir,
		pidFile: filepath.Join(root, "swaybg.pid"),
		kills:   &killRecorder{},
	}

	if scriptBody == "" {
		scriptBody = "#!/bin/sh\n" +
			"printf '%s\\n' \"$*\" >> \"$WALLTUI_TEST_LOG\"\n" +
			"echo $$ >> \"$WALLTUI_TEST_PIDS\"\n" +
			"sleep 30\n"
	}
	script := "#!/bin/sh\n" + scriptBody
	if err := os.WriteFile(filepath.Join(bin, "swaybg"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("WALLTUI_TEST_LOG", env.log)
	t.Setenv("WALLTUI_TEST_PIDS", env.pids)

	t.Cleanup(func() {
		data, _ := os.ReadFile(env.pids)
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			var pid int
			if _, err := fmt.Sscanf(line, "%d", &pid); err == nil {
				syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	return env
}

func (e *testEnv) backend() Swaybg {
	return Swaybg{
		PidFile:      e.pidFile,
		ProcDir:      e.procDir,
		Kill:         e.kills.kill,
		KillTimeout:  100 * time.Millisecond,
		StartTimeout: 300 * time.Millisecond,
		PollInterval: 10 * time.Millisecond,
	}
}

// remember makes the backend believe it started the given pid last time.
func (e *testEnv) remember(pid int) {
	e.t.Helper()
	if err := state.Set(e.pidFile, strconv.Itoa(pid)); err != nil {
		e.t.Fatal(err)
	}
}

func (e *testEnv) swaybgLog(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(e.log)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestName(t *testing.T) {
	if got := (Swaybg{}).Name(); got != "swaybg" {
		t.Errorf("Name() = %q, want swaybg", got)
	}
}

func TestApplyStartsSwaybg(t *testing.T) {
	env := newEnv(t, "")

	if err := env.backend().Apply("/walls/a.png", "fill"); err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	want := "-i /walls/a.png -m fill"
	if log := strings.TrimSpace(env.swaybgLog(t)); log != want {
		t.Errorf("swaybg args = %q, want %q", log, want)
	}
}

func TestApplyRetiresOldWallpaper(t *testing.T) {
	env := newEnv(t, "")
	mkproc(t, env.procDir, "500", "swaybg") // the wallpaper walltui started
	env.remember(500)

	if err := env.backend().Apply("/walls/a.png", "fill"); err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	if len(env.kills.calls) == 0 {
		t.Fatal("the wallpaper on screen was not retired")
	}
	if env.kills.calls[0].pid != 500 || env.kills.calls[0].sig != syscall.SIGTERM {
		t.Errorf("kill calls = %v, want SIGTERM to 500", env.kills.calls)
	}
}

// TestApplyLeavesForeignWallpaper: a wallpaper walltui did not start (another
// program, another user) must never be killed.
func TestApplyLeavesForeignWallpaper(t *testing.T) {
	env := newEnv(t, "")
	mkproc(t, env.procDir, "600", "swaybg") // someone else's wallpaper
	env.remember(500)                       // ours is gone already

	if err := env.backend().Apply("/walls/a.png", "fill"); err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	if len(env.kills.calls) != 0 {
		t.Errorf("a foreign wallpaper was killed: %v", env.kills.calls)
	}
}

// TestApplyIgnoresRecycledPid: a pid that now belongs to another program
// must not be killed.
func TestApplyIgnoresRecycledPid(t *testing.T) {
	env := newEnv(t, "")
	mkproc(t, env.procDir, "500", "firefox")
	env.remember(500)

	if err := env.backend().Apply("/walls/a.png", "fill"); err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	if len(env.kills.calls) != 0 {
		t.Errorf("a recycled pid was killed: %v", env.kills.calls)
	}
}

// TestApplyRecordsWallpaperProcess: the next apply must know which process
// to retire.
func TestApplyRecordsWallpaperProcess(t *testing.T) {
	env := newEnv(t, "")

	if err := env.backend().Apply("/walls/a.png", "fill"); err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	data, err := os.ReadFile(env.pidFile)
	if err != nil {
		t.Fatalf("pid file: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		t.Errorf("pid file = %q, want a pid", strings.TrimSpace(string(data)))
	}
}

// waylandLoggingSwaybg is a fake swaybg that logs the WAYLAND_SOCKET it sees.
const waylandLoggingSwaybg = "printf '%s\\n' \"${WAYLAND_SOCKET-unset}\" >> \"$WALLTUI_TEST_LOG\"\necho $$ >> \"$WALLTUI_TEST_PIDS\"\nsleep 30\n"

// TestApplyDropsWaylandSocket: sway's own background command inherits a
// WAYLAND_SOCKET that sway closes on reload. The swaybg walltui starts must
// connect through WAYLAND_DISPLAY instead, or the wallpaper dies on a reload.
func TestApplyDropsWaylandSocket(t *testing.T) {
	env := newEnv(t, waylandLoggingSwaybg)
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")
	t.Setenv("WAYLAND_SOCKET", "42")

	if err := env.backend().Apply("/walls/a.png", "fill"); err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	if log := strings.TrimSpace(env.swaybgLog(t)); log != "unset" {
		t.Errorf("swaybg saw WAYLAND_SOCKET=%q, want it unset", log)
	}
}

// TestApplyKeepsWaylandSocketWithoutDisplay: with no WAYLAND_DISPLAY to fall
// back to, the socket is the only way to reach the compositor, so it stays.
func TestApplyKeepsWaylandSocketWithoutDisplay(t *testing.T) {
	env := newEnv(t, waylandLoggingSwaybg)
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")
	os.Unsetenv("WAYLAND_DISPLAY") // cleanup restores the real value
	t.Setenv("WAYLAND_SOCKET", "42")

	if err := env.backend().Apply("/walls/a.png", "fill"); err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	if log := strings.TrimSpace(env.swaybgLog(t)); log != "42" {
		t.Errorf("swaybg saw WAYLAND_SOCKET=%q, want 42", log)
	}
}

func TestApplyCrash(t *testing.T) {
	env := newEnv(t, "#!/bin/sh\nexit 1\n")

	if err := env.backend().Apply("/walls/a.png", "fill"); !errors.Is(err, errs.ErrIntegration) {
		t.Errorf("Apply() = %v, want integration error", err)
	}
}

// TestApplyCrashKeepsOldWallpaper: the wallpaper on screen is only retired
// once the replacement is known to work.
func TestApplyCrashKeepsOldWallpaper(t *testing.T) {
	env := newEnv(t, "#!/bin/sh\nexit 1\n")
	mkproc(t, env.procDir, "500", "swaybg")
	env.remember(500)

	if err := env.backend().Apply("/walls/a.png", "fill"); !errors.Is(err, errs.ErrIntegration) {
		t.Fatalf("Apply() = %v, want integration error", err)
	}
	if len(env.kills.calls) != 0 {
		t.Errorf("the wallpaper on screen was killed: %v", env.kills.calls)
	}
}

func TestApplyMissingSwaybg(t *testing.T) {
	env := newEnv(t, "")
	t.Setenv("PATH", filepath.Dir(env.log)) // no swaybg anywhere

	if err := env.backend().Apply("/walls/a.png", "fill"); !errors.Is(err, errs.ErrDependency) {
		t.Errorf("Apply() = %v, want dependency error", err)
	}
}
