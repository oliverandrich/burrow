package burrow

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"maps"
	"net/http"
	"strings"

	"github.com/oliverandrich/burrow/i18n"
)

// ErrNoTemplateExecutor is returned when Render or RenderFragment is called
// without a TemplateExecutor in the context.
var ErrNoTemplateExecutor = errors.New("burrow: no template executor in context")

// Render executes a named template and writes the result.
// It applies automatic layout/HTMX logic:
//   - HTMX request (HX-Request header) → fragment only, no layout
//   - Normal request + layout name in context → fragment wrapped in layout
//   - Normal request + no layout → fragment only
func Render(w http.ResponseWriter, r *http.Request, statusCode int, name string, data map[string]any) error {
	exec := TemplateExec(r.Context())
	if exec == nil {
		return ErrNoTemplateExecutor
	}

	content, err := exec(r.Context(), name, data)
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

	data := map[string]any{
		"Code":    code,
		"Title":   localizedTitle,
		"Message": localizedMessage,
	}

	// Try the code-specific template first, fall back to the generic default,
	// and finally fall back to a plain text response if no error template exists.
	if err := Render(w, r, code, fmt.Sprintf("error/%d", code), data); err != nil {
		if err := Render(w, r, code, "error/default", data); err != nil {
			http.Error(w, fmt.Sprintf("%d — %s", code, localizedMessage), code)
		}
	}
}

// RenderContent writes pre-rendered HTML content, applying the same layout
// and HTMX logic as [Render]. The data map is passed to the layout template
// with "Content" added automatically.
//
// This is useful when content was rendered by a separate template system
// (e.g., a custom renderer's templates) but still needs layout wrapping.
func RenderContent(w http.ResponseWriter, r *http.Request, statusCode int, content template.HTML, data map[string]any) error {
	// HTMX requests get the fragment only, no layout wrapping.
	// Boosted requests targeting "body" (default) get the full layout.
	// Boosted requests with a custom target (e.g. hx-target="main")
	// get the fragment only — the layout is already on the page.
	// Note: we read headers directly to avoid an import cycle with contrib/htmx.
	isHTMX := r.Header.Get("HX-Request") == "true"
	isBoosted := r.Header.Get("HX-Boosted") == "true"
	hxTarget := r.Header.Get("HX-Target")
	if isHTMX && (!isBoosted || (hxTarget != "" && hxTarget != "body")) {
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

	html, err := exec(r.Context(), layoutTmpl, layoutData)
	if err != nil {
		return fmt.Errorf("burrow: execute layout template %q: %w", layoutTmpl, err)
	}
	return HTML(w, statusCode, string(html))
}

// RenderFragment renders a named template outside of an HTTP request.
// It retrieves the [TemplateExecutor] from ctx, so the context must have been
// enriched with [WithTemplateExecutor] (plus any request-scoped values the
// template needs, e.g., i18n.WithLocale).
//
// Use this for background jobs, SSE broadcasts, CLI commands, or any
// non-HTTP code that needs to render templates.
func RenderFragment(ctx context.Context, name string, data map[string]any) (template.HTML, error) {
	exec := TemplateExec(ctx)
	if exec == nil {
		return "", ErrNoTemplateExecutor
	}
	return exec(ctx, name, data)
}

// Deprecated: Use [Render] instead.
//
//go:fix inline
func RenderTemplate(w http.ResponseWriter, r *http.Request, statusCode int, name string, data map[string]any) error {
	return Render(w, r, statusCode, name, data)
}
