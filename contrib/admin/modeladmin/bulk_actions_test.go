package modeladmin

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestBulkAction_ToRenderBulkAction(t *testing.T) {
	a := BulkAction{
		Slug:    "archive",
		Label:   "Archive",
		Confirm: "Are you sure?",
	}
	ra := a.toRenderBulkAction()
	assert.Equal(t, "archive", ra.Slug)
	assert.Equal(t, "Archive", ra.Label)
	assert.Equal(t, "Are you sure?", ra.Confirm)
	assert.False(t, ra.ConfirmPage)
}

func TestBulkAction_ToRenderBulkAction_ConfirmPage(t *testing.T) {
	a := BulkAction{
		Slug:        "delete",
		Label:       "Delete",
		ConfirmPage: true,
	}
	ra := a.toRenderBulkAction()
	assert.True(t, ra.ConfirmPage)
	assert.Empty(t, ra.Confirm)
}

func TestDeleteBulkAction_Handler(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?_pragma=foreign_keys(1)")
	require.NoError(t, err)
	db := bun.NewDB(sqldb, sqlitedialect.New())
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	_, err = db.NewCreateTable().Model((*testItem)(nil)).Exec(ctx)
	require.NoError(t, err)

	// Seed 3 items.
	for i := 1; i <= 3; i++ {
		item := &testItem{Name: fmt.Sprintf("Item %d", i), Status: "active"}
		_, err = db.NewInsert().Model(item).Exec(ctx)
		require.NoError(t, err)
	}

	action := DeleteBulkAction[testItem]()
	err = action.Handler(ctx, db, []string{"1", "2"})
	require.NoError(t, err)

	// Only item 3 should remain.
	count, err := db.NewSelect().Model((*testItem)(nil)).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	var remaining testItem
	err = db.NewSelect().Model(&remaining).Scan(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Item 3", remaining.Name)
}

func TestDeleteBulkAction_ConfirmPageViaGET(t *testing.T) {
	db, renderer, ma := setupHandlerTest(t)
	// CanDelete=true → DeleteBulkAction (with ConfirmPage) auto-added by Init().
	seedItems(t, db, 3)

	r := newRouter(ma)

	// ConfirmPage actions navigate to the GET confirm page (client-side JS).
	// Verify the GET route renders the confirm page correctly.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/bulk/delete?_selected=1&_selected=2", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, renderer.confirmDeleteCalled)
	require.Len(t, renderer.lastDeleteItems, 2)
	assert.Equal(t, "1", renderer.lastDeleteItems[0].ID)
	assert.Equal(t, "2", renderer.lastDeleteItems[1].ID)
}

func TestHandleBulkAction_PreservesPage(t *testing.T) {
	_, _, ma := setupHandlerTest(t)
	ma.PageSize = 3
	ma.BulkActions = []BulkAction{bulkArchiveAction()}
	seedItems(t, db(t, ma), 30) // 10 pages of 3 items

	r := newRouter(ma)

	form := url.Values{
		"_selected": {"1", "2"},
		"_page":     {"3"},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/items/bulk/archive", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/admin/items?page=3", w.Header().Get("Location"))
}

func TestHandleBulkAction_PreservesPage_HXCurrentURLFallback(t *testing.T) {
	_, _, ma := setupHandlerTest(t)
	ma.PageSize = 3
	ma.BulkActions = []BulkAction{bulkArchiveAction()}
	seedItems(t, db(t, ma), 30) // 10 pages of 3 items

	r := newRouter(ma)

	// No _page form param, falls back to HX-Current-URL.
	form := url.Values{
		"_selected": {"1", "2"},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/items/bulk/archive", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Current-URL", "http://localhost:8080/admin/items?page=3")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/admin/items?page=3", w.Header().Get("Location"))
}

func TestHandleBulkAction_PreservesPage_RefererFallback(t *testing.T) {
	_, _, ma := setupHandlerTest(t)
	ma.PageSize = 3
	ma.BulkActions = []BulkAction{bulkArchiveAction()}
	seedItems(t, db(t, ma), 30) // 10 pages of 3 items

	r := newRouter(ma)

	// No _page, no HX-Current-URL, falls back to Referer.
	form := url.Values{
		"_selected": {"1", "2"},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/items/bulk/archive", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "http://localhost:8080/admin/items?page=3")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/admin/items?page=3", w.Header().Get("Location"))
}

func TestHandleBulkAction_ClampsToLastPage(t *testing.T) {
	_, _, ma := setupHandlerTest(t)
	ma.PageSize = 3
	ma.BulkActions = []BulkAction{bulkDeleteNowAction()}
	seedItems(t, db(t, ma), 10) // pages: 1,2,3,4 (last page has 1 item)

	r := newRouter(ma)

	// Delete the single item on page 4 (item ID 10).
	form := url.Values{
		"_selected": {"10"},
		"_page":     {"4"},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/items/bulk/delete-now", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/admin/items?page=3", w.Header().Get("Location"), "should clamp to last available page")
}

// bulkArchiveAction returns a no-op bulk action (no ConfirmPage) for testing page preservation.
func bulkArchiveAction() BulkAction {
	return BulkAction{
		Slug:  "archive",
		Label: "Archive",
		Handler: func(_ context.Context, _ *bun.DB, _ []string) error {
			return nil
		},
	}
}

// bulkDeleteNowAction returns a bulk delete action without ConfirmPage for testing page clamping.
func bulkDeleteNowAction() BulkAction {
	return BulkAction{
		Slug:  "delete-now",
		Label: "Delete Now",
		Handler: func(ctx context.Context, db *bun.DB, ids []string) error {
			_, err := db.NewDelete().Model((*testItem)(nil)).Where("id IN (?)", bun.List(ids)).Exec(ctx)
			return err
		},
	}
}

func TestHandleBulkAction_UnknownAction(t *testing.T) {
	_, _, ma := setupHandlerTest(t)
	// CanDelete=true → DeleteBulkAction auto-added by Init().

	r := newRouter(ma)

	form := url.Values{
		"_selected": {"1"},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/items/bulk/nonexistent", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleBulkAction_NoItemsSelected(t *testing.T) {
	_, _, ma := setupHandlerTest(t)
	// CanDelete=true → DeleteBulkAction auto-added by Init().

	r := newRouter(ma)

	form := url.Values{}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/items/bulk/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleBulkAction_NoBulkActions(t *testing.T) {
	_, _, ma := setupHandlerTest(t)
	ma.CanDelete = false // no auto-added DeleteBulkAction
	ma.CanEdit = false   // prevent POST /{id} from matching "bulk" as an ID

	r := newRouter(ma)

	form := url.Values{
		"_selected": {"1"},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/items/bulk/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// No /bulk/{action} route registered → chi returns 404.
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRenderConfig_BulkActions(t *testing.T) {
	_, renderer, ma := setupHandlerTest(t)
	ma.CanDelete = false // prevent auto-add so we control the exact set
	ma.BulkActions = []BulkAction{
		{Slug: "archive", Label: "Archive"},
		{Slug: "delete", Label: "Delete", ConfirmPage: true},
	}
	seedItems(t, db(t, ma), 1)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, renderer.listCalled)
	assert.True(t, renderer.lastConfig.HasBulkActions)
	require.Len(t, renderer.lastConfig.BulkActions, 2)
	assert.Equal(t, "archive", renderer.lastConfig.BulkActions[0].Slug)
	assert.Equal(t, "delete", renderer.lastConfig.BulkActions[1].Slug)
	assert.True(t, renderer.lastConfig.BulkActions[1].ConfirmPage)
}

func TestRenderConfig_NoBulkActions(t *testing.T) {
	_, renderer, ma := setupHandlerTest(t)
	ma.CanDelete = false // no auto-added DeleteBulkAction

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, renderer.listCalled)
	assert.False(t, renderer.lastConfig.HasBulkActions)
	assert.Empty(t, renderer.lastConfig.BulkActions)
}

func TestInit_AutoAddsDeleteBulkAction(t *testing.T) {
	_, _, ma := setupHandlerTest(t)
	assert.Empty(t, ma.BulkActions, "before Init, no bulk actions")

	r := newRouter(ma) // calls Init()
	_ = r

	require.Len(t, ma.BulkActions, 1)
	assert.Equal(t, "delete", ma.BulkActions[0].Slug)
}

func TestInit_NoAutoDeleteWhenCanDeleteFalse(t *testing.T) {
	_, _, ma := setupHandlerTest(t)
	ma.CanDelete = false

	r := newRouter(ma)
	_ = r

	assert.Empty(t, ma.BulkActions)
}

func TestInit_NoAutoDeleteWhenCustomDeleteExists(t *testing.T) {
	_, _, ma := setupHandlerTest(t)
	ma.BulkActions = []BulkAction{
		{Slug: "delete", Label: "Custom Delete"},
	}

	r := newRouter(ma)
	_ = r

	require.Len(t, ma.BulkActions, 1)
	assert.Equal(t, "Custom Delete", ma.BulkActions[0].Label, "should not overwrite custom delete")
}

// db is a helper that returns the DB from the ModelAdmin (used in tests that
// already have a seeded DB via setupHandlerTest but need it for assertions).
func db(t *testing.T, ma *ModelAdmin[testItem]) *bun.DB {
	t.Helper()
	return ma.DB
}
