// Package templates provides the default HTML template renderer for modeladmin
// CRUD views. It embeds pre-built templates for list, detail, form, and delete
// confirmation pages.
package templates

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"sync"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/admin/modeladmin"
	"github.com/oliverandrich/burrow/contrib/bsicons"
	"github.com/oliverandrich/burrow/contrib/csrf"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/burrow/forms"
	"github.com/oliverandrich/burrow/i18n"
)

//go:embed *.html
var templateFS embed.FS

var (
	tmplOnce sync.Once
	tmpl     *template.Template
)

func getTemplates() *template.Template {
	tmplOnce.Do(func() {
		tmpl = template.Must(
			template.New("").Funcs(funcMap()).ParseFS(templateFS, "*.html"),
		)
	})
	return tmpl
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"isTruthy":        isTruthy,
		"formatDateValue": formatDateValue,
		"columnValue":     modeladmin.ColumnValue,
		"fieldValue":      modeladmin.FieldValue,
		"add":             func(a, b int) int { return a + b },
		"sub":             func(a, b int) int { return a - b },
		"pageRange":       pageRange,
		"dict":            dict,
		"printf":          fmt.Sprintf,
		"iconPlus":        func() template.HTML { return bsicons.PlusLg() },
		"T":               func(key string) string { return key }, // stub, overridden per-request
		"alertClass": func(level messages.Level) string {
			if level == messages.Error {
				return "danger"
			}
			return string(level)
		},
	}
}

// pageRange returns a slice [1..n] for iteration in pagination templates.
func pageRange(n int) []int {
	r := make([]int, n)
	for i := range n {
		r[i] = i + 1
	}
	return r
}

// dict creates a map from alternating key-value pairs for sub-template data.
func dict(pairs ...any) map[string]any {
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		if key, ok := pairs[i].(string); ok {
			m[key] = pairs[i+1]
		}
	}
	return m
}

func executeTemplate(name string, t func(string) string, data map[string]any) (template.HTML, error) {
	tmpl, err := getTemplates().Clone()
	if err != nil {
		return "", fmt.Errorf("clone templates: %w", err)
	}
	tmpl = tmpl.Funcs(template.FuncMap{
		"T":           t,
		"columnValue": modeladmin.ColumnValueFunc(t),
	})
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return template.HTML(buf.String()), nil //nolint:gosec // template output is trusted
}

// DefaultRenderer returns a Renderer that uses built-in Bootstrap 5 HTML
// templates for all ModelAdmin views.
func DefaultRenderer[T any]() modeladmin.Renderer[T] {
	return &defaultRenderer[T]{}
}

type defaultRenderer[T any] struct{}

func (d *defaultRenderer[T]) List(w http.ResponseWriter, r *http.Request, items []T, page burrow.PageResult, cfg modeladmin.RenderConfig) error {
	anyItems := make([]any, len(items))
	for i, item := range items {
		anyItems[i] = item
	}
	ctx := r.Context()
	t := func(key string) string { return i18n.T(ctx, key) }
	data := map[string]any{
		"Items":     anyItems,
		"Page":      page,
		"Cfg":       cfg,
		"CSRFToken": csrf.Token(ctx),
		"Messages":  messages.Get(ctx),
	}
	content, err := executeTemplate("modeladmin/list", t, data)
	if err != nil {
		return err
	}
	return renderWithLayout(w, r, cfg.DisplayPluralName, content)
}

func (d *defaultRenderer[T]) Detail(w http.ResponseWriter, r *http.Request, item *T, cfg modeladmin.RenderConfig) error {
	itemAny := any(*item)
	ctx := r.Context()
	t := func(key string) string { return i18n.T(ctx, key) }
	data := map[string]any{
		"Item":      itemAny,
		"IDValue":   fmt.Sprintf("%v", modeladmin.FieldValue(itemAny, cfg.IDField)),
		"Cfg":       cfg,
		"CSRFToken": csrf.Token(ctx),
		"Messages":  messages.Get(ctx),
	}
	content, err := executeTemplate("modeladmin/detail", t, data)
	if err != nil {
		return err
	}
	return renderWithLayout(w, r, cfg.DisplayPluralName, content)
}

func (d *defaultRenderer[T]) Form(w http.ResponseWriter, r *http.Request, item *T, fields []forms.BoundField, cfg modeladmin.RenderConfig) error {
	var itemAny any
	if item != nil {
		itemAny = any(*item)
	}

	ctx := r.Context()
	t := func(key string) string { return i18n.T(ctx, key) }
	data := map[string]any{
		"Item":      itemAny,
		"Fields":    fields,
		"Cfg":       cfg,
		"CSRFToken": csrf.Token(ctx),
		"Messages":  messages.Get(ctx),
	}
	content, err := executeTemplate("modeladmin/form", t, data)
	if err != nil {
		return err
	}
	return renderWithLayout(w, r, cfg.DisplayPluralName, content)
}

func (d *defaultRenderer[T]) ConfirmDelete(w http.ResponseWriter, r *http.Request, item *T, cfg modeladmin.RenderConfig) error {
	return d.Detail(w, r, item, cfg)
}

// renderWithLayout wraps pre-rendered content in the layout template from context.
// For HTMX requests, it returns the content fragment directly.
func renderWithLayout(w http.ResponseWriter, r *http.Request, title string, content template.HTML) error {
	if htmx.Request(r).IsHTMX() {
		return burrow.HTML(w, http.StatusOK, string(content))
	}

	layoutTmpl := burrow.Layout(r.Context())
	if layoutTmpl == "" {
		return burrow.HTML(w, http.StatusOK, string(content))
	}

	exec := burrow.TemplateExecutorFromContext(r.Context())
	if exec == nil {
		return burrow.HTML(w, http.StatusOK, string(content))
	}

	html, err := exec(r, layoutTmpl, map[string]any{
		"Title":   title,
		"Content": content,
	})
	if err != nil {
		return fmt.Errorf("burrow: execute layout template %q: %w", layoutTmpl, err)
	}
	return burrow.HTML(w, http.StatusOK, string(html))
}
