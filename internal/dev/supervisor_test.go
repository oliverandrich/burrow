package dev

import (
	"os"
	"os/exec"
	"runtime"
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
