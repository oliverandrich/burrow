package burrow

import (
	"fmt"
	"html/template"
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

	return RenderContent(w, r, statusCode, content, data)
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

	titleKey := fmt.Sprintf("error-%d-title", code)
	localizedTitle := i18n.T(r.Context(), titleKey)
	if localizedTitle == titleKey {
		localizedTitle = http.StatusText(code)
	}

	// Error pages render without the app layout — error templates are
	// responsible for their own HTML shell (if they need one).
	r = r.WithContext(WithLayout(r.Context(), ""))

	_ = Render(w, r, code, fmt.Sprintf("error/%d", code), map[string]any{
		"Code":    code,
		"Title":   localizedTitle,
		"Message": localizedMessage,
	})
}

// RenderContent writes pre-rendered HTML content, applying the same layout
// and HTMX logic as [Render]. The data map is passed to the layout template
// with "Content" added automatically.
//
// This is useful when content was rendered by a separate template system
// (e.g., modeladmin's built-in templates) but still needs layout wrapping.
func RenderContent(w http.ResponseWriter, r *http.Request, statusCode int, content template.HTML, data map[string]any) error {
	// HTMX requests get the fragment only, no layout wrapping.
	// Exception: boosted requests (hx-boost) swap the full body,
	// so they need the layout applied like normal requests.
	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
		return HTML(w, statusCode, string(content))
	}

	layoutTmpl := Layout(r.Context())
	if layoutTmpl == "" {
		return HTML(w, statusCode, string(content))
	}

	exec := TemplateExec(r.Context())
	if exec == nil {
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

// Deprecated: Use [Render] instead.
//
//go:fix inline
func RenderTemplate(w http.ResponseWriter, r *http.Request, statusCode int, name string, data map[string]any) error {
	return Render(w, r, statusCode, name, data)
}
