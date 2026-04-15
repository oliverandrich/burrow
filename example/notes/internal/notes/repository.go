package notes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/where"
)

// ErrNotFound is returned when a note is not found.
var ErrNotFound = den.ErrNotFound

// Repository provides data access for notes.
type Repository struct {
	db *den.DB
}

// NewRepository creates a new notes repository.
func NewRepository(db *den.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new note.
func (r *Repository) Create(ctx context.Context, note *Note) error {
	if err := den.Insert(ctx, r.db, note); err != nil {
		return fmt.Errorf("create note: %w", err)
	}
	return nil
}

// ListByUserID returns all notes for a user, most recent first.
func (r *Repository) ListByUserID(ctx context.Context, userID string) ([]Note, error) {
	ptrs, err := den.NewQuery[Note](ctx, r.db, where.Field("user_id").Eq(userID)).
		Sort("_created_at", den.Desc).
		All()
	if err != nil {
		return nil, fmt.Errorf("list notes for user %s: %w", userID, err)
	}
	notes := make([]Note, len(ptrs))
	for i, p := range ptrs {
		notes[i] = *p
	}
	return notes, nil
}

// ListByUserIDPaged returns paginated notes for a user using offset-based pagination.
// Notes are ordered by created_at descending (newest first).
func (r *Repository) ListByUserIDPaged(ctx context.Context, userID string, pr burrow.PageRequest) ([]Note, burrow.PageResult, error) {
	ptrs, count, err := den.NewQuery[Note](ctx, r.db, where.Field("user_id").Eq(userID)).
		Sort("_created_at", den.Desc).
		Limit(pr.Limit).
		Skip(pr.Offset()).
		AllWithCount()
	if err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("list notes for user %s: %w", userID, err)
	}

	notes := make([]Note, len(ptrs))
	for i, p := range ptrs {
		notes[i] = *p
	}
	return notes, burrow.OffsetResult(pr, int(count)), nil
}

// SearchByUserID performs a full-text search across notes for a user.
// Results are ordered by created_at descending (newest first) with offset-based pagination.
// Returns empty results for empty queries.
func (r *Repository) SearchByUserID(ctx context.Context, userID string, query string, pr burrow.PageRequest) ([]Note, burrow.PageResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, burrow.PageResult{}, nil
	}

	// Den Search does not support AllWithCount, so we do a two-pass approach:
	// count all matches, then fetch the page.
	allPtrs, err := den.NewQuery[Note](ctx, r.db, where.Field("user_id").Eq(userID)).
		Search(query)
	if err != nil {
		// Treat search errors (e.g. FTS5 syntax) as empty results.
		return nil, burrow.PageResult{}, nil //nolint:nilerr // intentional: treat FTS errors as empty results
	}

	count := len(allPtrs)

	// Manual pagination over search results.
	offset := pr.Offset()
	limit := pr.Limit
	if offset > count {
		offset = count
	}
	end := min(offset+limit, count)
	page := allPtrs[offset:end]

	notes := make([]Note, len(page))
	for i, p := range page {
		notes[i] = *p
	}
	return notes, burrow.OffsetResult(pr, count), nil
}

// ListAllPaged returns all notes with pagination (no user scope, for admin).
func (r *Repository) ListAllPaged(ctx context.Context, pr burrow.PageRequest) ([]Note, burrow.PageResult, error) {
	ptrs, count, err := den.NewQuery[Note](ctx, r.db).
		Sort("_created_at", den.Desc).
		Limit(pr.Limit).
		Skip(pr.Offset()).
		AllWithCount()
	if err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("list all notes: %w", err)
	}

	notes := make([]Note, len(ptrs))
	for i, p := range ptrs {
		notes[i] = *p
	}
	return notes, burrow.OffsetResult(pr, int(count)), nil
}

// DeleteByID deletes a note by ID (no user scope, for admin).
func (r *Repository) DeleteByID(ctx context.Context, noteID string) error {
	note, err := den.FindByID[Note](ctx, r.db, noteID)
	if err != nil {
		if errors.Is(err, den.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("delete note %s: %w", noteID, err)
	}
	if err := den.Delete(ctx, r.db, note); err != nil {
		return fmt.Errorf("delete note %s: %w", noteID, err)
	}
	return nil
}

// GetByID returns a single note by ID, scoped to the given user.
func (r *Repository) GetByID(ctx context.Context, noteID, userID string) (*Note, error) {
	note, err := den.NewQuery[Note](ctx, r.db,
		where.Field("_id").Eq(noteID),
		where.Field("user_id").Eq(userID),
	).First()
	if err != nil {
		return nil, fmt.Errorf("get note %s: %w", noteID, err)
	}
	return note, nil
}

// Update updates an existing note.
func (r *Repository) Update(ctx context.Context, note *Note) error {
	if err := den.Update(ctx, r.db, note); err != nil {
		return fmt.Errorf("update note %s: %w", note.ID, err)
	}
	return nil
}

// Delete deletes a note owned by the given user.
func (r *Repository) Delete(ctx context.Context, noteID, userID string) error {
	note, err := den.NewQuery[Note](ctx, r.db,
		where.Field("_id").Eq(noteID),
		where.Field("user_id").Eq(userID),
	).First()
	if err != nil {
		if errors.Is(err, den.ErrNotFound) {
			return nil // nothing to delete
		}
		return fmt.Errorf("delete note %s: %w", noteID, err)
	}
	if err := den.Delete(ctx, r.db, note); err != nil {
		return fmt.Errorf("delete note %s: %w", noteID, err)
	}
	return nil
}
