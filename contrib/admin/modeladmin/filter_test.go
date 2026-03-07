package modeladmin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildActiveFilters_NoFilters(t *testing.T) {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items", nil)
	result := buildActiveFilters(nil, r, nil)
	assert.Nil(t, result)
}

func TestBuildActiveFilters_SelectFilter(t *testing.T) {
	filters := []FilterDef{
		{
			Field: "status",
			Label: "Status",
			Type:  "select",
			Choices: []Choice{
				{Value: "pending", Label: "Pending"},
				{Value: "done", Label: "Done"},
			},
		},
	}
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items", nil)
	result := buildActiveFilters(filters, r, nil)

	require.Len(t, result, 1)
	af := result[0]
	assert.Equal(t, "status", af.Field)
	assert.Equal(t, "Status", af.Label)
	assert.False(t, af.HasActive)

	// "All" + 2 choices = 3
	require.Len(t, af.Choices, 3)
	assert.Equal(t, "All", af.Choices[0].Label)
	assert.True(t, af.Choices[0].IsActive, "All should be active when no filter is set")
	assert.Equal(t, "Pending", af.Choices[1].Label)
	assert.False(t, af.Choices[1].IsActive)
	assert.Equal(t, "Done", af.Choices[2].Label)
	assert.False(t, af.Choices[2].IsActive)
}

func TestBuildActiveFilters_ActiveFilter(t *testing.T) {
	filters := []FilterDef{
		{
			Field: "status",
			Label: "Status",
			Type:  "select",
			Choices: []Choice{
				{Value: "pending", Label: "Pending"},
				{Value: "done", Label: "Done"},
			},
		},
	}
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items?status=done", nil)
	result := buildActiveFilters(filters, r, nil)

	require.Len(t, result, 1)
	af := result[0]
	assert.True(t, af.HasActive)
	assert.False(t, af.Choices[0].IsActive, "All should not be active")
	assert.False(t, af.Choices[1].IsActive, "Pending should not be active")
	assert.True(t, af.Choices[2].IsActive, "Done should be active")
}

func TestBuildActiveFilters_WithTranslateFunc(t *testing.T) {
	translations := map[string]string{
		"filter-status":  "Status",
		"filter-all":     "Alle",
		"filter-pending": "Ausstehend",
		"filter-done":    "Erledigt",
	}
	tf := func(_ *http.Request, key string) string {
		if v, ok := translations[key]; ok {
			return v
		}
		return key
	}

	filters := []FilterDef{
		{
			Field:       "status",
			Label:       "Status",
			LabelKey:    "filter-status",
			Type:        "select",
			AllLabelKey: "filter-all",
			Choices: []Choice{
				{Value: "pending", Label: "Pending", LabelKey: "filter-pending"},
				{Value: "done", Label: "Done", LabelKey: "filter-done"},
			},
		},
	}
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items", nil)
	result := buildActiveFilters(filters, r, tf)

	require.Len(t, result, 1)
	af := result[0]
	assert.Equal(t, "Status", af.Label)
	assert.Equal(t, "Alle", af.Choices[0].Label)
	assert.Equal(t, "Ausstehend", af.Choices[1].Label)
	assert.Equal(t, "Erledigt", af.Choices[2].Label)
}

func TestFilterURL_SetsParam(t *testing.T) {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items", nil)
	u := filterURL(r, "status", "done")
	assert.Equal(t, "/admin/items?status=done", u)
}

func TestFilterURL_RemovesParam(t *testing.T) {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items?status=done", nil)
	u := filterURL(r, "status", "")
	assert.Equal(t, "/admin/items", u)
}

func TestFilterURL_PreservesOtherParams(t *testing.T) {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items?q=search&sort=name", nil)
	u := filterURL(r, "status", "done")
	assert.Contains(t, u, "q=search")
	assert.Contains(t, u, "sort=name")
	assert.Contains(t, u, "status=done")
}

func TestFilterURL_ResetsPage(t *testing.T) {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items?page=3&q=test", nil)
	u := filterURL(r, "status", "done")
	assert.NotContains(t, u, "page=")
	assert.Contains(t, u, "q=test")
	assert.Contains(t, u, "status=done")
}

func TestFilterURL_NoParams(t *testing.T) {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items", nil)
	u := filterURL(r, "status", "")
	assert.Equal(t, "/admin/items", u)
}
