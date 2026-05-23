//go:build windows

package dev

import (
	"os"
	"os/exec"
)

// setProcessGroup is a no-op on Windows: there is no portable
// equivalent to Unix process groups in os/exec. Best-effort support
// only — burrow dev is not tested on Windows.
func setProcessGroup(_ *exec.Cmd) {}

// processGroupID is meaningless on Windows; return 0 so the
// caller's fall-back path (signal the bare process) is taken.
func processGroupID(_ *os.Process) (int, error) { return 0, nil }

// terminateGroup kills the child directly. Windows has no graceful-
// shutdown signal exposed via os/exec.
func terminateGroup(p *os.Process, _ int) error {
	return p.Kill()
}
