package burrow

import (
	"fmt"
	"maps"
	"net/http"
	"strings"

	"github.com/oliverandrich/burrow/i18n"
)

// Render executes a named template and writes the result.
// It applies automatic layout/HTMX logic:
//   - HTMX request (HX-Request header) → fragment only, no layout
//   - Normal request + layout name in context → fragment wrapped in layout
//   - Normal request + no layout → fragment only
func Render(w http.ResponseWriter, r *http.Request, statusCode int, name string, data map[string]any) error {
	exec := TemplateExec(r.Context())
	if exec == nil {
		return fmt.Errorf("burrow: no template executor in context")
	}

	content, err := exec(r, name, data)
	if err != nil {
		return fmt.Errorf("burrow: execute template %q: %w", name, err)
	}

	// HTMX requests get the fragment only, no layout wrapping.
	// Exception: boosted requests (hx-boost) swap the full body,
	// so they need the layout applied like normal requests.
	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
		return HTML(w, statusCode, string(content))
	}

	// Normal request: wrap in layout if available.
	layoutTmpl := Layout(r.Context())
	if layoutTmpl == "" {
		return HTML(w, statusCode, string(content))
	}

	layoutData := make(map[string]any, len(data)+1)
	maps.Copy(layoutData, data)
	layoutData["Content"] = content

	html, err := exec(r, layoutTmpl, layoutData)
	if err != nil {
		return fmt.Errorf("burrow: execute layout template %q: %w", layoutTmpl, err)
	}
	return HTML(w, statusCode, string(html))
}

// RenderError writes an error response.
// For JSON API requests (Accept: application/json) it returns a JSON object.
// Otherwise it renders the "error/{code}" template through the standard
// [Render] pipeline (with layout wrapping, HTMX support, etc.).
func RenderError(w http.ResponseWriter, r *http.Request, code int, message string) {
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		_ = JSON(w, code, map[string]any{"error": message, "code": code})
		return
	}

	messageKey := fmt.Sprintf("error-%d", code)
	localizedMessage := i18n.T(r.Context(), messageKey)
	if localizedMessage == messageKey {
		localizedMessage = message
	}

	_ = Render(w, r, code, fmt.Sprintf("error/%d", code), map[string]any{
		"Code":    code,
		"Message": localizedMessage,
	})
}

// Deprecated: Use [Render] instead.
//
//go:fix inline
func RenderTemplate(w http.ResponseWriter, r *http.Request, statusCode int, name string, data map[string]any) error {
	return Render(w, r, statusCode, name, data)
}
