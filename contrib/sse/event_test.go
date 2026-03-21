package sse

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvent_Format_DataOnly(t *testing.T) {
	var buf bytes.Buffer
	e := Event{Data: "hello"}
	err := e.Format(&buf)
	require.NoError(t, err)
	assert.Equal(t, "data: hello\n\n", buf.String())
}

func TestEvent_Format_WithType(t *testing.T) {
	var buf bytes.Buffer
	e := Event{Type: "notifications", Data: "new item"}
	err := e.Format(&buf)
	require.NoError(t, err)
	assert.Equal(t, "event: notifications\ndata: new item\n\n", buf.String())
}

func TestEvent_Format_WithID(t *testing.T) {
	var buf bytes.Buffer
	e := Event{ID: "42", Data: "hello"}
	err := e.Format(&buf)
	require.NoError(t, err)
	assert.Equal(t, "id: 42\ndata: hello\n\n", buf.String())
}

func TestEvent_Format_WithRetry(t *testing.T) {
	var buf bytes.Buffer
	e := Event{Retry: 5000, Data: "hello"}
	err := e.Format(&buf)
	require.NoError(t, err)
	assert.Equal(t, "retry: 5000\ndata: hello\n\n", buf.String())
}

func TestEvent_Format_MultilineData(t *testing.T) {
	var buf bytes.Buffer
	e := Event{Data: "line1\nline2\nline3"}
	err := e.Format(&buf)
	require.NoError(t, err)
	assert.Equal(t, "data: line1\ndata: line2\ndata: line3\n\n", buf.String())
}

func TestEvent_Format_AllFields(t *testing.T) {
	var buf bytes.Buffer
	e := Event{
		Type:  "update",
		ID:    "7",
		Retry: 3000,
		Data:  "payload",
	}
	err := e.Format(&buf)
	require.NoError(t, err)
	expected := "id: 7\nevent: update\nretry: 3000\ndata: payload\n\n"
	assert.Equal(t, expected, buf.String())
}

func TestEvent_Format_EmptyData(t *testing.T) {
	var buf bytes.Buffer
	e := Event{}
	err := e.Format(&buf)
	require.NoError(t, err)
	assert.Equal(t, "data: \n\n", buf.String())
}
