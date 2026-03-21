package sse

import "sync"

// Client represents a connected SSE client subscription.
type Client struct { //nolint:govet // fieldalignment: readability over optimization
	once   sync.Once
	events chan Event
	topics map[string]struct{}
	done   chan struct{}
}

// Events returns the channel on which events are delivered.
func (c *Client) Events() <-chan Event { return c.events }

// Done returns a channel that is closed when the client is disconnected.
func (c *Client) Done() <-chan struct{} { return c.done }

// EventBroker manages SSE client connections and topic-based event distribution.
type EventBroker struct { //nolint:govet // fieldalignment: readability over optimization
	bufferSize int

	mu      sync.RWMutex
	topics  map[string]map[*Client]struct{}
	clients map[*Client]struct{}
	closed  bool
}

// NewEventBroker creates a broker. bufferSize controls per-client channel capacity.
func NewEventBroker(bufferSize int) *EventBroker {
	if bufferSize < 1 {
		bufferSize = 1
	}
	return &EventBroker{
		bufferSize: bufferSize,
		topics:     make(map[string]map[*Client]struct{}),
		clients:    make(map[*Client]struct{}),
	}
}

// Subscribe adds a client to the given topics. Returns a Client handle
// used by the SSE handler to read events and detect disconnection.
func (b *EventBroker) Subscribe(topics ...string) *Client {
	c := &Client{
		events: make(chan Event, b.bufferSize),
		topics: make(map[string]struct{}, len(topics)),
		done:   make(chan struct{}),
	}
	for _, t := range topics {
		c.topics[t] = struct{}{}
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		close(c.done)
		close(c.events)
		return c
	}

	b.clients[c] = struct{}{}
	for _, t := range topics {
		if b.topics[t] == nil {
			b.topics[t] = make(map[*Client]struct{})
		}
		b.topics[t][c] = struct{}{}
	}
	return c
}

// Unsubscribe removes a client from all topics and closes its channels.
func (b *EventBroker) Unsubscribe(c *Client) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.clients[c]; !ok {
		return
	}
	b.removeClientLocked(c)
}

// Publish sends an event to all clients subscribed to the given topic.
// The event's Type field is set to the topic if not already set.
// Non-blocking: if a client's buffer is full, the event is dropped for that client.
func (b *EventBroker) Publish(topic string, event Event) {
	if event.Type == "" {
		event.Type = topic
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return
	}

	subscribers := b.topics[topic]
	for c := range subscribers {
		select {
		case c.events <- event:
		default:
			// Drop event for slow client
		}
	}
}

// Close disconnects all clients. Safe to call multiple times.
func (b *EventBroker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}
	b.closed = true

	for c := range b.clients {
		b.removeClientLocked(c)
	}
}

// removeClientLocked removes a client from all topic maps and closes its channels.
// Must be called with b.mu held.
func (b *EventBroker) removeClientLocked(c *Client) {
	for t := range c.topics {
		if subs, ok := b.topics[t]; ok {
			delete(subs, c)
			if len(subs) == 0 {
				delete(b.topics, t)
			}
		}
	}
	delete(b.clients, c)
	c.once.Do(func() {
		close(c.done)
		close(c.events)
	})
}
