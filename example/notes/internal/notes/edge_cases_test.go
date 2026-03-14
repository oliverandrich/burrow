package notes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Repository edge cases ---

func TestCreateNoteEmptyTitle(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// The DB schema has title as NOT NULL but does not enforce non-empty at the DB level.
	// An empty string satisfies NOT NULL, so insertion succeeds.
	note := &Note{Title: "", Content: "Has content", UserID: 1}
	err := repo.Create(ctx, note)
	require.NoError(t, err)
	assert.NotZero(t, note.ID)

	found, err := repo.GetByID(ctx, note.ID, 1)
	require.NoError(t, err)
	assert.Empty(t, found.Title)
	assert.Equal(t, "Has content", found.Content)
}

func TestCreateNoteEmptyContent(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Title Only", Content: "", UserID: 1}
	err := repo.Create(ctx, note)
	require.NoError(t, err)
	assert.NotZero(t, note.ID)

	found, err := repo.GetByID(ctx, note.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, "Title Only", found.Title)
	assert.Empty(t, found.Content)
}

func TestCreateNoteEmptyTitleAndContent(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "", Content: "", UserID: 1}
	err := repo.Create(ctx, note)
	require.NoError(t, err)
	assert.NotZero(t, note.ID)
}

func TestListNotesEmptyDatabase(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// No notes created for user 1 — should return empty slice, not nil or error.
	notes, err := repo.ListByUserID(ctx, 1)
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestListByUserIDPagedEmptyDatabase(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	pr := burrow.PageRequest{Limit: 10, Page: 1}
	notes, page, err := repo.ListByUserIDPaged(t.Context(), 1, pr)
	require.NoError(t, err)
	assert.Empty(t, notes)
	assert.False(t, page.HasMore)
	assert.Equal(t, 0, page.TotalCount)
	assert.Equal(t, 0, page.TotalPages)
}

func TestDeleteNonExistentNote(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Deleting a note that does not exist should not error.
	err := repo.Delete(ctx, 999999, 1)
	require.NoError(t, err)
}

func TestDeleteAlreadyDeletedNote(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Ephemeral", Content: "Gone soon", UserID: 1}
	require.NoError(t, repo.Create(ctx, note))

	// Delete once.
	err := repo.Delete(ctx, note.ID, 1)
	require.NoError(t, err)

	// Delete again — should not error.
	err = repo.Delete(ctx, note.ID, 1)
	require.NoError(t, err)
}

func TestGetByIDNonExistent(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, 999999, 1)
	require.Error(t, err)
}

func TestUpdateNonExistentNote(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Updating a note that does not exist should not error (UPDATE affects 0 rows).
	note := &Note{ID: 999999, Title: "Ghost", Content: "Does not exist", UserID: 1}
	err := repo.Update(ctx, note)
	require.NoError(t, err)

	// Verify it was not actually created.
	_, err = repo.GetByID(ctx, 999999, 1)
	require.Error(t, err)
}

// --- Handler edge cases ---

func TestCreateNoteHandlerEmptyTitleReturnsValidationError(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)

	exec := testTemplateExecutor(t)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithTemplateExecutor(r.Context(), exec)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Post("/notes", func(w http.ResponseWriter, r *http.Request) {
		err := h.Create(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	form := strings.NewReader("title=&content=Some+content")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: 42}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	// No note should have been created.
	notes, err := repo.ListByUserID(context.Background(), 42)
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestCreateNoteHandlerEmptyContentSucceeds(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)

	msgMW := messages.New().Middleware()[0]
	r := chi.NewRouter()
	r.Use(msgMW)
	r.Post("/notes", func(w http.ResponseWriter, r *http.Request) {
		err := h.Create(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Content is not required by validation, so empty content should succeed.
	form := strings.NewReader("title=No+Content+Note&content=")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: 42}))
	req = session.Inject(req, map[string]any{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Non-HTMX: should redirect.
	assert.Equal(t, http.StatusSeeOther, rec.Code)

	notes, err := repo.ListByUserID(context.Background(), 42)
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "No Content Note", notes[0].Title)
	assert.Empty(t, notes[0].Content)
}

func TestListNotesHandlerEmptyDatabase(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: 42}))
	req = injectTemplateExecutor(t, req)
	rec := httptest.NewRecorder()

	err := h.List(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestDeleteNoteHandlerNonExistentNote(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)

	exec := testTemplateExecutor(t)
	msgMW := messages.New().Middleware()[0]
	r := chi.NewRouter()
	r.Use(msgMW)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithTemplateExecutor(r.Context(), exec)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Delete("/notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		err := h.Delete(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Delete a non-existent note — should still return OK (no-op delete).
	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/notes/999999", nil)
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: 42}))
	req = session.Inject(req, map[string]any{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSearchByUserIDEmptyDatabase(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	notes, page, err := repo.SearchByUserID(t.Context(), 1, "anything", burrow.PageRequest{Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, notes)
	assert.Equal(t, 0, page.TotalCount)
}

func TestListByUserIDPagedBeyondLastPage(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create a few notes.
	for i := range 3 {
		require.NoError(t, repo.Create(ctx, &Note{
			Title:   fmt.Sprintf("Note %d", i),
			Content: "Content",
			UserID:  1,
		}))
	}

	// Request page 100 with limit 10 — beyond available data.
	pr := burrow.PageRequest{Limit: 10, Page: 100}
	notes, page, err := repo.ListByUserIDPaged(ctx, 1, pr)
	require.NoError(t, err)
	assert.Empty(t, notes)
	assert.False(t, page.HasMore)
	assert.Equal(t, 3, page.TotalCount)
}
