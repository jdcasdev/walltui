package apply

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jdcasdev/walltui/internal/config"
	"github.com/jdcasdev/walltui/internal/errs"
	"github.com/jdcasdev/walltui/internal/state"
)

// fakeBackend records what it was asked to apply.
type fakeBackend struct {
	err     error
	formats []string
	calls   []string
}

func (f *fakeBackend) Name() string { return "fake" }

func (f *fakeBackend) Formats() []string { return f.formats }

func (f *fakeBackend) Apply(path, mode string) error {
	f.calls = append(f.calls, path+" "+mode)
	return f.err
}

// testEnv bundles a library dir, a state dir and the fake backend.
type testEnv struct {
	cfg     config.Config
	backend *fakeBackend
}

func newEnv(t *testing.T) *testEnv {
	t.Helper()
	root := t.TempDir()
	walls := filepath.Join(root, "walls")
	stateDir := filepath.Join(root, "state")
	for _, d := range []string{walls, stateDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return &testEnv{
		cfg: config.Config{
			WallpaperDir: walls,
			FitMode:      "fill",
			StateDir:     stateDir,
			StateFile:    filepath.Join(stateDir, "current"),
			LockFile:     filepath.Join(stateDir, "apply.lock"),
		},
		backend: &fakeBackend{formats: []string{".png"}},
	}
}

func (e *testEnv) options() Options {
	return Options{Backend: e.backend, LockTimeout: 200 * time.Millisecond}
}

func addWall(t *testing.T, env *testEnv, name string) string {
	t.Helper()
	path := filepath.Join(env.cfg.WallpaperDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunExplicitArg(t *testing.T) {
	env := newEnv(t)
	b := addWall(t, env, "b.png")
	addWall(t, env, "a.png")

	entry, err := Run(env.cfg, b, env.options())
	if err != nil {
		t.Fatalf("Run() = %v", err)
	}
	if entry.Rel != "b.png" {
		t.Errorf("entry.Rel = %q, want b.png", entry.Rel)
	}
	if len(env.backend.calls) != 1 || env.backend.calls[0] != b+" fill" {
		t.Errorf("backend calls = %v, want one call to %q", env.backend.calls, b+" fill")
	}

	cur, err := state.Get(env.cfg.StateFile)
	if err != nil || cur != b {
		t.Errorf("state = %q, %v; want %q, nil", cur, err, b)
	}

	// The lock must be released: a second apply acquires immediately.
	l, err := state.Acquire(env.cfg.LockFile, 100*time.Millisecond)
	if err != nil {
		t.Errorf("lock still held after Run: %v", err)
	}
	l.Release()
}

func TestRunArgOutsideLibrary(t *testing.T) {
	env := newEnv(t)
	outside := filepath.Join(t.TempDir(), "out.png")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(env.cfg, outside, env.options()); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("Run() = %v, want validation error", err)
	}
	if len(env.backend.calls) != 0 {
		t.Errorf("backend was called despite invalid arg: %v", env.backend.calls)
	}
	if cur, _ := state.Get(env.cfg.StateFile); cur != "" {
		t.Errorf("state written despite invalid arg: %q", cur)
	}
}

func TestRunFallbackChain(t *testing.T) {
	t.Run("empty state applies first sorted", func(t *testing.T) {
		env := newEnv(t)
		first := addWall(t, env, "1-first.png")
		addWall(t, env, "10-second.png")

		if _, err := Run(env.cfg, "", env.options()); err != nil {
			t.Fatalf("Run() = %v", err)
		}
		if len(env.backend.calls) != 1 || !strings.Contains(env.backend.calls[0], first) {
			t.Errorf("backend calls = %v, want %q", env.backend.calls, first)
		}
	})

	t.Run("stale state falls through to first", func(t *testing.T) {
		env := newEnv(t)
		first := addWall(t, env, "1-first.png")
		if err := state.Set(env.cfg.StateFile, filepath.Join(env.cfg.WallpaperDir, "ghost.png")); err != nil {
			t.Fatal(err)
		}

		if _, err := Run(env.cfg, "", env.options()); err != nil {
			t.Fatalf("Run() = %v", err)
		}
		if len(env.backend.calls) != 1 || !strings.Contains(env.backend.calls[0], first) {
			t.Errorf("backend calls = %v, want %q", env.backend.calls, first)
		}
	})

	t.Run("valid state is applied", func(t *testing.T) {
		env := newEnv(t)
		a := addWall(t, env, "a.png")
		b := addWall(t, env, "b.png")
		if err := state.Set(env.cfg.StateFile, b); err != nil {
			t.Fatal(err)
		}

		if _, err := Run(env.cfg, "", env.options()); err != nil {
			t.Fatalf("Run() = %v", err)
		}
		if len(env.backend.calls) != 1 || !strings.Contains(env.backend.calls[0], b) || strings.Contains(env.backend.calls[0], a) {
			t.Errorf("backend calls = %v, want only %q", env.backend.calls, b)
		}
	})

	t.Run("empty library", func(t *testing.T) {
		env := newEnv(t)
		if _, err := Run(env.cfg, "", env.options()); !errors.Is(err, errs.ErrNotFound) {
			t.Errorf("Run() = %v, want not-found error", err)
		}
	})
}

func TestRunLockContention(t *testing.T) {
	env := newEnv(t)
	addWall(t, env, "a.png")

	held, err := state.Acquire(env.cfg.LockFile, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()

	if _, err := Run(env.cfg, "", env.options()); !errors.Is(err, errs.ErrLock) {
		t.Errorf("Run() = %v, want lock timeout", err)
	}
	if len(env.backend.calls) != 0 {
		t.Errorf("backend called while lock held: %v", env.backend.calls)
	}
}

func TestRunBackendFailure(t *testing.T) {
	env := newEnv(t)
	addWall(t, env, "a.png")
	env.backend.err = errors.New("boom")

	if _, err := Run(env.cfg, "", env.options()); err == nil {
		t.Error("Run() = nil, want the backend error")
	}
	if cur, _ := state.Get(env.cfg.StateFile); cur != "" {
		t.Errorf("state written despite failure: %q", cur)
	}
}

func TestRunInvalidMode(t *testing.T) {
	env := newEnv(t)
	addWall(t, env, "a.png")
	env.cfg.FitMode = "tilee"

	if _, err := Run(env.cfg, "", env.options()); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("Run() = %v, want validation error", err)
	}
}
