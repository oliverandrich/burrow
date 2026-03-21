package sse

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_Headers(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	srv := httptest.NewServer(Handler(b, "test"))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "keep-alive", resp.Header.Get("Connection"))
}

func TestHandler_StreamsEvents(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	srv := httptest.NewServer(Handler(b, "updates"))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Give the handler a moment to subscribe
	time.Sleep(50 * time.Millisecond)

	b.Publish("updates", Event{Data: "hello world"})

	scanner := bufio.NewScanner(resp.Body)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		// After we see data line and blank line, we have a full event
		if line == "" && len(lines) >= 2 {
			break
		}
	}
	require.NoError(t, scanner.Err())

	joined := strings.Join(lines, "\n")
	assert.Contains(t, joined, "event: updates")
	assert.Contains(t, joined, "data: hello world")
}

func TestHandler_ClientDisconnect(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	srv := httptest.NewServer(Handler(b, "test"))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	// Cancel the context to simulate disconnect
	cancel()
	resp.Body.Close()

	// Give time for cleanup
	time.Sleep(100 * time.Millisecond)

	// Publish should not panic or block
	b.Publish("test", Event{Data: "after disconnect"})
}

func TestHandlerFunc_DynamicTopics(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	topicFn := func(r *http.Request) []string {
		return []string{"room:" + r.URL.Query().Get("room")}
	}

	srv := httptest.NewServer(HandlerFunc(b, topicFn))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"?room=42", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Give handler time to subscribe
	time.Sleep(50 * time.Millisecond)

	b.Publish("room:42", Event{Data: "room message"})

	scanner := bufio.NewScanner(resp.Body)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		if line == "" && len(lines) >= 2 {
			break
		}
	}

	joined := strings.Join(lines, "\n")
	assert.Contains(t, joined, "data: room message")
}

// nonFlushWriter wraps a ResponseWriter without the Flusher interface.
type nonFlushWriter struct {
	header http.Header
	body   []byte
	code   int
}

func (w *nonFlushWriter) Header() http.Header  { return w.header }
func (w *nonFlushWriter) WriteHeader(code int) { w.code = code }
func (w *nonFlushWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}

func TestContextHandler_StreamsEvents(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	// Wrap ContextHandler with middleware that injects the broker
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(WithBroker(r.Context(), b))
		ContextHandler("updates").ServeHTTP(w, r)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	time.Sleep(50 * time.Millisecond)

	b.Publish("updates", Event{Data: "context hello"})

	scanner := bufio.NewScanner(resp.Body)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		if line == "" && len(lines) >= 2 {
			break
		}
	}
	require.NoError(t, scanner.Err())

	joined := strings.Join(lines, "\n")
	assert.Contains(t, joined, "data: context hello")
}

func TestContextHandler_NoBrokerReturns500(t *testing.T) {
	// No broker in context
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/events", nil)
	ContextHandler("test").ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_NonFlushableWriter(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	w := &nonFlushWriter{header: make(http.Header)}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/events", nil)

	handler := Handler(b, "test")
	handler.ServeHTTP(w, req)

	// Should return 500 when Flusher is not available
	assert.Equal(t, http.StatusInternalServerError, w.code)
}
