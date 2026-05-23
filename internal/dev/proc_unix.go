//go:build !windows

package dev

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// setProcessGroup makes the child the leader of a new process group,
// so a single signal to -pgid reaches the child and every descendant
// it spawns (e.g. `go run`'s rebuilt binary).
func setProcessGroup(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.Setpgid = true
}

// processGroupID returns the process group id of p captured at the
// moment of the call. Used by the supervisor to record the pgid right
// after Start, so the value survives Wait reaping the child (which
// would otherwise leave the pid recyclable by the kernel).
func processGroupID(p *os.Process) (int, error) {
	return syscall.Getpgid(p.Pid)
}

// terminateGroup sends SIGTERM to the supplied process group. Wired
// into exec.Cmd.Cancel; exec.Cmd.WaitDelay then escalates to SIGKILL
// automatically when the timeout elapses.
//
// Falls back to signalling the bare process when pgid is zero or
// negative (process group never established — happens if Getpgid
// returned an error at Start time).
func terminateGroup(p *os.Process, pgid int) error {
	if pgid <= 0 {
		if err := p.Signal(syscall.SIGTERM); err != nil {
			if errors.Is(err, os.ErrProcessDone) {
				return os.ErrProcessDone
			}
			return err
		}
		return nil
	}
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return nil
}
