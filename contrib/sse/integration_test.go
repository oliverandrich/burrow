package sse_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/oliverandrich/burrow/contrib/sse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_PublishAndReceive(t *testing.T) {
	broker := sse.NewEventBroker(16)
	defer broker.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /events", sse.Handler(broker, "chat", "alerts"))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	// Wait for subscription to be established
	time.Sleep(50 * time.Millisecond)

	// Publish to both topics
	broker.Publish("chat", sse.Event{Data: "hello from chat"})
	broker.Publish("alerts", sse.Event{Data: "alert!"})

	// Read both events
	scanner := bufio.NewScanner(resp.Body)
	events := readSSEEvents(t, scanner, 2)

	require.Len(t, events, 2)
	assert.Contains(t, events[0], "data: hello from chat")
	assert.Contains(t, events[1], "data: alert!")
}

func TestIntegration_MultipleClients(t *testing.T) {
	broker := sse.NewEventBroker(16)
	defer broker.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /events", sse.Handler(broker, "news"))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect two clients
	connect := func() *http.Response {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		return resp
	}

	resp1 := connect()
	defer resp1.Body.Close()
	resp2 := connect()
	defer resp2.Body.Close()

	time.Sleep(50 * time.Millisecond)

	broker.Publish("news", sse.Event{Data: "breaking"})

	// Both clients should receive the event
	scanner1 := bufio.NewScanner(resp1.Body)
	events1 := readSSEEvents(t, scanner1, 1)
	require.Len(t, events1, 1)
	assert.Contains(t, events1[0], "data: breaking")

	scanner2 := bufio.NewScanner(resp2.Body)
	events2 := readSSEEvents(t, scanner2, 1)
	require.Len(t, events2, 1)
	assert.Contains(t, events2[0], "data: breaking")
}

func TestIntegration_ShutdownDisconnectsClients(t *testing.T) {
	broker := sse.NewEventBroker(16)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /events", sse.Handler(broker, "test"))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	time.Sleep(50 * time.Millisecond)

	// Close broker (simulates app shutdown)
	broker.Close()

	// Reading should complete (EOF or empty)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		// Drain remaining data
	}
	// Should reach here without timeout
}

func TestIntegration_ContextHelpers(t *testing.T) {
	broker := sse.NewEventBroker(16)
	defer broker.Close()

	ctx := sse.WithBroker(context.Background(), broker)
	got := sse.Broker(ctx)

	require.NotNil(t, got)
	assert.Equal(t, broker, got)

	// Publish via context-retrieved broker
	c := got.Subscribe("test")
	got.Publish("test", sse.Event{Data: "via context"})

	select {
	case e := <-c.Events():
		assert.Equal(t, "via context", e.Data)
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

func TestIntegration_DynamicTopicsHandlerFunc(t *testing.T) {
	broker := sse.NewEventBroker(16)
	defer broker.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /events", sse.HandlerFunc(broker, func(r *http.Request) []string {
		return []string{"user:" + r.URL.Query().Get("uid")}
	}))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events?uid=abc", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	time.Sleep(50 * time.Millisecond)

	// Publish to the dynamic topic
	broker.Publish("user:abc", sse.Event{Data: "for user abc"})
	// Publish to a different user — should not arrive
	broker.Publish("user:xyz", sse.Event{Data: "for user xyz"})

	scanner := bufio.NewScanner(resp.Body)
	events := readSSEEvents(t, scanner, 1)
	require.Len(t, events, 1)
	assert.Contains(t, events[0], "data: for user abc")
}

// readSSEEvents reads n complete SSE events from the scanner.
// An SSE event is delimited by a blank line.
func readSSEEvents(t *testing.T, scanner *bufio.Scanner, n int) []string {
	t.Helper()
	var events []string
	var current []string

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if len(current) > 0 {
				events = append(events, strings.Join(current, "\n"))
				current = nil
				if len(events) >= n {
					return events
				}
			}
			continue
		}
		current = append(current, line)
	}
	return events
}
