package modeladmin

import (
	"strings"

	"github.com/uptrace/bun"
)

// applySearch adds a LIKE search across the given fields to the query.
// The search term is escaped to prevent SQL injection via LIKE wildcards.
func applySearch(q *bun.SelectQuery, term string, fields []string) *bun.SelectQuery {
	term = strings.TrimSpace(term)
	if term == "" || len(fields) == 0 {
		return q
	}

	escaped := escapeLike(term)
	pattern := "%" + escaped + "%"

	q = q.WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
		for i, field := range fields {
			if i == 0 {
				sq = sq.Where("? LIKE ? ESCAPE '\\'", bun.Ident(field), pattern)
			} else {
				sq = sq.WhereOr("? LIKE ? ESCAPE '\\'", bun.Ident(field), pattern)
			}
		}
		return sq
	})

	return q
}

// escapeLike escapes LIKE special characters (%, _, \) in a search term.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
