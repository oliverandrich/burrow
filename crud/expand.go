package crud

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/den"
)

// WithExpandable declares the relation fields clients may inline with
// ?expand=. Each name is the JSON key of a den.Link[T] (or []den.Link[T])
// field on the document type. A GET or list request that names an allowlisted
// field hydrates that relation and renders it as a nested object instead of the
// bare id (?expand=author,tags expands both); names not in the allowlist are
// ignored, never an error.
//
// Expansion is read-only (get and list) and resolves one level deep. Hydration
// is batched, so expanding a list is not an N+1. It does not combine with
// [WithPresenter] — a presenter owns the output shape, so a hydrated link never
// surfaces through it. Without this option ?expand is ignored.
func WithExpandable[T any](fields ...string) Option[T] {
	return func(rs *Resource[T]) {
		rs.expandable = make(map[string]bool, len(fields))
		for _, f := range fields {
			rs.expandable[f] = true
		}
	}
}

// validateExpandable warns about WithExpandable entries that are not actually
// den.Link relation fields on T — a typo there would otherwise be a silent
// no-op forever. Best-effort and run once at registration: if T isn't
// registered yet (den.LinkFields errors), it simply skips. Allowlist contents
// are trusted developer config, so this is the right place to fail loudly,
// unlike a client's unknown ?expand value, which stays silently ignored.
func (rs *Resource[T]) validateExpandable() {
	if len(rs.expandable) == 0 {
		return
	}
	metas, err := den.LinkFields[T](rs.db)
	if err != nil {
		return
	}
	valid := make(map[string]bool, len(metas))
	for _, m := range metas {
		valid[m.JSONName] = true
	}
	for name := range rs.expandable {
		if !valid[name] {
			slog.Warn("crud: WithExpandable names a field that is not a den.Link relation", "field", name)
		}
	}
}

// expandFields returns the allowlisted relation fields the request asked to
// expand (?expand=a,b), or nil when expansion is off or none match.
func (rs *Resource[T]) expandFields(params url.Values) []string {
	if len(rs.expandable) == 0 {
		return nil
	}
	var fields []string
	for tok := range strings.SplitSeq(params.Get("expand"), ",") {
		if tok = strings.TrimSpace(tok); tok != "" && rs.expandable[tok] {
			fields = append(fields, tok)
		}
	}
	return fields
}

// writeJSON writes v as the JSON response. When relation fields are being
// expanded it marshals via den.Marshal, which renders hydrated den.Link values
// as nested objects (and is byte-identical to encoding/json when nothing is
// hydrated); otherwise it uses the plain [burrow.JSON] path.
func (rs *Resource[T]) writeJSON(w http.ResponseWriter, status int, v any, expand []string) error {
	if len(expand) == 0 {
		return burrow.JSON(w, status, v)
	}
	body, err := den.Marshal(v)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(body)
	return err
}

// writeList writes a paginated list response. When expanding it keeps the
// concrete []*T item type so den.Marshal can see (and expand) the link fields:
// den.Marshal decides whether to descend into a field from its static type, and
// a PageResponse[any]{Items []any} makes it skip Items entirely (the static
// element type is `any`, which "contains no link"), so nothing would expand.
// Expansion is incompatible with WithPresenter, so the raw documents are the
// right output there; otherwise items go through the presenter as usual.
func (rs *Resource[T]) writeList(w http.ResponseWriter, items []*T, page burrow.PageResult, expand []string) error {
	if len(expand) > 0 {
		return rs.writeJSON(w, http.StatusOK, burrow.PageResponse[*T]{Items: items, Pagination: page}, expand)
	}
	return burrow.JSON(w, http.StatusOK, burrow.PageResponse[any]{Items: rs.views(items), Pagination: page})
}
