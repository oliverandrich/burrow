package modeladmin

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oliverandrich/burrow/forms"
)

func TestColumnText(t *testing.T) {
	t.Run("string field", func(t *testing.T) {
		item := testArticle{Title: "Hello <World>"}
		got := columnText(item, "Title")
		assert.Equal(t, "Hello <World>", got)
	})

	t.Run("int field", func(t *testing.T) {
		item := testArticle{Views: 42}
		got := columnText(item, "Views")
		assert.Equal(t, "42", got)
	})

	t.Run("bool true", func(t *testing.T) {
		item := testArticle{Active: true}
		got := columnText(item, "Active")
		assert.Equal(t, "true", got)
	})

	t.Run("bool false", func(t *testing.T) {
		item := testArticle{Active: false}
		got := columnText(item, "Active")
		assert.Equal(t, "false", got)
	})

	t.Run("time field", func(t *testing.T) {
		item := testArticle{CreatedAt: time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)}
		got := columnText(item, "CreatedAt")
		assert.Equal(t, "2024-06-15 10:30", got)
	})

	t.Run("zero time", func(t *testing.T) {
		item := testArticle{CreatedAt: time.Time{}}
		got := columnText(item, "CreatedAt")
		assert.Empty(t, got)
	})

	t.Run("stringer field", func(t *testing.T) {
		type article struct {
			Category *testCategory
		}
		item := article{Category: &testCategory{ID: 1, Name: "Science"}}
		got := columnText(item, "Category")
		assert.Equal(t, "Science", got)
	})

	t.Run("nil pointer", func(t *testing.T) {
		type article struct {
			Category *testCategory
		}
		item := article{Category: nil}
		got := columnText(item, "Category")
		assert.Empty(t, got)
	})

	t.Run("non-existent field", func(t *testing.T) {
		item := testArticle{}
		got := columnText(item, "NonExistent")
		assert.Empty(t, got)
	})

	t.Run("pointer to non-nil string", func(t *testing.T) {
		type withPtr struct {
			Name *string
		}
		name := "hello"
		item := withPtr{Name: &name}
		got := columnText(item, "Name")
		assert.Equal(t, "hello", got)
	})

	t.Run("skips computed columns", func(t *testing.T) {
		item := testArticle{Title: "Hello"}
		// columnText does not accept computed columns — it returns the struct field.
		got := columnText(item, "Title")
		assert.Equal(t, "Hello", got)
	})
}

func TestHandleExportCSV(t *testing.T) {
	db, _, ma := setupHandlerTest(t)
	ma.ListFields = []string{"Name", "Status"}
	ma.CanExport = true
	seedItems(t, db, 3)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/export.csv", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "items-")
	assert.Contains(t, w.Header().Get("Content-Disposition"), ".csv")

	reader := csv.NewReader(strings.NewReader(w.Body.String()))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	// 1 header + 3 data rows
	assert.Len(t, records, 4)
	assert.Equal(t, []string{"Name", "Status"}, records[0])
}

func TestHandleExportCSV_WithSearch(t *testing.T) {
	db, _, ma := setupHandlerTest(t)
	ma.ListFields = []string{"Name", "Status"}
	ma.SearchFields = []string{"name"}
	ma.CanExport = true

	// Seed diverse items.
	for _, name := range []string{"Alpha", "Beta", "AlphaTwo"} {
		item := &testItem{Name: name, Status: "active"}
		_, err := db.NewInsert().Model(item).Exec(t.Context())
		require.NoError(t, err)
	}

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/export.csv?q=Alpha", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	reader := csv.NewReader(strings.NewReader(w.Body.String()))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	// 1 header + 2 matching rows
	assert.Len(t, records, 3)
}

func TestHandleExportCSV_NotRegisteredWhenDisabled(t *testing.T) {
	_, _, ma := setupHandlerTest(t)
	ma.CanExport = false

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/export.csv", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Route not registered, so chi returns 405 or 404.
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHandleExportJSON(t *testing.T) {
	db, _, ma := setupHandlerTest(t)
	ma.ListFields = []string{"Name", "Status"}
	ma.CanExport = true
	seedItems(t, db, 3)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/export.json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "items-")
	assert.Contains(t, w.Header().Get("Content-Disposition"), ".json")

	var result []map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 3)
	// Each object should have keys matching ListFields.
	for _, row := range result {
		assert.Contains(t, row, "Name")
		assert.Contains(t, row, "Status")
	}
}

func TestHandleExportJSON_WithFilter(t *testing.T) {
	db, _, ma := setupHandlerTest(t)
	ma.ListFields = []string{"Name", "Status"}
	ma.CanExport = true
	ma.Filters = []FilterDef{
		{Field: "status", Label: "Status", Type: "select", Choices: []forms.Choice{
			{Value: "active", Label: "Active"},
			{Value: "inactive", Label: "Inactive"},
		}},
	}

	// Seed items with different statuses.
	for _, s := range []string{"active", "active", "inactive"} {
		item := &testItem{Name: fmt.Sprintf("Item-%s", s), Status: s}
		_, err := db.NewInsert().Model(item).Exec(t.Context())
		require.NoError(t, err)
	}

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/export.json?status=inactive", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result []map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}
