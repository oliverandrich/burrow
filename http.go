package burrow

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
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
		if err := fn(w, r); err != nil {
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
				http.Error(w, httpErr.Message, httpErr.Code)
			} else {
				slog.Error("unhandled error", //nolint:gosec // slog handlers escape values
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
				)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}
	}
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

// Bind parses the request body into the given struct.
// It supports JSON and form-encoded bodies.
func Bind(r *http.Request, v any) error {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		return json.NewDecoder(r.Body).Decode(v)
	}
	// Form binding.
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("parse form: %w", err)
	}
	return bindForm(r, v)
}

// bindForm maps form values to struct fields using the "form" tag.
func bindForm(r *http.Request, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("bind: expected pointer to struct")
	}
	rv = rv.Elem()
	rt := rv.Type()

	for i := range rt.NumField() {
		field := rt.Field(i)
		tag := field.Tag.Get("form")
		if tag == "" || tag == "-" {
			// Fall back to json tag.
			tag = field.Tag.Get("json")
			if tag == "" || tag == "-" {
				continue
			}
			// Strip options like ",omitempty".
			if idx := strings.IndexByte(tag, ','); idx != -1 {
				tag = tag[:idx]
			}
		}
		val := r.FormValue(tag)
		if val != "" && rv.Field(i).CanSet() && rv.Field(i).Kind() == reflect.String {
			rv.Field(i).SetString(val)
		}
	}
	return nil
}
