package modeladmin

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/uptrace/bun"

	"github.com/oliverandrich/burrow/forms"
	"github.com/oliverandrich/burrow/i18n"
)

// FilterDef describes a filter available in the admin list view.
type FilterDef struct { //nolint:govet // fieldalignment: readability over optimization
	Field    string         // database column name
	Label    string         // human-readable label
	LabelKey string         // i18n key for the label; translated via i18n.T at request time
	Type     string         // "select", "bool", "date_range"
	Choices  []forms.Choice // for select filters
}

// applyFilters applies filter query parameters to the Bun query.
func applyFilters(q *bun.SelectQuery, r *http.Request, filters []FilterDef) *bun.SelectQuery {
	for _, f := range filters {
		val := r.URL.Query().Get(f.Field)
		if val == "" {
			continue
		}

		switch f.Type {
		case "select":
			// Validate that the value is an allowed choice.
			if isValidChoice(val, f.Choices) {
				q = q.Where("? = ?", bun.Ident(f.Field), val)
			}
		case "bool":
			switch val {
			case "true", "1":
				q = q.Where("? = ?", bun.Ident(f.Field), true)
			case "false", "0":
				q = q.Where("? = ?", bun.Ident(f.Field), false)
			}
		}
	}

	return q
}

// applySort applies column sorting from query parameters.
// Only fields in the allowed list are accepted. The query param format is
// "sort=field" for ascending or "sort=-field" for descending.
// Returns true if a sort was applied.
func applySort(q *bun.SelectQuery, r *http.Request, allowed []string) (*bun.SelectQuery, bool) {
	sortParam := r.URL.Query().Get("sort")
	if sortParam == "" {
		return q, false
	}

	desc := false
	field := sortParam
	if strings.HasPrefix(field, "-") {
		desc = true
		field = field[1:]
	}

	if !slices.Contains(allowed, field) {
		return q, false
	}

	if desc {
		q = q.OrderExpr("? DESC", bun.Ident(field))
	} else {
		q = q.OrderExpr("? ASC", bun.Ident(field))
	}

	return q, true
}

// isValidChoice checks if a value is in the list of allowed choices.
func isValidChoice(val string, choices []forms.Choice) bool {
	for _, c := range choices {
		if c.Value == val {
			return true
		}
	}
	return false
}

// buildActiveFilters builds ActiveFilter data for template rendering.
// Labels are translated via i18n.T from the request context.
func buildActiveFilters(filters []FilterDef, r *http.Request) []ActiveFilter {
	if len(filters) == 0 {
		return nil
	}
	ctx := r.Context()
	result := make([]ActiveFilter, len(filters))
	for i, f := range filters {
		activeVal := r.URL.Query().Get(f.Field)
		choices := make([]ActiveChoice, 0, len(f.Choices)+1)

		// "All" choice removes this filter.
		allLabel := i18n.T(ctx, "modeladmin-all")
		choices = append(choices, ActiveChoice{
			Label:    allLabel,
			URL:      filterURL(r, f.Field, ""),
			IsActive: activeVal == "",
		})

		for _, c := range f.Choices {
			label := c.Label
			if c.LabelKey != "" {
				label = i18n.T(ctx, c.LabelKey)
			}
			choices = append(choices, ActiveChoice{
				Value:    c.Value,
				Label:    label,
				URL:      filterURL(r, f.Field, c.Value),
				IsActive: activeVal == c.Value,
			})
		}

		filterLabel := f.Label
		if f.LabelKey != "" {
			filterLabel = i18n.T(ctx, f.LabelKey)
		}
		result[i] = ActiveFilter{
			Field:     f.Field,
			Label:     filterLabel,
			Choices:   choices,
			HasActive: activeVal != "",
		}
	}
	return result
}

// filterURL builds a URL that sets one filter param while preserving others.
// The page param is always dropped to reset pagination.
func filterURL(r *http.Request, field, value string) string {
	q := make(url.Values)
	for k, vs := range r.URL.Query() {
		if k == field || k == "page" {
			continue
		}
		q[k] = vs
	}
	if value != "" {
		q.Set(field, value)
	}
	if len(q) == 0 {
		return r.URL.Path
	}
	return r.URL.Path + "?" + q.Encode()
}
