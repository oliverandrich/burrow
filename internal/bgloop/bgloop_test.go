package bgloop

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useLogger swaps the default slog logger for one writing into buf and
// restores it after the test.
func useLogger(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestRecover_SwallowsPanicAndLogs(t *testing.T) {
	buf := useLogger(t)

	require.NotPanics(t, func() {
		defer Recover("test.loop")
		panic("boom")
	})

	out := buf.String()
	assert.Contains(t, out, "background loop")
	assert.Contains(t, out, "test.loop")
	assert.Contains(t, out, "boom")
	assert.Contains(t, out, "bgloop_test.go", "stack trace should be logged")
}

func TestRecover_NoPanicNoLog(t *testing.T) {
	buf := useLogger(t)

	require.NotPanics(t, func() {
		defer Recover("test.loop")
	})

	assert.Empty(t, buf.String(), "no panic must not log anything")
}

// TestRecover_LoopSurvives pins the intended call-site pattern: a panic in one
// iteration is contained, and subsequent iterations keep running.
func TestRecover_LoopSurvives(t *testing.T) {
	_ = useLogger(t)

	iterations := 0
	for i := range 3 {
		func() {
			defer Recover("test.loop")
			iterations++
			if i == 0 {
				panic("first iteration explodes")
			}
		}()
	}

	assert.Equal(t, 3, iterations, "loop must continue after a panicking iteration")
}
