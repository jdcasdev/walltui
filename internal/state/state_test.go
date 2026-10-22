package state

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jdcasdev/walltui/internal/errs"
)

func TestGetMissing(t *testing.T) {
	got, err := Get(filepath.Join(t.TempDir(), "current"))
	if err != nil || got != "" {
		t.Errorf("Get() = %q, %v; want empty, nil", got, err)
	}
}

func TestSetGetRoundtrip(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "state", "current")
	const want = "/walls/nature.png"

	if err := Set(stateFile, want); err != nil {
		t.Fatalf("Set() = %v", err)
	}

	got, err := Get(stateFile)
	if err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if got != want {
		t.Errorf("Get() = %q, want %q", got, want)
	}

	info, err := os.Stat(stateFile)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("state file mode = %o, want 600", perm)
	}
}

func TestSetEmpty(t *testing.T) {
	if err := Set(filepath.Join(t.TempDir(), "current"), ""); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("Set(\"\") = %v, want validation error", err)
	}
}

func TestSetLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "current")
	if err := Set(stateFile, "/walls/a.png"); err != nil {
		t.Fatal(err)
	}
	if err := Set(stateFile, "/walls/b.png"); err != nil {
		t.Fatal(err)
	}
	leftovers, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range leftovers {
		if strings.HasPrefix(e.Name(), ".current.") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
	if got, _ := Get(stateFile); got != "/walls/b.png" {
		t.Errorf("Get() = %q, want /walls/b.png", got)
	}
}

func TestClear(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "current")
	if err := Clear(stateFile); err != nil {
		t.Fatalf("Clear(missing) = %v", err)
	}
	if err := Set(stateFile, "/walls/a.png"); err != nil {
		t.Fatal(err)
	}
	if err := Clear(stateFile); err != nil {
		t.Fatalf("Clear() = %v", err)
	}
	if got, err := Get(stateFile); err != nil || got != "" {
		t.Errorf("Get() after Clear = %q, %v; want empty, nil", got, err)
	}
}

func TestLockMutualExclusion(t *testing.T) {
	lockFile := filepath.Join(t.TempDir(), "apply.lock")

	l, err := Acquire(lockFile, time.Second)
	if err != nil {
		t.Fatalf("Acquire() = %v", err)
	}

	if _, err := Acquire(lockFile, 150*time.Millisecond); !errors.Is(err, errs.ErrLock) {
		t.Errorf("second Acquire() = %v, want lock timeout", err)
	}

	if err := l.Release(); err != nil {
		t.Fatalf("Release() = %v", err)
	}

	l2, err := Acquire(lockFile, time.Second)
	if err != nil {
		t.Fatalf("Acquire() after release = %v", err)
	}
	l2.Release()
}

func TestLockContentionSameProcess(t *testing.T) {
	// flock is per open file description: two opens in one process still
	// contend, mirroring the cross-process apply serialization.
	lockFile := filepath.Join(t.TempDir(), "apply.lock")
	l1, err := Acquire(lockFile, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer l1.Release()

	if _, err := Acquire(lockFile, 100*time.Millisecond); !errors.Is(err, errs.ErrLock) {
		t.Errorf("second open contended, got %v, want lock timeout", err)
	}
}
