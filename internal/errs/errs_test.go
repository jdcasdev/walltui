package errs

import (
	"errors"
	"fmt"
	"testing"
)

func TestCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, CodeOK},
		{"usage", ErrUsage, CodeUsage},
		{"validation", ErrValidation, CodeValidation},
		{"notfound", ErrNotFound, CodeNotFound},
		{"dep", ErrDependency, CodeDependency},
		{"lock", ErrLock, CodeLock},
		{"integration", ErrIntegration, CodeIntegration},
		{"wrapped lock", fmt.Errorf("apply.lock: %w", ErrLock), CodeLock},
		{"doubly wrapped", fmt.Errorf("acquire: %w", fmt.Errorf("flock: %w", ErrLock)), CodeLock},
		{"unknown", errors.New("boom"), CodeUsage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Code(tt.err); got != tt.want {
				t.Errorf("Code(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
