package templates

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/admin/modeladmin"
)

type testItem struct { //nolint:govet // fieldalignment: test struct
	ID   int64 `bun:",pk,autoincrement"`
	Name string
}

func TestRenderWithLayout_SkipsLayoutForHTMX(t *testing.T) {
	// Set up a layout that wraps content in a marker.
	layoutCalled := false
	layout := func(w http.ResponseWriter, _ *http.Request, code int, content template.HTML, _ map[string]any) error {
		layoutCalled = true
		return burrow.HTML(w, code, "<layout>"+string(content)+"</layout>")
	}

	content := template.HTML("<p>fragment</p>")

	t.Run("normal request uses layout", func(t *testing.T) {
		layoutCalled = false
		ctx := burrow.WithLayout(context.Background(), layout)
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/admin/items", nil)
		w := httptest.NewRecorder()

		err := renderWithLayout(w, req, "Items", content)
		require.NoError(t, err)
		assert.True(t, layoutCalled)
		assert.Contains(t, w.Body.String(), "<layout>")
	})

	t.Run("HTMX request skips layout", func(t *testing.T) {
		layoutCalled = false
		ctx := burrow.WithLayout(context.Background(), layout)
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/admin/items", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		err := renderWithLayout(w, req, "Items", content)
		require.NoError(t, err)
		assert.False(t, layoutCalled)
		assert.Contains(t, w.Body.String(), "<p>fragment</p>")
		assert.NotContains(t, w.Body.String(), "<layout>")
	})
}

func TestDefaultRenderer_List(t *testing.T) {
	r := DefaultRenderer[testItem]()
	items := []testItem{
		{ID: 1, Name: "Alpha"},
		{ID: 2, Name: "Beta"},
	}
	page := burrow.PageResult{Page: 1, TotalCount: 2, TotalPages: 1}
	cfg := modeladmin.RenderConfig{
		Slug:       "items",
		Display:    "Items",
		ListFields: []string{"ID", "Name"},
		IDField:    "ID",
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items", nil)
	w := httptest.NewRecorder()

	err := r.List(w, req, items, page, cfg)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Alpha")
	assert.Contains(t, body, "Beta")
	assert.Contains(t, body, "Items")
}

func TestDefaultRenderer_List_WithRowActions(t *testing.T) {
	r := DefaultRenderer[testItem]()
	items := []testItem{{ID: 1, Name: "Alpha"}}
	page := burrow.PageResult{Page: 1, TotalCount: 1, TotalPages: 1}
	cfg := modeladmin.RenderConfig{
		Slug:          "items",
		Display:       "Items",
		ListFields:    []string{"ID", "Name"},
		IDField:       "ID",
		HasRowActions: true,
		RowActions: []modeladmin.RenderAction{
			{Slug: "retry", Label: "Retry", Method: "POST", Class: "btn-success"},
		},
		ItemActionSets: [][]modeladmin.RenderAction{
			{
				{Slug: "retry", Label: "Retry", Method: "POST", Class: "btn-success"},
			},
		},
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items", nil)
	w := httptest.NewRecorder()

	err := r.List(w, req, items, page, cfg)
	require.NoError(t, err)
	body := w.Body.String()
	assert.Contains(t, body, "Actions")
	assert.Contains(t, body, "Retry")
	assert.Contains(t, body, "btn-success")
	assert.Contains(t, body, "hx-post")
	assert.Contains(t, body, "/retry")
}

func TestDefaultRenderer_List_WithFilters(t *testing.T) {
	r := DefaultRenderer[testItem]()
	items := []testItem{{ID: 1, Name: "Alpha"}}
	page := burrow.PageResult{Page: 1, TotalCount: 1, TotalPages: 1}
	cfg := modeladmin.RenderConfig{
		Slug:       "items",
		Display:    "Items",
		ListFields: []string{"ID", "Name"},
		IDField:    "ID",
		Filters: []modeladmin.ActiveFilter{
			{
				Field: "status",
				Label: "Status",
				Choices: []modeladmin.ActiveChoice{
					{Label: "All", URL: "/admin/items", IsActive: false},
					{Value: "active", Label: "Active", URL: "/admin/items?status=active", IsActive: true},
					{Value: "archived", Label: "Archived", URL: "/admin/items?status=archived", IsActive: false},
				},
				HasActive: true,
			},
		},
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items?status=active", nil)
	w := httptest.NewRecorder()

	err := r.List(w, req, items, page, cfg)
	require.NoError(t, err)
	body := w.Body.String()
	assert.Contains(t, body, "Status:")
	assert.Contains(t, body, "Active")
	assert.Contains(t, body, "Archived")
	assert.Contains(t, body, "nav-pills")
	assert.Contains(t, body, "active")
}

func TestDefaultRenderer_Detail(t *testing.T) {
	r := DefaultRenderer[testItem]()
	item := &testItem{ID: 1, Name: "Alpha"}
	cfg := modeladmin.RenderConfig{
		Slug:       "items",
		Display:    "Items",
		ListFields: []string{"ID", "Name"},
		IDField:    "ID",
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items/1", nil)
	w := httptest.NewRecorder()

	err := r.Detail(w, req, item, cfg)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Alpha")
}

func TestDefaultRenderer_Form_Create(t *testing.T) {
	r := DefaultRenderer[testItem]()
	fields := modeladmin.AutoFields[testItem](nil)
	cfg := modeladmin.RenderConfig{
		Slug:      "items",
		Display:   "Items",
		CanCreate: true,
		IDField:   "ID",
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items/new", nil)
	w := httptest.NewRecorder()

	err := r.Form(w, req, nil, fields, nil, cfg)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "New Items")
	assert.Contains(t, body, "name=\"name\"")
}

func TestDefaultRenderer_Form_Edit(t *testing.T) {
	r := DefaultRenderer[testItem]()
	item := &testItem{ID: 42, Name: "Existing"}
	fields := modeladmin.AutoFields(item)
	cfg := modeladmin.RenderConfig{
		Slug:    "items",
		Display: "Items",
		CanEdit: true,
		IDField: "ID",
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items/42", nil)
	w := httptest.NewRecorder()

	err := r.Form(w, req, item, fields, nil, cfg)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Edit Items")
	assert.Contains(t, body, "Existing")
}

func TestDefaultRenderer_Form_WithValidationErrors(t *testing.T) {
	r := DefaultRenderer[testItem]()
	fields := modeladmin.AutoFields[testItem](nil)
	ve := &burrow.ValidationError{
		Errors: []burrow.FieldError{
			{Field: "name", Tag: "required", Message: "name is required"},
		},
	}
	cfg := modeladmin.RenderConfig{
		Slug:    "items",
		Display: "Items",
		IDField: "ID",
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items/new", nil)
	w := httptest.NewRecorder()

	err := r.Form(w, req, nil, fields, ve, cfg)
	require.NoError(t, err)
	body := w.Body.String()
	assert.Contains(t, body, "is-invalid")
	assert.Contains(t, body, "name is required")
}
