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

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	_ "modernc.org/sqlite"

	"github.com/oliverandrich/burrow"
)

// mockRenderer records calls for testing.
type mockRenderer struct { //nolint:govet // fieldalignment: test struct
	listCalled          bool
	detailCalled        bool
	formCalled          bool
	confirmDeleteCalled bool
	lastItems           any
	lastItem            any
	lastFields          []FormField
	lastErrors          *burrow.ValidationError
	lastConfig          RenderConfig
	lastPage            burrow.PageResult
}

func (m *mockRenderer) List(w http.ResponseWriter, r *http.Request, items []testItem, page burrow.PageResult, cfg RenderConfig) error {
	m.listCalled = true
	m.lastItems = items
	m.lastPage = page
	m.lastConfig = cfg
	w.WriteHeader(http.StatusOK)
	return nil
}

func (m *mockRenderer) Detail(w http.ResponseWriter, r *http.Request, item *testItem, cfg RenderConfig) error {
	m.detailCalled = true
	m.lastItem = item
	m.lastConfig = cfg
	w.WriteHeader(http.StatusOK)
	return nil
}

func (m *mockRenderer) Form(w http.ResponseWriter, r *http.Request, item *testItem, fields []FormField, errors *burrow.ValidationError, cfg RenderConfig) error {
	m.formCalled = true
	m.lastItem = item
	m.lastFields = fields
	m.lastErrors = errors
	m.lastConfig = cfg
	w.WriteHeader(http.StatusOK)
	return nil
}

func (m *mockRenderer) ConfirmDelete(w http.ResponseWriter, r *http.Request, item *testItem, cfg RenderConfig) error {
	m.confirmDeleteCalled = true
	m.lastItem = item
	m.lastConfig = cfg
	w.WriteHeader(http.StatusOK)
	return nil
}

func setupHandlerTest(t *testing.T) (*bun.DB, *mockRenderer, *ModelAdmin[testItem]) {
	t.Helper()
	sqldb, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db := bun.NewDB(sqldb, sqlitedialect.New())
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	_, err = db.NewCreateTable().Model((*testItem)(nil)).Exec(ctx)
	require.NoError(t, err)

	renderer := &mockRenderer{}
	ma := &ModelAdmin[testItem]{
		Slug:        "items",
		DisplayName: "Item", DisplayPluralName: "Items",
		DB:        db,
		Renderer:  renderer,
		CanCreate: true,
		CanEdit:   true,
		CanDelete: true,
		PageSize:  10,
	}

	return db, renderer, ma
}

func newRouter(ma *ModelAdmin[testItem]) chi.Router {
	r := chi.NewRouter()
	ma.Routes(r)
	return r
}

func TestHandleList(t *testing.T) {
	db, renderer, ma := setupHandlerTest(t)
	seedItems(t, db, 5)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, renderer.listCalled)
	items := renderer.lastItems.([]testItem)
	assert.Len(t, items, 5)
	assert.Equal(t, 5, renderer.lastPage.TotalCount)
}

func TestHandleList_Pagination(t *testing.T) {
	db, renderer, ma := setupHandlerTest(t)
	ma.PageSize = 3
	seedItems(t, db, 10)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items?page=2", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, renderer.listCalled)
	items := renderer.lastItems.([]testItem)
	assert.Len(t, items, 3)
	assert.Equal(t, 2, renderer.lastPage.Page)
}

func TestHandleDetail_ReadOnly(t *testing.T) {
	db, renderer, ma := setupHandlerTest(t)
	ma.CanEdit = false

	item := &testItem{Name: "Detail Test", Status: "active"}
	_, err := db.NewInsert().Model(item).Exec(context.Background())
	require.NoError(t, err)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/items/%d", item.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, renderer.detailCalled)
}

func TestHandleDetail_EditMode(t *testing.T) {
	db, renderer, ma := setupHandlerTest(t)

	item := &testItem{Name: "Edit Test", Status: "active"}
	_, err := db.NewInsert().Model(item).Exec(context.Background())
	require.NoError(t, err)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/items/%d", item.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, renderer.formCalled, "should render form when CanEdit is true")
	assert.NotNil(t, renderer.lastFields)
}

