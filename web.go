package burrow

import (
	"context"
	"html/template"
	"net/http"

	"github.com/oliverandrich/burrow/web"
)

// NewHTTPError constructs a typed HTTP error. Wrapper around
// [web.NewHTTPError].
func NewHTTPError(code int, message string) *HTTPError { return web.NewHTTPError(code, message) }

// Handle wraps a [HandlerFunc] into an http.HandlerFunc with centralized
// error handling. Wrapper around [web.Handle].
func Handle(fn HandlerFunc) http.HandlerFunc { return web.Handle(fn) }

// JSON writes a JSON response with the given status code. Wrapper around
// [web.JSON].
func JSON(w http.ResponseWriter, code int, v any) error { return web.JSON(w, code, v) }

// Text writes a plain-text response with the given status code. Wrapper
// around [web.Text].
func Text(w http.ResponseWriter, code int, s string) error { return web.Text(w, code, s) }

// HTML writes an HTML response with the given status code. Wrapper around
// [web.HTML].
func HTML(w http.ResponseWriter, code int, s string) error { return web.HTML(w, code, s) }

// Bind parses a request body (JSON, multipart, or form-encoded) into v and
// validates it. Wrapper around [web.Bind].
func Bind(r *http.Request, v any) error { return web.Bind(r, v) }

// Validate runs validator on v and returns any [*ValidationError]. Wrapper
// around [web.Validate].
func Validate(v any) error { return web.Validate(v) }

// Render writes a rendered template into the HTTP response, wrapping it in
// the layout for full-page requests. Wrapper around [web.Render].
func Render(w http.ResponseWriter, r *http.Request, statusCode int, name string, data map[string]any) error {
	return web.Render(w, r, statusCode, name, data)
}

// RenderError writes a typed error response (JSON or HTML based on Accept).
// Wrapper around [web.RenderError].
func RenderError(w http.ResponseWriter, r *http.Request, code int, message string) {
	web.RenderError(w, r, code, message)
}

// RenderContent writes pre-rendered template.HTML into the response. Wrapper
// around [web.RenderContent].
func RenderContent(w http.ResponseWriter, r *http.Request, statusCode int, content template.HTML, data map[string]any) error {
	return web.RenderContent(w, r, statusCode, content, data)
}

// RenderFragment executes a named template into template.HTML using the
// context's executor. Wrapper around [web.RenderFragment].
func RenderFragment(ctx context.Context, name string, data map[string]any) (template.HTML, error) {
	return web.RenderFragment(ctx, name, data)
}
