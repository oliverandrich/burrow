//go:build !windows

package burrow

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"syscall"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findFreePort(t *testing.T) int {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	return port
}

//nolint:revive // testing.T before context is conventional for test helpers
func waitForServer(t *testing.T, ctx context.Context, addr string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
		if err != nil {
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s did not become ready in time", addr)
}

// TestStartServer_GracefulShutdown verifies that cancelling the context
// triggers a clean server shutdown. This also exercises the tableflip
// integration on Unix systems.
//
// Note: tableflip uses global signal handlers, so only one test in this
// file can safely create a tableflip.Upgrader per process. We combine
// all assertions into a single test to avoid conflicts.
func TestSignalDone(t *testing.T) {
	done := signalDone(syscall.SIGUSR1)

	select {
	case <-done:
		t.Fatal("channel should not be closed before signal")
	default:
	}

	// Send the signal to ourselves.
	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGUSR1))

	select {
	case <-done:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("channel should have been closed after signal")
	}
}

func TestServeAndWait_WithOnReady(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	readyCh := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())

	setup := &tlsSetup{addr: ln.Addr().String()}
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &Config{Server: ServerConfig{ShutdownTimeout: 1}}
	registry := NewRegistry()
	done := make(chan struct{})

	errCh := make(chan error, 1)
	go func() {
		errCh <- serveAndWait(ctx, ln, nil, setup, handler, cfg, registry, func() error {
			close(readyCh)
			return nil
		}, done, "done")
	}()

	addr := fmt.Sprintf("http://%s/", ln.Addr().String())
	waitForServer(t, ctx, addr)

	select {
	case <-readyCh:
		// onReady was called
	case <-time.After(2 * time.Second):
		t.Fatal("onReady was not called")
	}

	cancel()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}

func TestServeAndWait_WithHTTPRedirectServer(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	httpLn, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())

	setup := &tlsSetup{
		addr:        ln.Addr().String(),
		httpAddr:    httpLn.Addr().String(),
		httpHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusMovedPermanently) }),
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &Config{Server: ServerConfig{ShutdownTimeout: 1}}
	registry := NewRegistry()
	done := make(chan struct{})

	errCh := make(chan error, 1)
	go func() {
		errCh <- serveAndWait(ctx, ln, httpLn, setup, handler, cfg, registry, nil, done, "done")
	}()

	// Wait for both servers.
	mainAddr := fmt.Sprintf("http://%s/", ln.Addr().String())
	httpAddr := fmt.Sprintf("http://%s/", httpLn.Addr().String())
	waitForServer(t, ctx, mainAddr)
	waitForServer(t, ctx, httpAddr)

	// Verify HTTP redirect server responds.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, httpAddr, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusMovedPermanently, resp.StatusCode)

	cancel()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}

func TestServeAndWait_DoneChannelTriggersShutdown(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	setup := &tlsSetup{addr: ln.Addr().String()}
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &Config{Server: ServerConfig{ShutdownTimeout: 1}}
	registry := NewRegistry()
	done := make(chan struct{})

	errCh := make(chan error, 1)
	go func() {
		errCh <- serveAndWait(t.Context(), ln, nil, setup, handler, cfg, registry, nil, done, "done triggered")
	}()

	addr := fmt.Sprintf("http://%s/", ln.Addr().String())
	waitForServer(t, t.Context(), addr)

	// Trigger shutdown via done channel.
	close(done)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}

func TestServeAndWait_OnReadyError(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	setup := &tlsSetup{addr: ln.Addr().String()}
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {})
	cfg := &Config{Server: ServerConfig{ShutdownTimeout: 1}}
	registry := NewRegistry()
	done := make(chan struct{})

	err = serveAndWait(t.Context(), ln, nil, setup, handler, cfg, registry, func() error {
		return fmt.Errorf("ready failed")
	}, done, "done")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ready failed")
}

func TestStartServer_GracefulShutdown(t *testing.T) {
	pidFile := t.TempDir() + "/test.pid"
	port := findFreePort(t)

	cfg := &Config{
		Server: ServerConfig{
			Host:            "127.0.0.1",
			Port:            port,
			PIDFile:         pidFile,
			ShutdownTimeout: 5,
		},
		TLS: TLSConfig{Mode: "off"},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	registry := NewRegistry()
	ctx, cancel := context.WithCancel(t.Context())

	errCh := make(chan error, 1)
	go func() {
		errCh <- startServer(ctx, handler, cfg, registry)
	}()

	addr := fmt.Sprintf("http://127.0.0.1:%d/", port)
	waitForServer(t, ctx, addr)

	// Verify server is serving requests.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", string(body))

	// Verify PID file was created.
	assert.FileExists(t, pidFile)

	// Cancel context to trigger graceful shutdown.
	cancel()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}
