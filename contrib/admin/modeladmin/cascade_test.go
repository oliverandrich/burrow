package modeladmin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterTableDisplayName(t *testing.T) {
	t.Run("registered name", func(t *testing.T) {
		RegisterTableDisplayName("test_widgets", "Widgets")
		t.Cleanup(func() {
			tableDisplayMu.Lock()
			delete(tableDisplayNames, "test_widgets")
			tableDisplayMu.Unlock()
		})
		tableDisplayMu.RLock()
		name := tableDisplayNames["test_widgets"]
		tableDisplayMu.RUnlock()
		assert.Equal(t, "Widgets", name)
	})

	t.Run("empty table and name are ignored", func(t *testing.T) {
		RegisterTableDisplayName("", "Empty")
		RegisterTableDisplayName("something", "")
		tableDisplayMu.RLock()
		_, found := tableDisplayNames["something"]
		tableDisplayMu.RUnlock()
		assert.False(t, found)
	})
}

func TestHandleConfirmDelete(t *testing.T) {
	db, renderer, ma := setupHandlerTest(t)

	item := &testItem{Name: "Cascade Test", Status: "active"}
	ctx := t.Context()
	err := createItem(ctx, db, item)
	require.NoError(t, err)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/items/bulk/delete?_selected="+item.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, renderer.confirmDeleteCalled)
	require.Len(t, renderer.lastDeleteItems, 1)
	assert.Equal(t, item.ID, renderer.lastDeleteItems[0].ID)
}

func TestHandleConfirmDelete_NoItemsSelected(t *testing.T) {
	_, _, ma := setupHandlerTest(t)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/bulk/delete", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleConfirmDelete_Forbidden(t *testing.T) {
	_, _, ma := setupHandlerTest(t)
	ma.CanDelete = false

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/bulk/delete?_selected=someID", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// No delete routes registered when CanDelete is false.
	assert.Equal(t, http.StatusNotFound, w.Code)
}
