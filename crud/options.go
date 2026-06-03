package crud

import (
	"net/http"
	"reflect"

	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/where"
)

// WithScope narrows every action to the conditions returned for the request —
// typically an ownership or tenancy filter. It applies to list, get, update,
// and delete alike, so a client cannot reach another's row by guessing its id
// (the single-row actions 404 instead).
func WithScope[T any](fn func(*http.Request) []where.Condition) Option[T] {
	return func(rs *Resource[T]) { rs.scope = fn }
}

// WithCreate sets a typed write model for create: the request body is bound
// and validated into C, then fn maps it to a new document. Use it to keep
// clients from setting server-owned fields (mass assignment). Without it, the
// body is bound directly onto T.
func WithCreate[T, C any](fn func(C, *http.Request) (*T, error)) Option[T] {
	return func(rs *Resource[T]) {
		rs.createType = reflect.TypeFor[C]()
		rs.decodeCreate = func(r *http.Request) (*T, error) {
			var dto C
			if err := bindValidate(r, &dto); err != nil {
				return nil, err
			}
			return fn(dto, r)
		}
	}
}

// WithUpdate sets a typed write model for update: the request body is bound
// and validated into U, then fn applies it onto the loaded document. Without
// it, the body is bound directly onto the loaded T.
//
// Update is a partial merge (the route is PATCH). Standard JSON decoding gives
// fields absent from the body their Go zero value, so for true partial updates
// make U's fields pointers and apply them conditionally:
//
//	crud.WithUpdate(func(in updatePost, p *Post, _ *http.Request) error {
//	    if in.Title != nil { p.Title = *in.Title }
//	    return nil
//	})
func WithUpdate[T, U any](fn func(U, *T, *http.Request) error) Option[T] {
	return func(rs *Resource[T]) {
		rs.updateType = reflect.TypeFor[U]()
		rs.applyUpdate = func(r *http.Request, dst *T) error {
			var dto U
			if err := bindValidate(r, &dto); err != nil {
				return err
			}
			return fn(dto, dst, r)
		}
	}
}

// WithPresenter shapes the JSON written for each document — use it to expose a
// stable, explicit representation instead of the raw stored fields. Without
// it, the document is marshalled as-is.
func WithPresenter[T any](fn func(*T) any) Option[T] {
	return func(rs *Resource[T]) { rs.present = fn }
}

// WithSort sets the default ordering for list responses (creation time
// descending when unset). field is a JSON field name (e.g. "title").
func WithSort[T any](field string, dir den.SortDirection) Option[T] {
	return func(rs *Resource[T]) {
		rs.sortField = field
		rs.sortDir = dir
	}
}

// Only restricts the resource to the named actions ([ActionList], [ActionGet],
// [ActionCreate], [ActionUpdate], [ActionDelete]). Mutually exclusive with
// [Except] — the last one applied wins.
func Only[T any](actions ...string) Option[T] {
	return func(rs *Resource[T]) { rs.enabled = actionSet(actions) }
}

// Except disables the named actions, keeping the rest.
func Except[T any](actions ...string) Option[T] {
	return func(rs *Resource[T]) {
		for _, a := range actions {
			delete(rs.enabled, a)
		}
	}
}

func actionSet(actions []string) map[string]bool {
	set := make(map[string]bool, len(actions))
	for _, a := range actions {
		set[a] = true
	}
	return set
}
