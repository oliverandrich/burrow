package crud

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/where"
)

// Query parameters reserved by the list endpoint for pagination, ordering, and
// search. A field declared with [WithFilter] under one of these names would
// collide with that built-in meaning, so such names are dropped from the
// filter allowlist (and documented as reserved).
var reservedListParams = map[string]bool{
	"limit":    true,
	"page":     true,
	"ordering": true,
	"search":   true,
}

// WithFilter declares the JSON field names clients may filter list results by.
// Filtering is opt-in: without this option query params are ignored. A param
// matching an allowlisted field constrains the field for equality
// (?status=active); repeating the param matches any of the values
// (?status=active&status=draft -> IN). Param values are coerced to the field's
// Go type, so numeric and boolean fields filter correctly; an uncoercible
// value (e.g. ?price=abc) is a 400. Unknown params are ignored, never an error.
//
// Filters are always ANDed with [WithScope], so they can only narrow what a
// caller already sees, never widen it. Field names that collide with the
// reserved params (limit, page, ordering, search) are ignored.
func WithFilter[T any](fields ...string) Option[T] {
	return func(rs *Resource[T]) {
		rs.filterable = make(map[string]bool, len(fields))
		for _, f := range fields {
			if reservedListParams[f] {
				continue
			}
			rs.filterable[f] = true
		}
		// Built once at construction so request-time coercion is a map lookup.
		rs.fieldKinds = fieldKinds(reflect.TypeFor[T]())
	}
}

// WithOrdering declares the JSON field names clients may sort list results by.
// Clients request ordering with ?ordering=field, a leading '-' for descending,
// and commas for tie-breakers (?ordering=-priority,title). Field names are the
// JSON names; Den's built-in columns use underscore-prefixed names, so allowlist
// them via the constants (e.g. den.FieldCreatedAt). Only allowlisted fields take
// effect; when the param is absent or names no allowlisted field, the default
// sort ([WithSort]) applies.
func WithOrdering[T any](fields ...string) Option[T] {
	return func(rs *Resource[T]) {
		rs.orderable = make(map[string]bool, len(fields))
		for _, f := range fields {
			rs.orderable[f] = true
		}
	}
}

// WithSearch declares the JSON field names a ?search= term matches against, as
// a SQL LIKE substring across every listed field (ORed together). Matching is
// the database's LIKE — case-insensitive on SQLite (ASCII), case-sensitive on
// PostgreSQL; it is a plain substring scan and does not use Den's full-text
// (`den:"fts"`) index even on fields that have one. The search clause is ANDed
// with [WithScope], so it never reaches another caller's rows. Without this
// option ?search is ignored.
func WithSearch[T any](fields ...string) Option[T] {
	return func(rs *Resource[T]) { rs.searchFields = fields }
}

// fieldKinds maps each JSON field name of a struct type to its Go kind,
// recursing into embedded (anonymous, untagged) structs so promoted fields
// like Den's document.Base are included.
func fieldKinds(rt reflect.Type) map[string]reflect.Kind {
	m := make(map[string]reflect.Kind)
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return
		}
		for f := range t.Fields() {
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if f.Anonymous && name == "" {
				walk(f.Type)
				continue
			}
			if name == "" || name == "-" {
				continue
			}
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			m[name] = ft.Kind()
		}
	}
	walk(rt)
	return m
}

// listConditions builds the filter and search clauses for a list request from
// its parsed query params. Errors only on an uncoercible filter value
// (errBadRequest).
func (rs *Resource[T]) listConditions(q url.Values) ([]where.Condition, error) {
	conds, err := rs.filterConditions(q)
	if err != nil {
		return nil, err
	}
	if s := rs.searchCondition(q); s != nil {
		conds = append(conds, s)
	}
	return conds, nil
}

// filterConditions turns allowlisted query params into equality/IN conditions,
// coercing each value to its field's type.
func (rs *Resource[T]) filterConditions(q url.Values) ([]where.Condition, error) {
	if len(rs.filterable) == 0 {
		return nil, nil
	}
	var conds []where.Condition
	for field := range rs.filterable {
		raw, ok := q[field]
		if !ok || len(raw) == 0 {
			continue
		}
		values, err := rs.coerceAll(field, raw)
		if err != nil {
			return nil, err
		}
		if len(values) == 1 {
			conds = append(conds, where.Field(field).Eq(values[0]))
		} else {
			conds = append(conds, where.Field(field).In(values...))
		}
	}
	return conds, nil
}

// coerceAll coerces every raw value for a field to the field's Go type.
func (rs *Resource[T]) coerceAll(field string, raw []string) ([]any, error) {
	kind := rs.fieldKinds[field]
	values := make([]any, len(raw))
	for i, s := range raw {
		v, err := coerce(kind, s)
		if err != nil {
			return nil, fmt.Errorf("%w: filter %q: %w", errBadRequest, field, err)
		}
		values[i] = v
	}
	return values, nil
}

// searchCondition ORs a substring match for the ?search term across the
// configured search fields. Returns nil when search is unconfigured or empty.
func (rs *Resource[T]) searchCondition(q url.Values) where.Condition {
	term := q.Get("search")
	if term == "" || len(rs.searchFields) == 0 {
		return nil
	}
	clauses := make([]where.Condition, len(rs.searchFields))
	for i, f := range rs.searchFields {
		clauses[i] = where.Field(f).StringContains(term)
	}
	if len(clauses) == 1 {
		return clauses[0]
	}
	return where.Or(clauses...)
}

// sortEntries resolves the list sort: the client's ?ordering when it names
// allowlisted fields, otherwise the resource default.
func (rs *Resource[T]) sortEntries(q url.Values) []sortEntry {
	if entries := rs.requestedSort(q.Get("ordering")); len(entries) > 0 {
		return entries
	}
	return []sortEntry{{field: rs.sortField, dir: rs.sortDir}}
}

// requestedSort parses a comma-separated ?ordering value into sort entries,
// keeping only allowlisted fields ('-' prefix means descending).
func (rs *Resource[T]) requestedSort(ordering string) []sortEntry {
	if ordering == "" || len(rs.orderable) == 0 {
		return nil
	}
	var entries []sortEntry
	for tok := range strings.SplitSeq(ordering, ",") {
		tok = strings.TrimSpace(tok)
		dir := den.Asc
		if strings.HasPrefix(tok, "-") {
			dir = den.Desc
			tok = tok[1:]
		}
		if tok == "" || !rs.orderable[tok] {
			continue
		}
		entries = append(entries, sortEntry{field: tok, dir: dir})
	}
	return entries
}

// sortEntry is one resolved ordering criterion.
type sortEntry struct {
	field string
	dir   den.SortDirection
}

// coerce converts a query-param string to the Go kind of the target field.
// Unknown kinds fall back to the raw string.
func coerce(kind reflect.Kind, s string) (any, error) {
	switch kind { //nolint:exhaustive // only the scalar kinds need coercion; default keeps the raw string

	case reflect.Bool:
		return strconv.ParseBool(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.ParseInt(s, 10, 64)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.ParseUint(s, 10, 64)
	case reflect.Float32, reflect.Float64:
		return strconv.ParseFloat(s, 64)
	default:
		return s, nil
	}
}
