package ratelimit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRateLimitConcurrentSameClient(t *testing.T) {
	const burst = 5
	a := newTestApp(t, 1, burst) // 1 req/s, burst 5 — very slow refill so burst is the hard cap during test

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.Middleware()[0](inner)

	// Fire burst*3 concurrent requests from the same IP.
	// Exactly `burst` should be allowed; the rest should get 429.
	const totalRequests = burst * 3
	var (
		wg         sync.WaitGroup
		allowedCnt atomic.Int64
		deniedCnt  atomic.Int64
	)

	for range totalRequests {
		wg.Go(func() {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			req.RemoteAddr = "10.0.0.1:1234"
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			switch rr.Code {
			case http.StatusOK:
				allowedCnt.Add(1)
			case http.StatusTooManyRequests:
				deniedCnt.Add(1)
			}
		})
	}

	wg.Wait()

	allowed := allowedCnt.Load()
	denied := deniedCnt.Load()

	assert.Equal(t, int64(totalRequests), allowed+denied, "every request should be either 200 or 429")
	assert.Equal(t, int64(burst), allowed, "exactly burst requests should be allowed")
	assert.Equal(t, int64(totalRequests-burst), denied, "remaining requests should be denied")
}

func TestRateLimitConcurrentDifferentClients(t *testing.T) {
	const burst = 3
	a := newTestApp(t, 1, burst) // 1 req/s, burst 3

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.Middleware()[0](inner)

	// 10 different clients, each sending burst+2 requests concurrently.
	// Each client should get exactly `burst` allowed requests.
	const numClients = 10
	const requestsPerClient = burst + 2

	type result struct {
		allowed int64
		denied  int64
	}
	results := make([]result, numClients)

	var wg sync.WaitGroup

	for clientIdx := range numClients {
		clientIP := fmt.Sprintf("10.0.%d.1:1234", clientIdx)
		for range requestsPerClient {
			idx := clientIdx
			ip := clientIP
			wg.Go(func() {
				req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
				req.RemoteAddr = ip
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)

				switch rr.Code {
				case http.StatusOK:
					atomic.AddInt64(&results[idx].allowed, 1)
				case http.StatusTooManyRequests:
					atomic.AddInt64(&results[idx].denied, 1)
				}
			})
		}
	}

	wg.Wait()

	for i, r := range results {
		total := r.allowed + r.denied
		assert.Equal(t, int64(requestsPerClient), total, "client %d: all requests should be accounted for", i)
		assert.Equal(t, int64(burst), r.allowed, "client %d: exactly burst requests should be allowed", i)
		assert.Equal(t, int64(requestsPerClient-burst), r.denied, "client %d: remaining should be denied", i)
	}
}
