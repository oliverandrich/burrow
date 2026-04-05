package burrow

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// HandlerFunc is an HTTP handler that returns an error.
// Use Handle() to convert it to a standard http.HandlerFunc.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// HTTPError represents an HTTP error with a status code and message.
type HTTPError struct {
	Message string
	Code    int
}

func (e *HTTPError) Error() string {
	return e.Message
}

// NewHTTPError creates a new HTTPError.
func NewHTTPError(code int, message string) *HTTPError {
	return &HTTPError{Code: code, Message: message}
}

// Handle converts a HandlerFunc into a standard http.HandlerFunc
// with centralized error handling.
func Handle(fn HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w}
		if err := fn(sw, r); err != nil {
			if sw.written {
				slog.Error("error after response started", //nolint:gosec // slog handlers escape values
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
				)
				return
			}
			var httpErr *HTTPError
			if errors.As(err, &httpErr) {
				if httpErr.Code >= 500 {
					slog.Error("server error", //nolint:gosec // slog handlers escape values
						"status", httpErr.Code,
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
					)
				}
				RenderError(w, r, httpErr.Code, httpErr.Message)
			} else {
				slog.Error("unhandled error", //nolint:gosec // slog handlers escape values
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
				)
				RenderError(w, r, http.StatusInternalServerError, "internal server error")
			}
		}
	}
}

// statusWriter wraps http.ResponseWriter to track whether a response has been started.
type statusWriter struct {
	http.ResponseWriter
	written bool
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.written = true
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	sw.written = true
	return sw.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter, allowing the standard library
// to access optional interfaces like http.Flusher and http.Hijacker.
func (sw *statusWriter) Unwrap() http.ResponseWriter {
	return sw.ResponseWriter
}

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, code int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	return json.NewEncoder(w).Encode(v)
}

// Text writes a plain text response with the given status code.
func Text(w http.ResponseWriter, code int, s string) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(code)
	_, err := w.Write([]byte(s))
	return err
}

// HTML writes an HTML response with the given status code.
func HTML(w http.ResponseWriter, code int, s string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_, err := w.Write([]byte(s))
	return err
}

const defaultMaxMemory = 32 << 20 // 32 MB

// Bind parses the request body into the given struct and validates it.
//
// Content-Type dispatch:
//   - application/json → JSON decoding
//   - multipart/form-data → multipart parsing + form decoding
//   - everything else → form-encoded parsing + form decoding
//
// Form decoding uses "form" struct tags (falling back to "json", then field name)
// and supports all basic types (string, int, bool, float, slices, etc.).
//
// After decoding, Bind calls [Validate] automatically. If validation fails it
// returns a [*ValidationError] with per-field errors. Structs without "validate"
// tags pass through unchanged.
func Bind(r *http.Request, v any) error {
	ct := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "application/json"):
		if err := json.NewDecoder(r.Body).Decode(v); err != nil {
			return fmt.Errorf("decode json: %w", err)
		}
	case strings.HasPrefix(ct, "multipart/form-data"):
		if err := r.ParseMultipartForm(defaultMaxMemory); err != nil {
			return fmt.Errorf("parse multipart form: %w", err)
		}
		if err := formDecoder.Decode(v, r.Form); err != nil {
			return fmt.Errorf("decode form: %w", err)
		}
	default: // application/x-www-form-urlencoded
		if err := r.ParseForm(); err != nil {
			return fmt.Errorf("parse form: %w", err)
		}
		if err := formDecoder.Decode(v, r.Form); err != nil {
			return fmt.Errorf("decode form: %w", err)
		}
	}
	return Validate(v)
}
