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

func TestIdFromRequest_CustomIDFunc(t *testing.T) {
	db, renderer, ma := setupHandlerTest(t)
	ma.IDFunc = func(r *http.Request) string {
		return chi.URLParam(r, "id")
	}

	item := &testItem{Name: "Custom ID", Status: "active"}
	_, err := db.NewInsert().Model(item).Exec(context.Background())
	require.NoError(t, err)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/items/%d", item.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, renderer.formCalled)
}

func TestPageSize_Default(t *testing.T) {
	ma := &ModelAdmin[testItem]{}
	assert.Equal(t, 25, ma.pageSize())
}

func TestPageSize_Custom(t *testing.T) {
	ma := &ModelAdmin[testItem]{PageSize: 50}
	assert.Equal(t, 50, ma.pageSize())
}

func TestPageSize_Zero(t *testing.T) {
	ma := &ModelAdmin[testItem]{PageSize: 0}
	assert.Equal(t, 25, ma.pageSize(), "zero should return default")
}

func TestPageSize_Negative(t *testing.T) {
	ma := &ModelAdmin[testItem]{PageSize: -1}
	assert.Equal(t, 25, ma.pageSize(), "negative should return default")
}

// validatedItem has a validate tag to trigger validation errors.
type validatedItem struct { //nolint:govet // fieldalignment: test struct
	bun.BaseModel `bun:"table:validated_items"`
	ID            int64  `bun:",pk,autoincrement"`
	Name          string `bun:",notnull" validate:"required"`
}

// mockValidatedRenderer records calls for validatedItem tests.
type mockValidatedRenderer struct { //nolint:govet // fieldalignment: test struct
	listCalled          bool
	detailCalled        bool
	formCalled          bool
	confirmDeleteCalled bool
	lastItem            any
	lastFields          []FormField
	lastErrors          *burrow.ValidationError
	lastConfig          RenderConfig
}

func (m *mockValidatedRenderer) List(w http.ResponseWriter, _ *http.Request, _ []validatedItem, _ burrow.PageResult, cfg RenderConfig) error {
	m.listCalled = true
	m.lastConfig = cfg
	w.WriteHeader(http.StatusOK)
	return nil
}

func (m *mockValidatedRenderer) Detail(w http.ResponseWriter, _ *http.Request, item *validatedItem, cfg RenderConfig) error {
	m.detailCalled = true
	m.lastItem = item
	m.lastConfig = cfg
	w.WriteHeader(http.StatusOK)
	return nil
}

func (m *mockValidatedRenderer) Form(w http.ResponseWriter, _ *http.Request, item *validatedItem, fields []FormField, errors *burrow.ValidationError, cfg RenderConfig) error {
	m.formCalled = true
	m.lastItem = item
	m.lastFields = fields
	m.lastErrors = errors
	m.lastConfig = cfg
	w.WriteHeader(http.StatusOK)
	return nil
}

func (m *mockValidatedRenderer) ConfirmDelete(w http.ResponseWriter, _ *http.Request, item *validatedItem, cfg RenderConfig) error {
	m.confirmDeleteCalled = true
	m.lastItem = item
	m.lastConfig = cfg
	w.WriteHeader(http.StatusOK)
	return nil
}

func setupValidatedHandlerTest(t *testing.T) (*bun.DB, *mockValidatedRenderer, *ModelAdmin[validatedItem]) {
	t.Helper()
	sqldb, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db := bun.NewDB(sqldb, sqlitedialect.New())
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	_, err = db.NewCreateTable().Model((*validatedItem)(nil)).Exec(ctx)
	require.NoError(t, err)

	renderer := &mockValidatedRenderer{}
	ma := &ModelAdmin[validatedItem]{
		Slug:        "validated",
		DisplayName: "Validated", DisplayPluralName: "Validated Items",
		DB:        db,
		Renderer:  renderer,
		CanCreate: true,
		CanEdit:   true,
		CanDelete: true,
		PageSize:  10,
	}

	return db, renderer, ma
}

