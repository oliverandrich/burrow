//go:build !windows

package selfupdate

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// acquireFlock takes a non-blocking exclusive lock on path. Returns
// an error wrapping syscall.EWOULDBLOCK when another process holds
// the lock — the caller should surface that as "another update is
// already running".
func acquireFlock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600) //nolint:gosec // dev-tooling lock file; path is hashed.
	if err != nil {
		return nil, fmt.Errorf("selfupdate: open lock file %s: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("selfupdate: another update is already running (lock held on %s)", path)
		}
		return nil, fmt.Errorf("selfupdate: flock %s: %w", path, err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
