package modeladmin

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/uptrace/bun"
)

// FilterDef describes a filter available in the admin list view.
type FilterDef struct {
	Field   string   // database column name
	Label   string   // human-readable label
	Type    string   // "select", "bool", "date_range"
	Choices []Choice // for select filters
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
func applySort(q *bun.SelectQuery, r *http.Request, allowed []string) *bun.SelectQuery {
	sortParam := r.URL.Query().Get("sort")
	if sortParam == "" {
		return q
	}

	desc := false
	field := sortParam
	if strings.HasPrefix(field, "-") {
		desc = true
		field = field[1:]
	}

	if !slices.Contains(allowed, field) {
		return q
	}

	if desc {
		q = q.OrderExpr("? DESC", bun.Ident(field))
	} else {
		q = q.OrderExpr("? ASC", bun.Ident(field))
	}

	return q
}

// isValidChoice checks if a value is in the list of allowed choices.
func isValidChoice(val string, choices []Choice) bool {
	for _, c := range choices {
		if c.Value == val {
			return true
		}
	}
	return false
}

// buildActiveFilters builds ActiveFilter data for template rendering.
func buildActiveFilters(filters []FilterDef, r *http.Request) []ActiveFilter {
	if len(filters) == 0 {
		return nil
	}
	result := make([]ActiveFilter, len(filters))
	for i, f := range filters {
		activeVal := r.URL.Query().Get(f.Field)
		choices := make([]ActiveChoice, 0, len(f.Choices)+1)

		// "All" choice removes this filter.
		choices = append(choices, ActiveChoice{
			Label:    "All",
			URL:      filterURL(r, f.Field, ""),
			IsActive: activeVal == "",
		})

		for _, c := range f.Choices {
			choices = append(choices, ActiveChoice{
				Value:    c.Value,
				Label:    c.Label,
				URL:      filterURL(r, f.Field, c.Value),
				IsActive: activeVal == c.Value,
			})
		}

		result[i] = ActiveFilter{
			Field:     f.Field,
			Label:     f.Label,
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