func TestHandleDetail_NotFound(t *testing.T) {
	_, _, ma := setupHandlerTest(t)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleNew(t *testing.T) {
	_, renderer, ma := setupHandlerTest(t)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/new", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, renderer.formCalled)
	assert.Nil(t, renderer.lastItem) // nil item = create mode
}

func TestHandleNew_Forbidden(t *testing.T) {
	_, _, ma := setupHandlerTest(t)
	ma.CanCreate = false

	// When CanCreate is false, the route is not registered at all.
	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/new", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// "new" is treated as an ID since the /new route is not registered.
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleCreate(t *testing.T) {
	db, _, ma := setupHandlerTest(t)

	r := newRouter(ma)
	form := url.Values{
		"name":   {"Created Item"},
		"status": {"active"},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/items", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/admin/items", w.Header().Get("Location"))

	// Verify item was created in DB.
	var items []testItem
	err := db.NewSelect().Model(&items).Scan(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Created Item", items[0].Name)
}

func TestHandleUpdate(t *testing.T) {
	db, _, ma := setupHandlerTest(t)

	item := &testItem{Name: "Original", Status: "active"}
	_, err := db.NewInsert().Model(item).Exec(context.Background())
	require.NoError(t, err)

	r := newRouter(ma)
	form := url.Values{
		"name":   {"Updated"},
		"status": {"inactive"},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/items/%d", item.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/admin/items", w.Header().Get("Location"))

	// Verify update.
	var loaded testItem
	err = db.NewSelect().Model(&loaded).Where("id = ?", item.ID).Scan(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Updated", loaded.Name)
	assert.Equal(t, "inactive", loaded.Status)
}

func TestHandleUpdate_Continue(t *testing.T) {
	db, _, ma := setupHandlerTest(t)

	item := &testItem{Name: "Original", Status: "active"}
	_, err := db.NewInsert().Model(item).Exec(context.Background())
	require.NoError(t, err)

	r := newRouter(ma)
	form := url.Values{
		"name":      {"Updated"},
		"status":    {"active"},
		"_continue": {"1"},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/items/%d", item.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, fmt.Sprintf("/admin/items/%d", item.ID), w.Header().Get("Location"))
}

func TestHandleUpdate_Forbidden(t *testing.T) {
	db, _, ma := setupHandlerTest(t)
	ma.CanEdit = false

	item := &testItem{Name: "Test", Status: "active"}
	_, err := db.NewInsert().Model(item).Exec(context.Background())
	require.NoError(t, err)

	// When CanEdit is false, the POST route is not registered.
	r := newRouter(ma)
	form := url.Values{"name": {"Updated"}}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/items/%d", item.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleDelete(t *testing.T) {
	db, _, ma := setupHandlerTest(t)

	item := &testItem{Name: "Delete Me", Status: "active"}
	_, err := db.NewInsert().Model(item).Exec(context.Background())
	require.NoError(t, err)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, fmt.Sprintf("/items/%d", item.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "/admin/items", w.Header().Get("HX-Redirect"))

	// Verify deletion.
	count, err := db.NewSelect().Model((*testItem)(nil)).Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestHandleDelete_NotFound(t *testing.T) {
	_, _, ma := setupHandlerTest(t)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/items/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleDelete_Forbidden(t *testing.T) {
	db, _, ma := setupHandlerTest(t)
	ma.CanDelete = false

	item := &testItem{Name: "Test", Status: "active"}
	_, err := db.NewInsert().Model(item).Exec(context.Background())
	require.NoError(t, err)

	// When CanDelete is false, the DELETE route is not registered.
	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, fmt.Sprintf("/items/%d", item.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestRenderConfig(t *testing.T) {
	_, renderer, ma := setupHandlerTest(t)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, renderer.listCalled)
	assert.Equal(t, "items", renderer.lastConfig.Slug)
	assert.Equal(t, "Items", renderer.lastConfig.DisplayPluralName)
	assert.True(t, renderer.lastConfig.CanCreate)
	assert.True(t, renderer.lastConfig.CanEdit)
	assert.True(t, renderer.lastConfig.CanDelete)
}