func TestHandleCreate_ValidationError(t *testing.T) {
	_, renderer, ma := setupValidatedHandlerTest(t)

	router := chi.NewRouter()
	ma.Routes(router)

	// Submit form with empty name — should trigger validation error.
	form := url.Values{
		"name": {""},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/validated", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, renderer.formCalled, "should render form with validation errors")
	assert.NotNil(t, renderer.lastErrors, "should pass validation errors to renderer")
	assert.True(t, renderer.lastErrors.HasField("Name"), "should have error on Name field")
}

func TestHandleUpdate_ValidationError(t *testing.T) {
	db, renderer, ma := setupValidatedHandlerTest(t)

	item := &validatedItem{Name: "Original"}
	_, err := db.NewInsert().Model(item).Exec(context.Background())
	require.NoError(t, err)

	router := chi.NewRouter()
	ma.Routes(router)

	// Submit form with empty name — should trigger validation error.
	form := url.Values{
		"name": {""},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/validated/%d", item.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, renderer.formCalled, "should render form with validation errors")
	assert.NotNil(t, renderer.lastErrors, "should pass validation errors to renderer")
	assert.True(t, renderer.lastErrors.HasField("Name"), "should have error on Name field")
}

func TestHandleUpdate_NotFound(t *testing.T) {
	_, _, ma := setupHandlerTest(t)

	r := newRouter(ma)
	form := url.Values{"name": {"Updated"}}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/items/999", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleNew_FieldChoices(t *testing.T) {
	_, renderer, ma := setupHandlerTest(t)
	ma.FieldChoices = map[string]ChoicesFunc{
		"Status": func(_ context.Context) ([]Choice, error) {
			return []Choice{
				{Value: "active", Label: "Active"},
				{Value: "inactive", Label: "Inactive"},
			}, nil
		},
	}

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/new", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, renderer.formCalled)

	var statusField *FormField
	for i := range renderer.lastFields {
		if renderer.lastFields[i].Name == "Status" {
			statusField = &renderer.lastFields[i]
			break
		}
	}
	require.NotNil(t, statusField, "Status field should exist")
	assert.Equal(t, "select", statusField.Type)
	assert.Len(t, statusField.Choices, 2)
	assert.Equal(t, "Active", statusField.Choices[0].Label)
}

func TestHandleDetail_FieldChoices(t *testing.T) {
	db, renderer, ma := setupHandlerTest(t)
	ma.FieldChoices = map[string]ChoicesFunc{
		"Status": func(_ context.Context) ([]Choice, error) {
			return []Choice{
				{Value: "active", Label: "Active"},
				{Value: "inactive", Label: "Inactive"},
			}, nil
		},
	}

	item := &testItem{Name: "Test", Status: "active"}
	_, err := db.NewInsert().Model(item).Exec(context.Background())
	require.NoError(t, err)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/items/%d", item.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, renderer.formCalled)

	var statusField *FormField
	for i := range renderer.lastFields {
		if renderer.lastFields[i].Name == "Status" {
			statusField = &renderer.lastFields[i]
			break
		}
	}
	require.NotNil(t, statusField)
	assert.Equal(t, "select", statusField.Type)
	assert.Len(t, statusField.Choices, 2)
}

func TestHandleCreate_FieldChoicesOnValidationError(t *testing.T) {
	_, renderer, ma := setupValidatedHandlerTest(t)
	ma.FieldChoices = map[string]ChoicesFunc{
		"Name": func(_ context.Context) ([]Choice, error) {
			return []Choice{{Value: "a", Label: "A"}}, nil
		},
	}

	router := chi.NewRouter()
	ma.Routes(router)

	form := url.Values{"name": {""}}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/validated", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.True(t, renderer.formCalled)
	var nameField *FormField
	for i := range renderer.lastFields {
		if renderer.lastFields[i].Name == "Name" {
			nameField = &renderer.lastFields[i]
			break
		}
	}
	require.NotNil(t, nameField)
	assert.Equal(t, "select", nameField.Type, "FieldChoices should be applied even on validation error re-render")
}

func TestRenderConfig_EmptyMessage(t *testing.T) {
	ma := &ModelAdmin[testItem]{
		Slug:        "items",
		DisplayName: "Item", DisplayPluralName: "Items",
	}
	cfg := ma.renderConfig()
	assert.Equal(t, "No items found.", cfg.EmptyMessage)
}

func TestRenderConfig_CustomEmptyMessage(t *testing.T) {
	ma := &ModelAdmin[testItem]{
		Slug:         "items",
		DisplayName:  "Item",
		EmptyMessage: "Nothing here!",
	}
	cfg := ma.renderConfig()
	assert.Equal(t, "Nothing here!", cfg.EmptyMessage)
}
