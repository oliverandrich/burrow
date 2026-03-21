package sse

import (
	"fmt"
	"io"
	"strings"
)

// Event represents a single SSE event.
type Event struct {
	// Type is the SSE event type (maps to htmx sse-swap="<type>").
	// Empty string means the default "message" event.
	Type string

	// Data is the event payload. Multi-line data is handled automatically
	// by splitting on newlines and emitting separate "data:" lines.
	Data string

	// ID is an optional event ID for Last-Event-ID reconnection.
	ID string

	// Retry is an optional reconnection time in milliseconds.
	// Zero means the field is omitted.
	Retry int
}

// Format writes the event in SSE wire format to w.
// The output follows the Server-Sent Events specification:
// optional id, event, and retry fields followed by one or more data lines,
// terminated by a blank line.
func (e Event) Format(w io.Writer) error {
	if e.ID != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", e.ID); err != nil {
			return err
		}
	}
	if e.Type != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", e.Type); err != nil {
			return err
		}
	}
	if e.Retry > 0 {
		if _, err := fmt.Fprintf(w, "retry: %d\n", e.Retry); err != nil {
			return err
		}
	}

	for line := range strings.SplitSeq(e.Data, "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}

	_, err := fmt.Fprint(w, "\n")
	return err
}
