package dev

import (
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupervisor_StartAndStop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("supervisor signal handling is Linux-tested only")
	}
	sleep, err := exec.LookPath("sleep")
	require.NoError(t, err)

	cfg := Config{KillTimeout: 500 * time.Millisecond}
	s := newSupervisor(t.Context(), "test", "", os.Environ(), cfg, sleep, "60")
	require.NoError(t, s.Start())
	require.NotZero(t, s.PID())

	start := time.Now()
	require.NoError(t, s.Stop())
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 200*time.Millisecond,
		"sleep(1) should exit promptly on SIGTERM (got %s)", elapsed)
}

func TestSupervisor_Restart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("supervisor signal handling is Linux-tested only")
	}
	sleep, err := exec.LookPath("sleep")
	require.NoError(t, err)

	cfg := Config{KillTimeout: 500 * time.Millisecond}
	s := newSupervisor(t.Context(), "test", "", os.Environ(), cfg, sleep, "60")
	require.NoError(t, s.Start())
	firstPID := s.PID()

	require.NoError(t, s.Restart())
	secondPID := s.PID()
	assert.NotEqual(t, firstPID, secondPID, "Restart must produce a new PID")
	require.NoError(t, s.Stop())
}

func TestSupervisor_StopWhenAlreadyExited(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("supervisor signal handling is Linux-tested only")
	}
	trueBin, err := exec.LookPath("true")
	require.NoError(t, err)

	cfg := Config{KillTimeout: 500 * time.Millisecond}
	s := newSupervisor(t.Context(), "test", "", os.Environ(), cfg, trueBin)
	require.NoError(t, s.Start())

	// `true` exits ~immediately — Stop must be a safe no-op.
	time.Sleep(50 * time.Millisecond)
	require.NoError(t, s.Stop())
}

// TestSupervisor_HandleUnsolicitedExit_IgnoresStaleObservation pins the
// fix for the monitorAppExits race: when an exit observation lands after
// a Restart has already installed a fresh generation, the late observer
// must NOT cancel the new child.
func TestSupervisor_HandleUnsolicitedExit_IgnoresStaleObservation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("supervisor signal handling is Linux-tested only")
	}
	sleep, err := exec.LookPath("sleep")
	require.NoError(t, err)

	cfg := Config{KillTimeout: 500 * time.Millisecond}
	s := newSupervisor(t.Context(), "test", "", os.Environ(), cfg, sleep, "60")
	require.NoError(t, s.Start())

	firstPID := s.PID()
	firstExited, ok := s.currentExitedChan()
	require.True(t, ok)

	// Kill the first child outside the supervisor and wait for the Wait
	// goroutine to close its exited channel — this is the moment
	// monitorAppExits would normally unblock and start the cleanup.
	require.NoError(t, syscall.Kill(firstPID, syscall.SIGKILL))
	<-firstExited

	// A Restart races ahead before the (still-blocked) cleanup runs:
	// Stop is a no-op against the dead state but resets the slots, then
	// Start installs a fresh generation.
	require.NoError(t, s.Restart())
	secondPID := s.PID()
	secondExited, ok := s.currentExitedChan()
	require.True(t, ok)
	require.NotEqual(t, firstPID, secondPID, "Restart must produce a new PID")

	// Now the late observer wakes up holding the OLD exited channel.
	did, err := s.handleUnsolicitedExit(firstExited)
	require.NoError(t, err)
	assert.False(t, did, "stale observation must not act on the current generation")

	// The fresh child is untouched.
	assert.Equal(t, secondPID, s.PID(), "fresh child must still be the registered process")
	select {
	case <-secondExited:
		t.Fatal("fresh child was killed by the stale-observation handler")
	case <-time.After(50 * time.Millisecond):
	}

	require.NoError(t, s.Stop())
}

// TestSupervisor_HandleUnsolicitedExit_ActsOnCurrentObservation covers
// the happy path: when the observed exited channel still matches the
// supervisor's current generation, cleanup runs and state is cleared.
func TestSupervisor_HandleUnsolicitedExit_ActsOnCurrentObservation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("supervisor signal handling is Linux-tested only")
	}
	sleep, err := exec.LookPath("sleep")
	require.NoError(t, err)

	cfg := Config{KillTimeout: 500 * time.Millisecond}
	s := newSupervisor(t.Context(), "test", "", os.Environ(), cfg, sleep, "60")
	require.NoError(t, s.Start())

	pid := s.PID()
	exited, ok := s.currentExitedChan()
	require.True(t, ok)

	require.NoError(t, syscall.Kill(pid, syscall.SIGKILL))
	<-exited

	did, _ := s.handleUnsolicitedExit(exited)
	assert.True(t, did, "matching observation must run the cleanup path")

	assert.Zero(t, s.PID(), "state must be cleared after cleanup")
	_, ok = s.currentExitedChan()
	assert.False(t, ok, "no exited channel must remain after cleanup")
}

func TestSupervisor_KillsProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("supervisor signal handling is Linux-tested only")
	}
	sh, err := exec.LookPath("sh")
	require.NoError(t, err)

	// Parent shell spawns a sleep child. Without process-group signalling
	// SIGTERM would only kill the shell, leaving sleep orphaned.
	cfg := Config{KillTimeout: 500 * time.Millisecond}
	s := newSupervisor(t.Context(), "test", "", os.Environ(), cfg, sh, "-c", "sleep 60 & wait")
	require.NoError(t, s.Start())

	start := time.Now()
	require.NoError(t, s.Stop())
	assert.Less(t, time.Since(start), 600*time.Millisecond,
		"process-group SIGTERM should bring down the shell + its sleep child quickly")
}
