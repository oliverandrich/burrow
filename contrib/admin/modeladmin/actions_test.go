package modeladmin

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	_ "modernc.org/sqlite"
)

func TestRowAction_Defaults(t *testing.T) {
	a := RowAction{Slug: "retry", Label: "Retry"}
	assert.Equal(t, http.MethodPost, a.method())
	assert.Equal(t, "btn-outline-secondary", a.class())
}

func TestRowAction_CustomMethodAndClass(t *testing.T) {
	a := RowAction{
		Slug:   "cancel",
		Label:  "Cancel",
		Method: http.MethodDelete,
		Class:  "btn-danger",
	}
	assert.Equal(t, http.MethodDelete, a.method())
	assert.Equal(t, "btn-danger", a.class())
}

func TestRowAction_ToRenderAction(t *testing.T) {
	a := RowAction{
		Slug:    "retry",
		Label:   "Retry",
		Icon:    template.HTML("<i>r</i>"),
		Confirm: "Are you sure?",
	}
	ra := a.toRenderAction()
	assert.Equal(t, "retry", ra.Slug)
	assert.Equal(t, "Retry", ra.Label)
	assert.Equal(t, template.HTML("<i>r</i>"), ra.Icon)
	assert.Equal(t, http.MethodPost, ra.Method)
	assert.Equal(t, "btn-outline-secondary", ra.Class)
	assert.Equal(t, "Are you sure?", ra.Confirm)
}

func TestBuildItemActions_AllVisible(t *testing.T) {
	actions := []RowAction{
		{Slug: "retry", Label: "Retry"},
		{Slug: "cancel", Label: "Cancel"},
	}
	result := buildItemActions(actions, testItem{Status: "failed"})
	assert.Len(t, result.Actions, 2)
}

func TestBuildItemActions_ShowWhenFilters(t *testing.T) {
	actions := []RowAction{
		{
			Slug:  "retry",
			Label: "Retry",
			ShowWhen: func(item any) bool {
				if ti, ok := item.(testItem); ok {
					return ti.Status == "failed"
				}
				return false
			},
		},
		{Slug: "cancel", Label: "Cancel"},
	}

	t.Run("item matches ShowWhen", func(t *testing.T) {
		result := buildItemActions(actions, testItem{Status: "failed"})
		assert.Len(t, result.Actions, 2)
		assert.Equal(t, "retry", result.Actions[0].Slug)
	})

	t.Run("item does not match ShowWhen", func(t *testing.T) {
		result := buildItemActions(actions, testItem{Status: "active"})
		assert.Len(t, result.Actions, 1)
		assert.Equal(t, "cancel", result.Actions[0].Slug)
	})
}

func TestBuildItemActions_Empty(t *testing.T) {
	result := buildItemActions(nil, testItem{})
	assert.Empty(t, result.Actions)
}

func TestRowAction_RouteMounting(t *testing.T) {
	sqldb, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db := bun.NewDB(sqldb, sqlitedialect.New())
	t.Cleanup(func() { db.Close() })

	_, err = db.NewCreateTable().Model((*testItem)(nil)).Exec(context.Background())
	require.NoError(t, err)

	item := &testItem{Name: "Test", Status: "active"}
	_, err = db.NewInsert().Model(item).Exec(context.Background())
	require.NoError(t, err)

	actionCalled := false
	var actionID string

	ma := &ModelAdmin[testItem]{
		Slug:        "items",
		DisplayName: "Item", DisplayPluralName: "Items",
		DB:       db,
		Renderer: &mockRenderer{},
		RowActions: []RowAction{
			{
				Slug:  "retry",
				Label: "Retry",
				Handler: func(_ http.ResponseWriter, r *http.Request) error {
					actionCalled = true
					actionID = chi.URLParam(r, "id")
					return nil
				},
			},
		},
	}

	r := chi.NewRouter()
	ma.Routes(r)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/items/%d/retry", item.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, actionCalled)
	assert.Equal(t, fmt.Sprintf("%d", item.ID), actionID)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRowAction_DeleteMethod(t *testing.T) {
	sqldb, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db := bun.NewDB(sqldb, sqlitedialect.New())
	t.Cleanup(func() { db.Close() })

	_, err = db.NewCreateTable().Model((*testItem)(nil)).Exec(context.Background())
	require.NoError(t, err)

	item := &testItem{Name: "Test", Status: "active"}
	_, err = db.NewInsert().Model(item).Exec(context.Background())
	require.NoError(t, err)

	actionCalled := false

	ma := &ModelAdmin[testItem]{
		Slug:        "items",
		DisplayName: "Item", DisplayPluralName: "Items",
		DB:       db,
		Renderer: &mockRenderer{},
		RowActions: []RowAction{
			{
				Slug:   "cancel",
				Label:  "Cancel",
				Method: http.MethodDelete,
				Handler: func(_ http.ResponseWriter, _ *http.Request) error {
					actionCalled = true
					return nil
				},
			},
		},
	}

	r := chi.NewRouter()
	ma.Routes(r)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, fmt.Sprintf("/items/%d/cancel", item.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, actionCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleList_PassesActionsToRenderer(t *testing.T) {
	db, renderer, ma := setupHandlerTest(t)
	seedItems(t, db, 1)

	ma.RowActions = []RowAction{
		{Slug: "retry", Label: "Retry"},
	}

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, renderer.listCalled)
	assert.True(t, renderer.lastConfig.HasRowActions)
	require.Len(t, renderer.lastConfig.RowActions, 1)
	assert.Equal(t, "retry", renderer.lastConfig.RowActions[0].Slug)
	require.Len(t, renderer.lastConfig.ItemActionSets, 1)
	require.Len(t, renderer.lastConfig.ItemActionSets[0], 1)
	assert.Equal(t, "retry", renderer.lastConfig.ItemActionSets[0][0].Slug)
}

func TestHandleDetail_PassesActionsToRenderer(t *testing.T) {
	db, renderer, ma := setupHandlerTest(t)
	ma.CanEdit = false // use detail view, not form

	item := &testItem{Name: "Test", Status: "active"}
	_, err := db.NewInsert().Model(item).Exec(context.Background())
	require.NoError(t, err)

	ma.RowActions = []RowAction{
		{Slug: "retry", Label: "Retry"},
	}

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/items/%d", item.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, renderer.detailCalled)
	assert.True(t, renderer.lastConfig.HasRowActions)
}
