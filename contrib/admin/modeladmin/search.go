package modeladmin

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"

	"github.com/uptrace/bun"
)

// applySearch adds search filtering to the query.
// When ftsTable is set and the term is valid FTS5 syntax, it uses FTS5 MATCH.
// Otherwise it falls back to LIKE across the given fields.
func applySearch(q *bun.SelectQuery, db *bun.DB, term string, fields []string, ftsTable string) *bun.SelectQuery {
	term = strings.TrimSpace(term)
	if term == "" || len(fields) == 0 {
		return q
	}

	if ftsTable != "" {
		return applyFTSSearch(q, db, term, fields, ftsTable)
	}

	return applyLikeSearch(q, term, fields)
}

// applyFTSSearch filters using FTS5 MATCH. On syntax errors, falls back to LIKE.
func applyFTSSearch(q *bun.SelectQuery, db *bun.DB, term string, fields []string, ftsTable string) *bun.SelectQuery {
	// Validate FTS5 syntax with a lightweight probe query.
	// This avoids modifying q with an invalid MATCH that would fail later.
	var count int
	err := db.NewRaw("SELECT COUNT(*) FROM "+ftsTable+" WHERE "+ftsTable+" MATCH ?", term).Scan(context.Background(), &count) //nolint:gosec // ftsTable is derived from struct tags at boot, not user input
	if err != nil {
		slog.Debug("FTS5 query failed, falling back to LIKE", "error", err, "term", term)
		return applyLikeSearch(q, term, fields)
	}

	q = q.Where("?TableAlias.id IN (SELECT rowid FROM "+ftsTable+" WHERE "+ftsTable+" MATCH ?)", term) //nolint:gosec // ftsTable is derived from struct tags at boot, not user input
	return q
}

// applyLikeSearch adds a LIKE search across the given fields to the query.
// The search term is escaped to prevent SQL injection via LIKE wildcards.
func applyLikeSearch(q *bun.SelectQuery, term string, fields []string) *bun.SelectQuery {
	escaped := escapeLike(term)
	pattern := "%" + escaped + "%"

	q = q.WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
		for i, field := range fields {
			if i == 0 {
				sq = sq.Where("?TableAlias.? LIKE ? ESCAPE '\\'", bun.Ident(field), pattern)
			} else {
				sq = sq.WhereOr("?TableAlias.? LIKE ? ESCAPE '\\'", bun.Ident(field), pattern)
			}
		}
		return sq
	})

	return q
}

// detectFTS checks if an FTS5 table exists for the given table name.
// Returns "{tableName}_fts" if found, or "" if not.
func detectFTS(db *bun.DB, tblName string) string {
	ftsName := tblName + "_fts"
	var name string
	err := db.NewRaw("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", ftsName).Scan(context.Background(), &name)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Debug("FTS5 detection query failed", "error", err, "table", ftsName)
		}
		return ""
	}
	return name
}

// escapeLike escapes LIKE special characters (%, _, \) in a search term.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
