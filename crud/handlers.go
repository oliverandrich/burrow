package crud

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/i18n"
	"github.com/oliverandrich/den"
)

// errBadRequest marks a malformed request body, distinguishing it from a
// validation failure (which surfaces as a *burrow.ValidationError).
var errBadRequest = errors.New("invalid request body")

func (rs *Resource[T]) handleList(w http.ResponseWriter, r *http.Request) error {
	pr := burrow.ParsePageRequest(r)
	params := r.URL.Query()
	conds, err := rs.listConditions(params)
	if err != nil {
		return rs.fail(w, r, err)
	}
	q := den.NewQuery[T](rs.db, rs.scopeConds(r)...).Where(conds...)
	for _, s := range rs.sortEntries(params) {
		q = q.Sort(s.field, s.dir)
	}
	items, total, err := q.
		Limit(pr.Limit).
		Skip(pr.Offset()).
		AllWithCount(r.Context())
	if err != nil {
		return rs.fail(w, r, err)
	}

	views := make([]any, len(items))
	for i, doc := range items {
		views[i] = rs.view(doc)
	}
	return burrow.JSON(w, http.StatusOK, burrow.PageResponse[any]{
		Items:      views,
		Pagination: burrow.OffsetResult(pr, int(total)),
	})
}

func (rs *Resource[T]) handleGet(w http.ResponseWriter, r *http.Request) error {
	doc, err := rs.find(r)
	if err != nil {
		return rs.fail(w, r, err)
	}
	rs.setETag(w, doc)
	return burrow.JSON(w, http.StatusOK, rs.view(doc))
}

func (rs *Resource[T]) handleCreate(w http.ResponseWriter, r *http.Request) error {
	doc, err := rs.decodeCreate(r)
	if err != nil {
		return rs.fail(w, r, err)
	}
	if err := den.Save(r.Context(), rs.db, doc); err != nil {
		return rs.fail(w, r, err)
	}
	rs.setETag(w, doc)
	return burrow.JSON(w, http.StatusCreated, rs.view(doc))
}

func (rs *Resource[T]) handleUpdate(w http.ResponseWriter, r *http.Request) error {
	doc, err := rs.find(r)
	if err != nil {
		return rs.fail(w, r, err)
	}
	if err := rs.requirePrecondition(r, doc); err != nil {
		return rs.fail(w, r, err)
	}
	if err := rs.applyUpdate(r, doc); err != nil {
		return rs.fail(w, r, err)
	}
	if err := den.Save(r.Context(), rs.db, doc); err != nil {
		return rs.fail(w, r, err)
	}
	rs.setETag(w, doc)
	return burrow.JSON(w, http.StatusOK, rs.view(doc))
}

func (rs *Resource[T]) handleDelete(w http.ResponseWriter, r *http.Request) error {
	doc, err := rs.find(r)
	if err != nil {
		return rs.fail(w, r, err)
	}
	if err := rs.requirePrecondition(r, doc); err != nil {
		return rs.fail(w, r, err)
	}
	if err := den.Delete(r.Context(), rs.db, doc); err != nil {
		return rs.fail(w, r, err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// --- decoding helpers ---

// bindValidate decodes the JSON body into v and runs struct validation,
// marking a decode failure with errBadRequest so [Resource.fail] can tell a
// malformed body apart from a validation failure or a storage error.
func bindValidate(r *http.Request, v any) error {
	if err := burrow.Bind(r, v); err != nil {
		return fmt.Errorf("%w: %w", errBadRequest, err)
	}
	return burrow.Validate(v)
}

// --- error envelope ---

type errorEnvelope struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code   string            `json:"code"`
	Fields map[string]string `json:"fields,omitempty"`
}

// fail writes the canonical JSON error envelope for err. Every CRUD error is
// funnelled through here so API clients always see one envelope shape; the
// returned error is only the (rare) response-write failure, which
// burrow.Handle logs.
func (rs *Resource[T]) fail(w http.ResponseWriter, r *http.Request, err error) error {
	var ve *burrow.ValidationError
	switch {
	case errors.As(err, &ve):
		return writeError(w, http.StatusBadRequest, "validation_failed", validationFields(r.Context(), ve))
	case errors.Is(err, den.ErrNotFound):
		return writeError(w, http.StatusNotFound, "not_found", nil)
	case errors.Is(err, errBadRequest):
		return writeError(w, http.StatusBadRequest, "invalid_request", nil)
	case errors.Is(err, errPreconditionRequired):
		return writeError(w, http.StatusPreconditionRequired, "precondition_required", nil)
	case errors.Is(err, errPreconditionFailed), errors.Is(err, den.ErrRevisionConflict):
		return writeError(w, http.StatusPreconditionFailed, "precondition_failed", nil)
	default:
		slog.Error("crud: request failed", "error", err, "method", r.Method, "path", r.URL.Path) //nolint:gosec // G706: slog escapes values
		return writeError(w, http.StatusInternalServerError, "internal", nil)
	}
}

func writeError(w http.ResponseWriter, status int, code string, fields map[string]string) error {
	return burrow.JSON(w, status, errorEnvelope{Error: errorDetail{Code: code, Fields: fields}})
}

// validationFields localizes a validation error's messages and flattens them
// into a field→message map for the envelope.
func validationFields(ctx context.Context, ve *burrow.ValidationError) map[string]string {
	ve.Translate(ctx, i18n.TData)
	fields := make(map[string]string, len(ve.Errors))
	for _, fe := range ve.Errors {
		fields[fe.Field] = fe.Message
	}
	return fields
}
