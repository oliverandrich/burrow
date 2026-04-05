package modeladmin

import (
	"regexp"
	"strings"

	"github.com/oliverandrich/den/where"
)

// buildSearchConditions returns OR'd RegExp conditions across the given fields.
// Returns nil if the term is empty or no fields are specified.
func buildSearchConditions(term string, fields []string) []where.Condition {
	term = strings.TrimSpace(term)
	if term == "" || len(fields) == 0 {
		return nil
	}

	escaped := regexp.QuoteMeta(term)
	searchConds := make([]where.Condition, 0, len(fields))
	for _, field := range fields {
		searchConds = append(searchConds, where.Field(field).RegExp(escaped))
	}

	return []where.Condition{where.Or(searchConds...)}
}
