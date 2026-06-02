package crud

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// errPreconditionRequired and errPreconditionFailed drive the 428/412
// responses for the If-Match precondition; they are funnelled through
// [Resource.fail] like every other crud error.
var (
	errPreconditionRequired = errors.New("crud: if-match required")
	errPreconditionFailed   = errors.New("crud: if-match precondition failed")
)

// WithOptimisticConcurrency turns on ETag / If-Match handling for the resource.
// The document type T must opt into Den's revision tracking
// (DenSettings().UseRevision = true) so each row carries a `_rev` token; crud
// surfaces that token as a strong ETag.
//
// With it enabled, GET/create/update responses carry an ETag, and update/delete
// require a matching If-Match: a missing header is 428 (Precondition Required),
// a stale one is 412 (Precondition Failed). This stops a client from silently
// overwriting changes it never saw — an edit against a stale copy is rejected,
// and a write that loses a race between load and save surfaces as a 412 too
// (via Den's own revision check). Without this option crud ignores ETags
// entirely (current behavior).
func WithOptimisticConcurrency[T any]() Option[T] {
	return func(rs *Resource[T]) { rs.concurrency = true }
}

// revisionOf reads the document's current `_rev` token, or "" when the type
// carries no revision field.
func (rs *Resource[T]) revisionOf(doc *T) string {
	return rs.stringFieldAt(doc, rs.baseRev)
}

// setETag emits the document's revision as a strong ETag, when concurrency is
// enabled and the doc carries a revision. Must run before the response body is
// written (headers precede WriteHeader).
func (rs *Resource[T]) setETag(w http.ResponseWriter, doc *T) {
	if !rs.concurrency {
		return
	}
	if rev := rs.revisionOf(doc); rev != "" {
		w.Header().Set("ETag", strconv.Quote(rev))
	}
}

// requirePrecondition enforces If-Match on an unsafe request against the loaded
// document. No-op unless concurrency is enabled. Returns errPreconditionRequired
// when the header is absent and errPreconditionFailed when it doesn't match the
// document's current revision; "*" matches any existing row.
func (rs *Resource[T]) requirePrecondition(r *http.Request, doc *T) error {
	if !rs.concurrency {
		return nil
	}
	ifMatch := strings.TrimSpace(r.Header.Get("If-Match"))
	switch {
	case ifMatch == "":
		return errPreconditionRequired
	case ifMatch == "*":
		return nil
	case !etagMatches(ifMatch, rs.revisionOf(doc)):
		return errPreconditionFailed
	default:
		return nil
	}
}

// etagMatches reports whether an If-Match header value (a comma-separated list
// of strong validators) contains the document's current revision. Weak
// validators (W/"…") are invalid for If-Match and never match.
func etagMatches(ifMatch, rev string) bool {
	if rev == "" {
		return false
	}
	for tok := range strings.SplitSeq(ifMatch, ",") {
		tok = strings.TrimSpace(tok)
		if strings.HasPrefix(tok, "W/") {
			continue
		}
		if unq, err := strconv.Unquote(tok); err == nil && unq == rev {
			return true
		}
	}
	return false
}
