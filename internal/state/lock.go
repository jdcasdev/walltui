package state

import (
	"errors"
	"fmt"
	"os"
	"time"

	"syscall"

	"github.com/jdcasdev/walltui/internal/errs"
)

// DefaultLockTimeout is how long an apply waits for the lock before it
// gives up.
const DefaultLockTimeout = 10 * time.Second

// Lock is an exclusive flock held on the apply lock file.
type Lock struct {
	file *os.File
}

// Acquire takes the exclusive lock, polling until timeout.
func Acquire(lockFile string, timeout time.Duration) (*Lock, error) {
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot open lock file %s: %v", errs.ErrIntegration, lockFile, err)
	}

	deadline := time.Now().Add(timeout)
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		switch {
		case err == nil:
			return &Lock{file: f}, nil
		case !errorsIsWouldBlock(err):
			f.Close()
			return nil, fmt.Errorf("%w: flock %s: %v", errs.ErrIntegration, lockFile, err)
		case time.Now().After(deadline):
			f.Close()
			return nil, fmt.Errorf("%w: another apply is holding %s", errs.ErrLock, lockFile)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (l *Lock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	if cerr := l.file.Close(); err == nil {
		err = cerr
	}
	l.file = nil
	if err != nil {
		return fmt.Errorf("%w: releasing lock: %v", errs.ErrIntegration, err)
	}
	return nil
}

// errorsIsWouldBlock reports whether err is EWOULDBLOCK (== EAGAIN on Linux).
func errorsIsWouldBlock(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)
}
