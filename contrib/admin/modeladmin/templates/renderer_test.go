package templates

import (
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

	req := httptest.NewRequest(http.MethodGet, "/admin/items", nil)
	w := httptest.NewRecorder()

	err := r.List(w, req, items, page, cfg)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Alpha")
	assert.Contains(t, body, "Beta")
	assert.Contains(t, body, "Items")
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

	req := httptest.NewRequest(http.MethodGet, "/admin/items/1", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/admin/items/new", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/admin/items/42", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/admin/items/new", nil)
	w := httptest.NewRecorder()

	err := r.Form(w, req, nil, fields, ve, cfg)
	require.NoError(t, err)
	body := w.Body.String()
	assert.Contains(t, body, "is-invalid")
	assert.Contains(t, body, "name is required")
}
