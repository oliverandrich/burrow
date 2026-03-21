package burrow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPErrorImplementsError(t *testing.T) {
	err := NewHTTPError(http.StatusNotFound, "not found")
	assert.Equal(t, "not found", err.Error())
	assert.Equal(t, http.StatusNotFound, err.Code)
	assert.Equal(t, "not found", err.Message)
}

func TestHandleSuccess(t *testing.T) {
	handler := Handle(func(w http.ResponseWriter, _ *http.Request) error {
		return Text(w, http.StatusOK, "hello")
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "hello", rec.Body.String())
}

func TestHandleHTTPError(t *testing.T) {
	handler := Handle(func(_ http.ResponseWriter, _ *http.Request) error {
		return NewHTTPError(http.StatusForbidden, "forbidden")
	})

	req := httptest.NewRequestWithContext(TestErrorExecContext(t.Context()), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "forbidden")
}

func TestHandleGenericError(t *testing.T) {
	handler := Handle(func(_ http.ResponseWriter, _ *http.Request) error {
		return errors.New("something broke")
	})

	req := httptest.NewRequestWithContext(TestErrorExecContext(t.Context()), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal server error")
}

func TestJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	err := JSON(rec, http.StatusCreated, map[string]string{"status": "ok"})

	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestText(t *testing.T) {
	rec := httptest.NewRecorder()
	err := Text(rec, http.StatusOK, "hello world")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, "hello world", rec.Body.String())
}

func TestHTMLResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	err := HTML(rec, http.StatusOK, "<p>hello</p>")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, "<p>hello</p>", rec.Body.String())
}

func TestBindJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	body := strings.NewReader(`{"name":"alice"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")

	var p payload
	err := Bind(req, &p)

	require.NoError(t, err)
	assert.Equal(t, "alice", p.Name)
}

func TestBindForm(t *testing.T) {
	type payload struct {
		Name string `form:"name"`
	}
	body := strings.NewReader("name=alice")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var p payload
	err := Bind(req, &p)

	require.NoError(t, err)
	assert.Equal(t, "alice", p.Name)
}

func TestBindInvalidJSON(t *testing.T) {
	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")

	var p struct{ Name string }
	err := Bind(req, &p)

	require.Error(t, err)
}

func TestHandle5xxErrorIsLogged(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil))) })

	handler := Handle(func(_ http.ResponseWriter, _ *http.Request) error {
		return NewHTTPError(http.StatusInternalServerError, "db down")
	})

	req := httptest.NewRequestWithContext(TestErrorExecContext(t.Context()), http.MethodGet, "/test-path", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, buf.String(), "server error")
	assert.Contains(t, buf.String(), "db down")
	assert.Contains(t, buf.String(), "/test-path")
}

func TestHandle4xxErrorNotLogged(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil))) })

	handler := Handle(func(_ http.ResponseWriter, _ *http.Request) error {
		return NewHTTPError(http.StatusNotFound, "not found")
	})

	req := httptest.NewRequestWithContext(TestErrorExecContext(t.Context()), http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Empty(t, buf.String(), "4xx errors should not produce log output")
}

func TestBindFormWithNonStringFields(t *testing.T) {
	type payload struct {
		Name   string  `form:"name"`
		Age    int     `form:"age"`
		Active bool    `form:"active"`
		Score  float64 `form:"score"`
	}
	body := strings.NewReader("name=alice&age=30&active=true&score=9.5")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var p payload
	err := Bind(req, &p)

	require.NoError(t, err)
	assert.Equal(t, "alice", p.Name)
	assert.Equal(t, 30, p.Age)
	assert.True(t, p.Active)
	assert.InDelta(t, 9.5, p.Score, 0.001)
}

func TestBindMultipartForm(t *testing.T) {
	type payload struct {
		Name  string `form:"name"`
		Email string `form:"email"`
	}
	// Build a multipart form body.
	boundary := "testboundary"
	body := "--" + boundary + "\r\n" +
		"Content-Disposition: form-data; name=\"name\"\r\n\r\nalice\r\n" +
		"--" + boundary + "\r\n" +
		"Content-Disposition: form-data; name=\"email\"\r\n\r\nalice@example.com\r\n" +
		"--" + boundary + "--\r\n"

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)

	var p payload
	err := Bind(req, &p)

	require.NoError(t, err)
	assert.Equal(t, "alice", p.Name)
	assert.Equal(t, "alice@example.com", p.Email)
}

func TestRenderError_JSONForAPIRequest(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/missing", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	RenderError(rec, req, http.StatusNotFound, "page not found")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "page not found", body["error"])
	assert.InDelta(t, float64(http.StatusNotFound), body["code"], 0)
}

func TestRenderError_WithTemplate(t *testing.T) {
	exec := func(_ *http.Request, name string, data map[string]any) (template.HTML, error) {
		if name == "error/404" {
			return template.HTML(fmt.Sprintf("<h1>%d - %s - %s</h1>", data["Code"], data["Title"], data["Message"])), nil
		}
		return "", fmt.Errorf("template %q not found", name)
	}

	ctx := WithTemplateExecutor(t.Context(), exec)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()

	RenderError(rec, req, http.StatusNotFound, "page not found")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	// Without i18n context, Title falls back to http.StatusText.
	assert.Contains(t, rec.Body.String(), "<h1>404 - Not Found - page not found</h1>")
}

func TestRenderError_SkipsAppLayout(t *testing.T) {
	exec := func(_ *http.Request, name string, data map[string]any) (template.HTML, error) {
		switch name {
		case "error/500":
			return template.HTML(fmt.Sprintf("<p>%s</p>", data["Message"])), nil
		case "myapp/layout":
			return template.HTML(fmt.Sprintf("<html>%s</html>", data["Content"])), nil
		default:
			return "", fmt.Errorf("template %q not found", name)
		}
	}

	ctx := WithTemplateExecutor(t.Context(), exec)
	ctx = WithLayout(ctx, "myapp/layout")
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/broken", nil)
	rec := httptest.NewRecorder()

	RenderError(rec, req, http.StatusInternalServerError, "server error")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	// Error pages bypass the app layout — error templates own their HTML.
	assert.NotContains(t, rec.Body.String(), "<html>")
	assert.Contains(t, rec.Body.String(), "<p>server error</p>")
}

func TestRenderError_HTMXRequestGetsFragmentOnly(t *testing.T) {
	exec := func(_ *http.Request, name string, data map[string]any) (template.HTML, error) {
		switch name {
		case "error/404":
			return template.HTML(fmt.Sprintf("<p>%s</p>", data["Message"])), nil
		case "myapp/layout":
			return template.HTML(fmt.Sprintf("<html>%s</html>", data["Content"])), nil
		default:
			return "", fmt.Errorf("template %q not found", name)
		}
	}

	ctx := WithTemplateExecutor(t.Context(), exec)
	ctx = WithLayout(ctx, "myapp/layout")
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/missing", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	RenderError(rec, req, http.StatusNotFound, "not found")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "<p>not found</p>")
	assert.NotContains(t, rec.Body.String(), "<html>")
}

func TestHandle_UsesRenderError(t *testing.T) {
	exec := func(_ *http.Request, name string, data map[string]any) (template.HTML, error) {
		if name == "error/403" {
			return template.HTML(fmt.Sprintf("<h1>%d</h1>", data["Code"])), nil
		}
		return "", fmt.Errorf("not found")
	}

	handler := Handle(func(_ http.ResponseWriter, _ *http.Request) error {
		return NewHTTPError(http.StatusForbidden, "forbidden")
	})

	ctx := WithTemplateExecutor(t.Context(), exec)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "<h1>403</h1>")
}

func TestBindWithValidationFailure(t *testing.T) {
	type payload struct {
		Email string `form:"email" validate:"required,email"`
		Name  string `form:"name" validate:"required"`
	}
	body := strings.NewReader("email=notanemail")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var p payload
	err := Bind(req, &p)

	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.True(t, ve.HasField("name"))
	assert.True(t, ve.HasField("email"))
}

func TestBindJSONWithValidation(t *testing.T) {
	type payload struct {
		Email string `json:"email" validate:"required,email"`
	}
	body := strings.NewReader(`{"email":""}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")

	var p payload
	err := Bind(req, &p)

	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.True(t, ve.HasField("email"))
}

