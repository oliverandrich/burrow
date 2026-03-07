package modeladmin

import (
	"html/template"
	"net/http"

	"codeberg.org/oliverandrich/burrow"
)

// RowAction defines a custom per-row action in the admin list/detail views.
type RowAction struct { //nolint:govet // fieldalignment: readability over optimization
	Slug     string              // URL segment: /admin/{model}/{id}/{slug}
	Label    string              // button text
	Icon     template.HTML       // icon HTML (optional)
	Method   string              // "POST" or "DELETE" (default: "POST")
	Class    string              // CSS class (default: "btn-outline-secondary")
	Confirm  string              // hx-confirm text (empty = no confirm)
	Handler  burrow.HandlerFunc  // the actual handler
	ShowWhen func(item any) bool // nil = always show
}

// method returns the HTTP method, defaulting to POST.
func (a RowAction) method() string {
	if a.Method != "" {
		return a.Method
	}
	return http.MethodPost
}

// class returns the CSS class, defaulting to "btn-outline-secondary".
func (a RowAction) class() string {
	if a.Class != "" {
		return a.Class
	}
	return "btn-outline-secondary"
}

// RenderAction holds action metadata for template rendering (no handler/ShowWhen).
type RenderAction struct {
	Slug    string
	Label   string
	Icon    template.HTML
	Method  string
	Class   string
	Confirm string
}

// toRenderAction converts a RowAction to template-safe RenderAction.
func (a RowAction) toRenderAction() RenderAction {
	return RenderAction{
		Slug:    a.Slug,
		Label:   a.Label,
		Icon:    a.Icon,
		Method:  a.method(),
		Class:   a.class(),
		Confirm: a.Confirm,
	}
}

// ItemActions holds the actions available for a specific item.
type ItemActions struct {
	Actions []RenderAction
}

// buildItemActions evaluates ShowWhen for each RowAction against the given item.
func buildItemActions(actions []RowAction, item any) ItemActions {
	result := make([]RenderAction, 0, len(actions))
	for _, a := range actions {
		if a.ShowWhen != nil && !a.ShowWhen(item) {
			continue
		}
		result = append(result, a.toRenderAction())
	}
	return ItemActions{Actions: result}
}
