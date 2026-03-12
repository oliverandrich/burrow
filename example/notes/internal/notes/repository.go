package notes

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/oliverandrich/burrow"
	"github.com/uptrace/bun"
)

// Repository provides data access for notes.
type Repository struct {
	db *bun.DB
}

// NewRepository creates a new notes repository.
func NewRepository(db *bun.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new note.
func (r *Repository) Create(ctx context.Context, note *Note) error {
	if _, err := r.db.NewInsert().Model(note).Exec(ctx); err != nil {
		return fmt.Errorf("create note: %w", err)
	}
	return nil
}

// ListByUserID returns all notes for a user, most recent first.
func (r *Repository) ListByUserID(ctx context.Context, userID int64) ([]Note, error) {
	var notes []Note
	if err := r.db.NewSelect().Model(&notes).
		Where("user_id = ?", userID).
		Order("created_at DESC", "id DESC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list notes for user %d: %w", userID, err)
	}
	return notes, nil
}

// ListByUserIDPaged returns paginated notes for a user using cursor-based pagination.
// Notes are ordered by ID descending (newest first).
func (r *Repository) ListByUserIDPaged(ctx context.Context, userID int64, pr burrow.PageRequest) ([]Note, burrow.PageResult, error) {
	var notes []Note
	q := r.db.NewSelect().Model(&notes).Where("user_id = ?", userID)
	q = burrow.ApplyCursor(q, pr, "id")
	if err := q.Scan(ctx); err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("list notes for user %d: %w", userID, err)
	}

	notes, hasMore := burrow.TrimCursorResults(notes, pr.Limit)
	var lastID string
	if len(notes) > 0 {
		lastID = strconv.FormatInt(notes[len(notes)-1].ID, 10)
	}

	return notes, burrow.CursorResult(lastID, hasMore), nil
}

// SearchByUserID performs a full-text search across notes for a user using FTS5.
// Results are ordered by ID descending (newest first) with cursor-based pagination.
// Returns empty results for empty queries or FTS5 syntax errors.
func (r *Repository) SearchByUserID(ctx context.Context, userID int64, query string, pr burrow.PageRequest) ([]Note, burrow.PageResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, burrow.PageResult{}, nil
	}

	// Validate FTS5 syntax with a probe query.
	var count int
	if err := r.db.NewRaw("SELECT COUNT(*) FROM notes_fts WHERE notes_fts MATCH ?", query).Scan(ctx, &count); err != nil {
		// FTS5 syntax error — return empty results.
		return nil, burrow.PageResult{}, nil //nolint:nilerr // intentional: treat FTS5 syntax errors as empty results
	}

	var notes []Note
	q := r.db.NewSelect().Model(&notes).
		Where("user_id = ?", userID).
		Where("id IN (SELECT rowid FROM notes_fts WHERE notes_fts MATCH ?)", query)
	q = burrow.ApplyCursor(q, pr, "id")
	if err := q.Scan(ctx); err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("search notes for user %d: %w", userID, err)
	}

	notes, hasMore := burrow.TrimCursorResults(notes, pr.Limit)
	var lastID string
	if len(notes) > 0 {
		lastID = strconv.FormatInt(notes[len(notes)-1].ID, 10)
	}

	return notes, burrow.CursorResult(lastID, hasMore), nil
}

// GetByID returns a single note by ID, scoped to the given user.
func (r *Repository) GetByID(ctx context.Context, noteID, userID int64) (*Note, error) {
	note := new(Note)
	if err := r.db.NewSelect().Model(note).
		Where("id = ? AND user_id = ?", noteID, userID).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("get note %d: %w", noteID, err)
	}
	return note, nil
}

// Update updates an existing note.
func (r *Repository) Update(ctx context.Context, note *Note) error {
	if _, err := r.db.NewUpdate().Model(note).WherePK().Exec(ctx); err != nil {
		return fmt.Errorf("update note %d: %w", note.ID, err)
	}
	return nil
}

// Delete soft-deletes a note owned by the given user.
func (r *Repository) Delete(ctx context.Context, noteID, userID int64) error {
	if _, err := r.db.NewDelete().Model((*Note)(nil)).
		Where("id = ? AND user_id = ?", noteID, userID).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete note %d: %w", noteID, err)
	}
	return nil
}
