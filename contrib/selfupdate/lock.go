package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// acquireUpdateLock takes an exclusive lock keyed off the running
// executable's path, so concurrent `<app> update` invocations don't
// race on the same target binary. Returns a release function the
// caller defers; release closes the lock file but leaves it on disk
// (the OS releases the fcntl/flock when the fd closes).
//
// Implementation lives in lock_unix.go / lock_windows.go. The
// platform-independent piece is the lock-file path derivation —
// it lives next to the running executable's path so different
// installed binaries don't collide on a shared lock.
func acquireUpdateLock() (release func(), err error) {
	path, err := lockPath()
	if err != nil {
		return nil, err
	}
	return acquireFlock(path)
}

func lockPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("selfupdate: locate own executable for lock file: %w", err)
	}
	// Hash the full path so the lock file name is stable but doesn't
	// leak the exec path into /tmp; collisions across different
	// binaries are vanishingly unlikely with SHA-256.
	sum := sha256.Sum256([]byte(exe))
	return filepath.Join(os.TempDir(), "burrow-selfupdate-"+hex.EncodeToString(sum[:8])+".lock"), nil
}
