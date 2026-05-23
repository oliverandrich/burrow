package dev

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

// supervisor manages the lifecycle of a single long-running child
// process. It serialises Start / Stop / Restart calls and uses
// platform-specific signalling (process groups on Unix, plain Kill on
// Windows) to bring down the child and any sub-processes it spawned.
//
// The supervisor owns a parent context; each Start derives a child
// context cancelled by Stop. The exec package's Cancel + WaitDelay
// hooks then handle SIGTERM-then-SIGKILL on our behalf.
type supervisor struct {
	parentCtx context.Context //nolint:containedctx // long-lived supervisor owns its own ctx by design.
	name      string          // short label used in log lines (e.g. "app", "tailwind")
	dir       string          // working directory; "" means current
	env       []string        // full environment for the child
	cmd       []string        // [bin, args...]
	cfg       Config

	mu      sync.Mutex
	proc    *os.Process
	exited  chan struct{} // closed when the current child has exited
	cancel  context.CancelFunc
	exitErr error // populated before close(exited); read under mu
}

func newSupervisor(parentCtx context.Context, name, dir string, env []string, cfg Config, command ...string) *supervisor {
	return &supervisor{
		parentCtx: parentCtx,
		name:      name,
		dir:       dir,
		env:       env,
		cmd:       command,
		cfg:       cfg,
	}
}

// PID returns the current child's PID, or 0 when no child is running.
func (s *supervisor) PID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.proc == nil {
		return 0
	}
	return s.proc.Pid
}

// currentExitedChan returns the current child's exit channel and true
// when one is running. Returns (nil, false) when no child is alive.
// Callers select on the returned channel to observe unsolicited exits.
func (s *supervisor) currentExitedChan() (<-chan struct{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.exited == nil {
		return nil, false
	}
	return s.exited, true
}

// clearExited resets the supervisor state after an unsolicited exit
// observed via currentExitedChan. Returns the exit error captured by
// the Wait goroutine (nil for clean exit). Safe to call when no child
// is or was running — returns nil.
func (s *supervisor) clearExited() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.exitErr
	if s.cancel != nil {
		s.cancel()
	}
	s.proc = nil
	s.exited = nil
	s.cancel = nil
	s.exitErr = nil
	return err
}

// Start launches the configured command. It is an error to call
// Start while a child is already running — use Restart for that.
func (s *supervisor) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startLocked()
}

func (s *supervisor) startLocked() error {
	if s.proc != nil {
		return fmt.Errorf("dev: supervisor %q: already running (pid=%d)", s.name, s.proc.Pid)
	}
	if len(s.cmd) == 0 {
		return errors.New("dev: supervisor: empty command")
	}

	childCtx, cancel := context.WithCancel(s.parentCtx)
	c := exec.CommandContext(childCtx, s.cmd[0], s.cmd[1:]...) //nolint:gosec // dev-tooling, command is configured by the project owner.
	c.Dir = s.dir
	c.Env = s.env
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.WaitDelay = s.cfg.KillTimeout
	setProcessGroup(c)

	if err := c.Start(); err != nil {
		cancel()
		return fmt.Errorf("dev: supervisor %q: start: %w", s.name, err)
	}

	// Cache the process group id once at Start, then use the cached
	// value in Cancel. Reading c.Process.Pid at cancel time would
	// race with Wait reaping the child — the kernel could recycle the
	// PID and our signal would land on an unrelated process group.
	pgid, _ := processGroupID(c.Process)
	c.Cancel = func() error { return terminateGroup(c.Process, pgid) }

	s.proc = c.Process
	s.cancel = cancel
	exited := make(chan struct{})
	s.exited = exited
	go func() {
		// c.Wait blocks until the child exits and its pipes are
		// drained (or WaitDelay elapses). Publish exitErr under the
		// lock so concurrent readers (currentExitedChan callers) see
		// the value without a data race.
		err := c.Wait()
		s.mu.Lock()
		s.exitErr = err
		s.mu.Unlock()
		close(exited)
	}()
	return nil
}

// Stop cancels the current child's context (which triggers SIGTERM
// to the process group and SIGKILL after KillTimeout via exec.Cmd's
// Cancel + WaitDelay hooks) and waits for Wait to return. Safe to
// call when no child is running.
func (s *supervisor) Stop() error {
	s.mu.Lock()
	if s.proc == nil {
		s.mu.Unlock()
		return nil
	}
	exited := s.exited
	cancel := s.cancel
	// Clear state immediately so concurrent Start callers see "no
	// child" rather than "stale dead PID". The Wait goroutine
	// continues to run against the captured local references.
	s.proc = nil
	s.exited = nil
	s.cancel = nil
	s.mu.Unlock()

	cancel()
	if exited != nil {
		<-exited
	}
	return nil
}

// Restart stops the current child (if any) and starts a new one.
func (s *supervisor) Restart() error {
	if err := s.Stop(); err != nil {
		return err
	}
	return s.Start()
}
