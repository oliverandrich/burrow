//go:build !windows

package burrow

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrentRequests(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	setup := &tlsSetup{addr: ln.Addr().String()}
	cfg := &Config{Server: ServerConfig{ShutdownTimeout: 5}}
	registry := NewRegistry()
	done := make(chan struct{})

	errCh := make(chan error, 1)
	go func() {
		errCh <- serveAndWait(ctx, ln, nil, setup, handler, cfg, registry, nil, done, "done")
	}()

	addr := fmt.Sprintf("http://%s/", ln.Addr().String())
	waitForServer(t, ctx, addr)

	// Use a transport that closes connections after each request so the
	// server can shut down cleanly without waiting for idle keep-alive
	// connections.
	client := &http.Client{
		Transport: &http.Transport{DisableKeepAlives: true},
	}

	const numRequests = 50
	var (
		wg         sync.WaitGroup
		successCnt atomic.Int64
	)

	for range numRequests {
		wg.Go(func() {
			req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
			if reqErr != nil {
				return
			}
			resp, doErr := client.Do(req)
			if doErr != nil {
				return
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK && string(body) == "ok" {
				successCnt.Add(1)
			}
		})
	}

	wg.Wait()
	assert.Equal(t, int64(numRequests), successCnt.Load(), "all concurrent requests should succeed with HTTP 200")

	cancel()

	select {
	case srvErr := <-errCh:
		require.NoError(t, srvErr)
	case <-time.After(10 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}
