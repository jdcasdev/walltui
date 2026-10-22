package backend

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jdcasdev/walltui/internal/errs"
	"github.com/jdcasdev/walltui/internal/state"
)

// Swaybg draws the wallpaper with swaybg. The zero value is production.
type Swaybg struct {
	PidFile      string                                  // file that remembers the wallpaper process
	ProcDir      string                                  // procfs root (default /proc)
	Kill         func(pid int, sig syscall.Signal) error // default unix.Kill
	KillTimeout  time.Duration                           // wait for the old swaybg to die (default 1s)
	StartTimeout time.Duration                           // watch the new swaybg for early death (default 200ms)
	PollInterval time.Duration                           // poll granularity (default 50ms)
}

func (Swaybg) Name() string { return "swaybg" }

// formats are the image formats swaybg draws. PNG always works; the rest
// need gdk-pixbuf2, which the distribution packages ship.
var formats = []string{".png", ".jpg", ".jpeg", ".webp", ".bmp", ".gif"}

func (Swaybg) Formats() []string { return slices.Clone(formats) }

// Apply starts swaybg with the wallpaper and retires the one it replaces.
// The wallpaper on screen is only retired once the replacement is confirmed
// running, so a failed apply leaves it alone.
func (s Swaybg) Apply(path, mode string) error {
	o := s.withDefaults()
	pm := procManager{dir: o.ProcDir, kill: o.Kill, poll: o.PollInterval}

	// The wallpaper to replace: the one walltui started last time. Only that
	// one is retired, never a wallpaper another program or user owns.
	old := s.previousPid()

	cmd, err := startSwaybg(path, mode)
	if err != nil {
		return err
	}
	if err := awaitAlive(cmd, o.StartTimeout, o.PollInterval); err != nil {
		return fmt.Errorf("%w: swaybg failed to start: %v", errs.ErrIntegration, err)
	}
	pm.retire("swaybg", old, o.KillTimeout)
	s.rememberPid(cmd.Process.Pid)
	return nil
}

func (s Swaybg) previousPid() int {
	if s.PidFile == "" {
		return 0
	}
	data, err := state.Get(s.PidFile)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(data))
	if err != nil {
		return 0
	}
	return pid
}

// rememberPid records the wallpaper process, so the next apply can retire it.
// Failing to record it is not an apply failure: the wallpaper is on screen.
func (s Swaybg) rememberPid(pid int) {
	if s.PidFile == "" {
		return
	}
	_ = state.Set(s.PidFile, strconv.Itoa(pid))
}

// startSwaybg launches swaybg in its own process group so it outlives the
// caller (the TUI, or a short-lived CLI) without inheriting our lock.
func startSwaybg(path, mode string) (*exec.Cmd, error) {
	bin, err := exec.LookPath("swaybg")
	if err != nil {
		return nil, fmt.Errorf("%w: missing commands: swaybg", errs.ErrDependency)
	}

	cmd := exec.Command(bin, "-i", path, "-m", mode)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err == nil {
		cmd.Stdout, cmd.Stderr = devnull, devnull
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: cannot start swaybg: %v", errs.ErrIntegration, err)
	}
	return cmd, nil
}

// awaitAlive watches the child: nil while it keeps running, an error as soon
// as it exits inside the window. The Wait goroutine reaps it and closes the
// inherited /dev/null descriptors.
func awaitAlive(cmd *exec.Cmd, timeout, poll time.Duration) error {
	exited := make(chan error, 1)
	go func() {
		exited <- cmd.Wait()
		if f, ok := cmd.Stdout.(*os.File); ok {
			f.Close()
		}
	}()

	tick := time.NewTicker(poll)
	defer tick.Stop()
	deadline := time.After(timeout)
	for {
		select {
		case err := <-exited:
			return fmt.Errorf("process %d exited early: %v", cmd.Process.Pid, err)
		case <-deadline:
			return nil
		case <-tick.C:
		}
	}
}

func (s Swaybg) withDefaults() Swaybg {
	if s.ProcDir == "" {
		s.ProcDir = "/proc"
	}
	if s.Kill == nil {
		s.Kill = unixKill
	}
	if s.KillTimeout == 0 {
		s.KillTimeout = time.Second
	}
	if s.StartTimeout == 0 {
		s.StartTimeout = 200 * time.Millisecond
	}
	if s.PollInterval == 0 {
		s.PollInterval = 50 * time.Millisecond
	}
	return s
}
