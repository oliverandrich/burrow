package crud

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/den"
)

// WithFullTextSearch makes ?search= run a full-text search over the document
// type's Den FTS columns (the fields tagged `den:"fts"`), instead of the
// substring LIKE that [WithSearch] performs. Results are ranked by relevance.
//
// Use it for real search over text-heavy fields; it requires the type to
// declare `den:"fts"` fields (Den builds the index at registration on both
// SQLite and PostgreSQL). It is mutually exclusive with [WithSearch] — both
// bind ?search=, and if both are set full text wins. Filtering ([WithFilter])
// still narrows the results and the scope
// always applies; ?ordering does not (results are relevance-ranked), and
// because Den's FTS terminal returns no count, search responses omit
// total_count (has_more still indicates further pages).
func WithFullTextSearch[T any]() Option[T] {
	return func(rs *Resource[T]) { rs.fts = true }
}

// ftsActive reports whether this request should take the full-text path: the
// option is on and the client sent a non-blank ?search term.
func (rs *Resource[T]) ftsActive(params url.Values) bool {
	return rs.fts && strings.TrimSpace(params.Get("search")) != ""
}

// listFTS serves a relevance-ranked full-text search page. Filtering and scope
// still apply; pagination is offset-style (page/limit) but carries no total
// count, since Den's FTS terminal does not return one — has_more comes from
// over-fetching one row.
func (rs *Resource[T]) listFTS(w http.ResponseWriter, r *http.Request, params url.Values) error {
	conds, err := rs.filterConditions(params)
	if err != nil {
		return rs.fail(w, r, err)
	}
	pr := burrow.ParsePageRequest(r)
	items, err := den.NewQuery[T](rs.db, rs.scopeConds(r)...).
		Where(conds...).
		Limit(pr.Limit+1). // one extra row tells us whether another page exists
		Skip(pr.Offset()).
		Search(r.Context(), ftsQuery(params.Get("search")))
	if err != nil {
		return rs.fail(w, r, err)
	}

	hasMore := len(items) > pr.Limit
	if hasMore {
		items = items[:pr.Limit]
	}
	return burrow.JSON(w, http.StatusOK, burrow.PageResponse[any]{
		Items:      rs.views(items),
		Pagination: burrow.PageResult{Page: max(pr.Page, 1), HasMore: hasMore},
	})
}

// ftsQuery turns a raw user search term into a safe FTS5 MATCH expression:
// each whitespace-separated token becomes a quoted literal (embedded quotes
// doubled), joined by spaces so FTS5 ANDs them. This neutralises FTS5 operators
// and punctuation (AND/OR/NEAR, column filters, prefix *, stray quotes) that
// would otherwise be a syntax error or let a client scope columns. An all-blank
// term yields "" — callers route those to the normal list path instead.
//
// This bridges a Den gap: Den's Search passes the term straight to SQLite's
// FTS5 MATCH (PostgreSQL's plainto_tsquery already does this neutralising), so
// crud has to make the SQLite path safe. Tracked upstream as den-qf0k; once Den
// offers backend-agnostic literal-term search this can go away.
func ftsQuery(term string) string {
	var b strings.Builder
	for tok := range strings.FieldsSeq(term) {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteByte('"')
		b.WriteString(strings.ReplaceAll(tok, `"`, `""`))
		b.WriteByte('"')
	}
	return b.String()
}
