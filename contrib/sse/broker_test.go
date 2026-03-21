package sse

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBroker_SubscribeAndPublish(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	c := b.Subscribe("news")

	b.Publish("news", Event{Data: "hello"})

	select {
	case e := <-c.Events():
		assert.Equal(t, "news", e.Type)
		assert.Equal(t, "hello", e.Data)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestBroker_PublishSetsTypeFromTopic(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	c := b.Subscribe("alerts")
	b.Publish("alerts", Event{Data: "fire"})

	e := <-c.Events()
	assert.Equal(t, "alerts", e.Type)
}

func TestBroker_PublishPreservesExplicitType(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	c := b.Subscribe("alerts")
	b.Publish("alerts", Event{Type: "custom", Data: "fire"})

	e := <-c.Events()
	assert.Equal(t, "custom", e.Type)
}

func TestBroker_MultipleTopics(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	c := b.Subscribe("news", "alerts")

	b.Publish("news", Event{Data: "n1"})
	b.Publish("alerts", Event{Data: "a1"})

	events := make([]Event, 0, 2)
	for range 2 {
		select {
		case e := <-c.Events():
			events = append(events, e)
		case <-time.After(time.Second):
			t.Fatal("timed out")
		}
	}
	assert.Len(t, events, 2)
}

func TestBroker_UnsubscribeRemovesClient(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	c := b.Subscribe("news")
	b.Unsubscribe(c)

	b.Publish("news", Event{Data: "should not arrive"})

	select {
	case _, ok := <-c.Events():
		if ok {
			t.Fatal("received event after unsubscribe")
		}
	case <-time.After(100 * time.Millisecond):
		// Expected: channel closed, no event
	}
}

func TestBroker_UnsubscribeClosesDone(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	c := b.Subscribe("news")
	b.Unsubscribe(c)

	select {
	case <-c.Done():
		// expected
	case <-time.After(time.Second):
		t.Fatal("done channel not closed after unsubscribe")
	}
}

func TestBroker_NonBlockingPublish(t *testing.T) {
	b := NewEventBroker(1) // buffer of 1
	defer b.Close()

	c := b.Subscribe("news")

	// Fill the buffer
	b.Publish("news", Event{Data: "first"})
	// This should not block
	b.Publish("news", Event{Data: "dropped"})

	e := <-c.Events()
	assert.Equal(t, "first", e.Data)

	// No more events should be available
	select {
	case <-c.Events():
		t.Fatal("expected no more events")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestBroker_Close(t *testing.T) {
	b := NewEventBroker(16)

	c1 := b.Subscribe("news")
	c2 := b.Subscribe("alerts")

	b.Close()

	// All done channels should be closed
	select {
	case <-c1.Done():
	case <-time.After(time.Second):
		t.Fatal("c1 done not closed")
	}
	select {
	case <-c2.Done():
	case <-time.After(time.Second):
		t.Fatal("c2 done not closed")
	}
}

func TestBroker_PublishAfterClose(t *testing.T) {
	b := NewEventBroker(16)
	b.Close()

	// Should not panic
	b.Publish("news", Event{Data: "ignored"})
}

func TestBroker_SubscribeAfterClose(t *testing.T) {
	b := NewEventBroker(16)
	b.Close()

	c := b.Subscribe("news")
	require.NotNil(t, c)

	// Done should be closed immediately
	select {
	case <-c.Done():
	case <-time.After(time.Second):
		t.Fatal("done not closed on subscribe after close")
	}
}

func TestBroker_ConcurrentAccess(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			c := b.Subscribe("topic")
			for range 100 {
				b.Publish("topic", Event{Data: "concurrent"})
			}
			b.Unsubscribe(c)
		})
	}
	wg.Wait()
}

func TestBroker_NoSubscribers(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	// Should not panic when no subscribers
	b.Publish("empty-topic", Event{Data: "no one listening"})
}

func TestBroker_DoubleUnsubscribe(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	c := b.Subscribe("news")
	b.Unsubscribe(c)
	// Should not panic
	b.Unsubscribe(c)
}

func TestBroker_DoubleClose(t *testing.T) {
	b := NewEventBroker(16)
	b.Close()
	// Should not panic
	b.Close()
}
