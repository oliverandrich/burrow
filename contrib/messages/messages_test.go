package messages

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ burrow.App             = (*App)(nil)
	_ burrow.HasMiddleware   = (*App)(nil)
	_ burrow.HasDependencies = (*App)(nil)
)

func TestAdd(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = session.Inject(req, map[string]any{})

	err := Add(rec, req, Success, "Note created")
	require.NoError(t, err)

	values := session.GetValues(req)
	raw, ok := values[sessionKey]
	require.True(t, ok, "session should contain messages key")

	msgs, ok := raw.([]Message)
	require.True(t, ok, "value should be []Message")
	require.Len(t, msgs, 1)
	assert.Equal(t, Success, msgs[0].Level)
	assert.Equal(t, "Note created", msgs[0].Text)
}

func TestAddMultiple(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = session.Inject(req, map[string]any{})

	require.NoError(t, Add(rec, req, Success, "First"))
	require.NoError(t, Add(rec, req, Error, "Second"))
	require.NoError(t, Add(rec, req, Info, "Third"))

	values := session.GetValues(req)
	msgs, ok := values[sessionKey].([]Message)
	require.True(t, ok)
	require.Len(t, msgs, 3)
	assert.Equal(t, Success, msgs[0].Level)
	assert.Equal(t, Error, msgs[1].Level)
	assert.Equal(t, Info, msgs[2].Level)
}

func TestMiddleware(t *testing.T) {
	stored := []Message{
		{Level: Success, Text: "Saved"},
		{Level: Warning, Text: "Check input"},
	}

	app := New()
	mw := app.Middleware()[0]

	var gotMessages []Message
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMessages = Get(r.Context())
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = session.Inject(req, map[string]any{sessionKey: stored})

	handler.ServeHTTP(rec, req)

	// Messages should be available in context during the request.
	require.Len(t, gotMessages, 2)
	assert.Equal(t, Success, gotMessages[0].Level)
	assert.Equal(t, "Saved", gotMessages[0].Text)
	assert.Equal(t, Warning, gotMessages[1].Level)
	assert.Equal(t, "Check input", gotMessages[1].Text)

	// Messages should be cleared from session after middleware runs.
	values := session.GetValues(req)
	_, exists := values[sessionKey]
	assert.False(t, exists, "messages should be cleared from session")
}

func TestMiddlewareNoMessages(t *testing.T) {
	app := New()
	mw := app.Middleware()[0]

	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		msgs := Get(r.Context())
		assert.Nil(t, msgs, "should have no messages")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = session.Inject(req, map[string]any{})

	handler.ServeHTTP(rec, req)
	assert.True(t, called, "handler should have been called")
}

func TestGet(t *testing.T) {
	msgs := []Message{
		{Level: Info, Text: "Hello"},
	}
	ctx := Inject(context.Background(), msgs)

	got := Get(ctx)
	require.Len(t, got, 1)
	assert.Equal(t, Info, got[0].Level)
	assert.Equal(t, "Hello", got[0].Text)
}

func TestGetEmpty(t *testing.T) {
	got := Get(context.Background())
	assert.Nil(t, got)
}

func TestAddAndGetSameRequest(t *testing.T) {
	app := New()
	mw := app.Middleware()[0]

	var gotMessages []Message
	var addErr error
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addErr = Add(w, r, Success, "Created")
		gotMessages = Get(r.Context())
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = session.Inject(req, map[string]any{})
	handler.ServeHTTP(rec, req)

	require.NoError(t, addErr)
	require.Len(t, gotMessages, 1)
	assert.Equal(t, Success, gotMessages[0].Level)
	assert.Equal(t, "Created", gotMessages[0].Text)
}

func TestAddWithoutSession(t *testing.T) {
	app := New()
	mw := app.Middleware()[0]

	var addErr error
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addErr = Add(w, r, Success, "Hello")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No session.Inject — no session middleware.
	handler.ServeHTTP(rec, req)

	assert.NoError(t, addErr, "Add should not fail without session middleware")
}

func TestGetClearsSession(t *testing.T) {
	app := New()
	mw := app.Middleware()[0]

	var addErr error
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addErr = Add(w, r, Success, "Flash")
		_ = Get(r.Context())
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = session.Inject(req, map[string]any{})
	handler.ServeHTTP(rec, req)

	require.NoError(t, addErr)

	// After Get() consumed messages, session key should be cleared.
	values := session.GetValues(req)
	_, exists := values[sessionKey]
	assert.False(t, exists, "messages should be cleared from session after Get()")
}

func TestGetConsumeIsIdempotent(t *testing.T) {
	app := New()
	mw := app.Middleware()[0]

	var firstGet, secondGet []Message
	var addErr error
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addErr = Add(w, r, Success, "Once")
		firstGet = Get(r.Context())
		secondGet = Get(r.Context())
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = session.Inject(req, map[string]any{})
	handler.ServeHTTP(rec, req)

	require.NoError(t, addErr)
	require.Len(t, firstGet, 1)
	assert.Equal(t, "Once", firstGet[0].Text)
	assert.Nil(t, secondGet, "second Get() should return nil")
}

func TestMiddlewareSeededPlusAdd(t *testing.T) {
	app := New()
	mw := app.Middleware()[0]

	var gotMessages []Message
	var addErr error
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addErr = Add(w, r, Error, "Something broke")
		gotMessages = Get(r.Context())
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Seed session with an existing message from a previous request.
	req = session.Inject(req, map[string]any{
		sessionKey: []Message{{Level: Success, Text: "From previous request"}},
	})
	handler.ServeHTTP(rec, req)

	require.NoError(t, addErr)
	require.Len(t, gotMessages, 2)
	assert.Equal(t, Success, gotMessages[0].Level)
	assert.Equal(t, "From previous request", gotMessages[0].Text)
	assert.Equal(t, Error, gotMessages[1].Level)
	assert.Equal(t, "Something broke", gotMessages[1].Text)
}

func TestConvenienceHelpers(t *testing.T) {
	tests := []struct {
		name  string
		fn    func(http.ResponseWriter, *http.Request, string) error
		level Level
	}{
		{"AddInfo", AddInfo, Info},
		{"AddSuccess", AddSuccess, Success},
		{"AddWarning", AddWarning, Warning},
		{"AddError", AddError, Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = session.Inject(req, map[string]any{})

			require.NoError(t, tt.fn(rec, req, "test message"))

			values := session.GetValues(req)
			msgs, ok := values[sessionKey].([]Message)
			require.True(t, ok)
			require.Len(t, msgs, 1)
			assert.Equal(t, tt.level, msgs[0].Level)
			assert.Equal(t, "test message", msgs[0].Text)
		})
	}
}