func TestHandleValidationErrorIsUnhandled(t *testing.T) {
	handler := Handle(func(_ http.ResponseWriter, _ *http.Request) error {
		return &ValidationError{
			Errors: []FieldError{
				{Field: "email", Tag: "required", Message: "email is required"},
			},
		}
	})

	req := httptest.NewRequestWithContext(TestErrorExecContext(t.Context()), http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal server error")
}

func TestHandleErrorAfterResponseStarted(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil))) })

	handler := Handle(func(w http.ResponseWriter, _ *http.Request) error {
		// Write a partial response, then return an error.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		return NewHTTPError(http.StatusInternalServerError, "late error")
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/partial", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// The original 200 status should be preserved (not overwritten to 500).
	assert.Equal(t, http.StatusOK, rec.Code)
	// The error should be logged since it can't be written to the client.
	assert.Contains(t, buf.String(), "error after response started")
	assert.Contains(t, buf.String(), "late error")
}

func TestHandleUnhandledErrorIsLogged(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil))) })

	handler := Handle(func(_ http.ResponseWriter, _ *http.Request) error {
		return errors.New("unexpected failure")
	})

	req := httptest.NewRequestWithContext(TestErrorExecContext(t.Context()), http.MethodPost, "/submit", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, buf.String(), "unhandled error")
	assert.Contains(t, buf.String(), "unexpected failure")
	assert.Contains(t, buf.String(), "/submit")
}

// Benchmarks

func BenchmarkHandle_Success(b *testing.B) {
	handler := Handle(func(w http.ResponseWriter, _ *http.Request) error {
		return Text(w, http.StatusOK, "hello")
	})

	ctx := context.Background()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkHandle_HTTPError(b *testing.B) {
	handler := Handle(func(_ http.ResponseWriter, _ *http.Request) error {
		return NewHTTPError(http.StatusNotFound, "not found")
	})

	ctx := TestErrorExecContext(context.Background())
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)

	// Silence slog output during benchmarks.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkJSON(b *testing.B) {
	payload := map[string]any{
		"id":     42,
		"name":   "Test Item",
		"active": true,
		"tags":   []string{"go", "web", "framework"},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rec := httptest.NewRecorder()
		_ = JSON(rec, http.StatusOK, payload)
	}
}

func BenchmarkJSON_Struct(b *testing.B) {
	type item struct { //nolint:govet // benchmark struct, readability over alignment
		ID     int64    `json:"id"`
		Name   string   `json:"name"`
		Active bool     `json:"active"`
		Tags   []string `json:"tags"`
	}
	payload := item{ID: 42, Name: "Test Item", Active: true, Tags: []string{"go", "web", "framework"}}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rec := httptest.NewRecorder()
		_ = JSON(rec, http.StatusOK, payload)
	}
}

func BenchmarkBind_JSON(b *testing.B) {
	type payload struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
	}
	body := `{"name":"alice","email":"alice@example.com","age":30}`

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		var p payload
		_ = Bind(req, &p)
	}
}

func BenchmarkBind_Form(b *testing.B) {
	type payload struct {
		Name  string `form:"name"`
		Email string `form:"email"`
		Age   int    `form:"age"`
	}
	body := "name=alice&email=alice%40example.com&age=30"

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		var p payload
		_ = Bind(req, &p)
	}
}

func BenchmarkBind_JSONWithValidation(b *testing.B) {
	type payload struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"required,min=1,max=150"`
	}
	body := `{"name":"alice","email":"alice@example.com","age":30}`

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		var p payload
		_ = Bind(req, &p)
	}
}
