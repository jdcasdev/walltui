// Package errs defines the shared error kinds and their exit codes. Call
// sites wrap a sentinel with %w to carry human context; main maps the error
// to an exit code with Code.
package errs

import "errors"

// Sentinel error kinds. Match with errors.Is, never compare text.
var (
	ErrUsage       = errors.New("usage error")
	ErrValidation  = errors.New("validation error")
	ErrNotFound    = errors.New("not found")
	ErrDependency  = errors.New("missing dependency")
	ErrLock        = errors.New("lock timeout")
	ErrIntegration = errors.New("integration failure")
)

// Exit codes: 0 ok, 1 usage, 2 validation, 3 not found, 4 missing
// dependency, 5 lock timeout, 6 integration failure.
const (
	CodeOK          = 0
	CodeUsage       = 1
	CodeValidation  = 2
	CodeNotFound    = 3
	CodeDependency  = 4
	CodeLock        = 5
	CodeIntegration = 6
)

// Code maps an error to its exit code. Unknown errors exit 1 (usage).
func Code(err error) int {
	switch {
	case err == nil:
		return CodeOK
	case errors.Is(err, ErrUsage):
		return CodeUsage
	case errors.Is(err, ErrValidation):
		return CodeValidation
	case errors.Is(err, ErrNotFound):
		return CodeNotFound
	case errors.Is(err, ErrDependency):
		return CodeDependency
	case errors.Is(err, ErrLock):
		return CodeLock
	case errors.Is(err, ErrIntegration):
		return CodeIntegration
	default:
		return CodeUsage
	}
}
