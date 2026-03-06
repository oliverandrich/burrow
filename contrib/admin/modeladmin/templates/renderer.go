package templates

import (
	"net/http"

	"github.com/a-h/templ"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/admin/modeladmin"
)

// DefaultRenderer returns a Renderer that uses the built-in Bootstrap 5 Templ
// templates for all ModelAdmin views.
func DefaultRenderer[T any]() modeladmin.Renderer[T] {
	return &defaultRenderer[T]{}
}

type defaultRenderer[T any] struct{}

func (d *defaultRenderer[T]) List(w http.ResponseWriter, r *http.Request, items []T, page burrow.PageResult, cfg modeladmin.RenderConfig) error {
	// Convert typed slice to []any for the template.
	anyItems := make([]any, len(items))
	for i, item := range items {
		anyItems[i] = item
	}
	return renderWithLayout(w, r, cfg.Display, listPage(anyItems, page, cfg))
}

func (d *defaultRenderer[T]) Detail(w http.ResponseWriter, r *http.Request, item *T, cfg modeladmin.RenderConfig) error {
	return renderWithLayout(w, r, cfg.Display, detailPage(any(*item), cfg))
}

func (d *defaultRenderer[T]) Form(w http.ResponseWriter, r *http.Request, item *T, fields []modeladmin.FormField, errors *burrow.ValidationError, cfg modeladmin.RenderConfig) error {
	var itemAny any
	if item != nil {
		itemAny = any(*item)
	}
	return renderWithLayout(w, r, cfg.Display, formPage(itemAny, fields, errors, cfg))
}

func (d *defaultRenderer[T]) ConfirmDelete(w http.ResponseWriter, r *http.Request, item *T, cfg modeladmin.RenderConfig) error {
	return renderWithLayout(w, r, cfg.Display, detailPage(any(*item), cfg))
}

// renderWithLayout wraps content in the layout from context, or renders bare content.
func renderWithLayout(w http.ResponseWriter, r *http.Request, title string, content templ.Component) error {
	lay := burrow.Layout(r.Context())
	if lay != nil {
		return burrow.Render(w, r, http.StatusOK, lay(title, content))
	}
	return burrow.Render(w, r, http.StatusOK, content)
}
