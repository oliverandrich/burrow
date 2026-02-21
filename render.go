package burrow

import (
	"net/http"

	"github.com/a-h/templ"
)

// Render renders a templ.Component into the HTTP response with the given status code.
func Render(w http.ResponseWriter, r *http.Request, statusCode int, component templ.Component) error {
	buf := templ.GetBuffer()
	defer templ.ReleaseBuffer(buf)

	if err := component.Render(r.Context(), buf); err != nil {
		return err
	}

	return HTML(w, statusCode, buf.String())
}

// LayoutFunc wraps page content in a layout template.
// The layout reads framework values (nav items, user, locale, CSRF)
// from the context via helper functions.
type LayoutFunc func(title string, content templ.Component) templ.Component
