package templates

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/admin/modeladmin"
	"github.com/oliverandrich/burrow/contrib/messages"
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
		Slug:        "items",
		DisplayName: "Item", DisplayPluralName: "Items",
		ListFields:      []string{"ID", "Name"},
		ListFieldLabels: []string{"ID", "Name"},
		IDField:         "ID",
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
		Slug:              "items",
		DisplayName:       "Item",
		DisplayPluralName: "Items",
		ListFields:        []string{"ID", "Name"},
		IDField:           "ID",
		HasRowActions:     true,
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
	assert.Contains(t, body, "modeladmin-actions")
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
		Slug:              "items",
		DisplayName:       "Item",
		DisplayPluralName: "Items",
		ListFields:        []string{"ID", "Name"},
		IDField:           "ID",
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

func TestDefaultRenderer_List_WithMessages(t *testing.T) {
	r := DefaultRenderer[testItem]()
	items := []testItem{{ID: 1, Name: "Alpha"}}
	page := burrow.PageResult{Page: 1, TotalCount: 1, TotalPages: 1}
	cfg := modeladmin.RenderConfig{
		Slug:              "items",
		DisplayName:       "Item",
		DisplayPluralName: "Items",
		ListFields:        []string{"ID", "Name"},
		ListFieldLabels:   []string{"ID", "Name"},
		IDField:           "ID",
	}

	msgs := []messages.Message{
		{Level: messages.Success, Text: "https://example.com/auth/register?invite=abc123"},
		{Level: messages.Error, Text: "Something went wrong"},
	}
	ctx := messages.Inject(context.Background(), msgs)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/admin/items", nil)
	w := httptest.NewRecorder()

	err := r.List(w, req, items, page, cfg)
	require.NoError(t, err)
	body := w.Body.String()
	assert.Contains(t, body, "alert-success")
	assert.Contains(t, body, "https://example.com/auth/register?invite=abc123")
	assert.Contains(t, body, "alert-danger")
	assert.Contains(t, body, "Something went wrong")
}

func TestDefaultRenderer_Detail(t *testing.T) {
	r := DefaultRenderer[testItem]()
	item := &testItem{ID: 1, Name: "Alpha"}
	cfg := modeladmin.RenderConfig{
		Slug:        "items",
		DisplayName: "Item", DisplayPluralName: "Items",
		ListFields:      []string{"ID", "Name"},
		ListFieldLabels: []string{"ID", "Name"},
		IDField:         "ID",
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
		Slug:              "items",
		DisplayName:       "Item",
		DisplayPluralName: "Items",
		CanCreate:         true,
		IDField:           "ID",
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items/new", nil)
	w := httptest.NewRecorder()

	err := r.Form(w, req, nil, fields, nil, cfg)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "modeladmin-new Item")
	assert.Contains(t, body, "name=\"name\"")
}

func TestDefaultRenderer_Form_Edit(t *testing.T) {
	r := DefaultRenderer[testItem]()
	item := &testItem{ID: 42, Name: "Existing"}
	fields := modeladmin.AutoFields(item)
	cfg := modeladmin.RenderConfig{
		Slug:              "items",
		DisplayName:       "Item",
		DisplayPluralName: "Items",
		CanEdit:           true,
		IDField:           "ID",
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items/42", nil)
	w := httptest.NewRecorder()

	err := r.Form(w, req, item, fields, nil, cfg)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "modeladmin-edit Item")
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
		Slug:              "items",
		DisplayName:       "Item",
		DisplayPluralName: "Items",
		IDField:           "ID",
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items/new", nil)
	w := httptest.NewRecorder()

	err := r.Form(w, req, nil, fields, ve, cfg)
	require.NoError(t, err)
	body := w.Body.String()
	assert.Contains(t, body, "is-invalid")
	assert.Contains(t, body, "name is required")
}

// --- isTruthy tests ---

func TestIsTruthy(t *testing.T) {
	t.Run("bool true", func(t *testing.T) {
		assert.True(t, isTruthy(true))
	})
	t.Run("bool false", func(t *testing.T) {
		assert.False(t, isTruthy(false))
	})
	t.Run("int non-zero", func(t *testing.T) {
		assert.True(t, isTruthy(42))
	})
	t.Run("int zero", func(t *testing.T) {
		assert.False(t, isTruthy(0))
	})
	t.Run("int64 non-zero", func(t *testing.T) {
		assert.True(t, isTruthy(int64(1)))
	})
	t.Run("int64 zero", func(t *testing.T) {
		assert.False(t, isTruthy(int64(0)))
	})
	t.Run("uint non-zero", func(t *testing.T) {
		assert.True(t, isTruthy(uint(5)))
	})
	t.Run("uint zero", func(t *testing.T) {
		assert.False(t, isTruthy(uint(0)))
	})
	t.Run("uint64 non-zero", func(t *testing.T) {
		assert.True(t, isTruthy(uint64(100)))
	})
	t.Run("uint64 zero", func(t *testing.T) {
		assert.False(t, isTruthy(uint64(0)))
	})
	t.Run("string true", func(t *testing.T) {
		assert.True(t, isTruthy("true"))
	})
	t.Run("string on", func(t *testing.T) {
		assert.True(t, isTruthy("on"))
	})
	t.Run("string 1", func(t *testing.T) {
		assert.True(t, isTruthy("1"))
	})
	t.Run("string false", func(t *testing.T) {
		assert.False(t, isTruthy("false"))
	})
	t.Run("string empty", func(t *testing.T) {
		assert.False(t, isTruthy(""))
	})
	t.Run("unknown type returns false", func(t *testing.T) {
		assert.False(t, isTruthy([]int{1, 2, 3}))
	})
	t.Run("nil returns false", func(t *testing.T) {
		assert.False(t, isTruthy(nil))
	})
}

// --- formatDateValue tests ---

func TestFormatDateValue(t *testing.T) {
	t.Run("valid time", func(t *testing.T) {
		tm := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, "2024-06-15", formatDateValue(tm))
	})
	t.Run("zero time returns empty", func(t *testing.T) {
		assert.Empty(t, formatDateValue(time.Time{}))
	})
	t.Run("non-time type uses Sprintf", func(t *testing.T) {
		assert.Equal(t, "hello", formatDateValue("hello"))
	})
	t.Run("nil uses Sprintf", func(t *testing.T) {
		assert.Equal(t, "<nil>", formatDateValue(nil))
	})
	t.Run("int uses Sprintf", func(t *testing.T) {
		assert.Equal(t, "42", formatDateValue(42))
	})
}

// --- pageRange tests ---

func TestPageRange(t *testing.T) {
	t.Run("zero pages", func(t *testing.T) {
		assert.Empty(t, pageRange(0))
	})
	t.Run("one page", func(t *testing.T) {
		assert.Equal(t, []int{1}, pageRange(1))
	})
	t.Run("five pages", func(t *testing.T) {
		assert.Equal(t, []int{1, 2, 3, 4, 5}, pageRange(5))
	})
}

// --- dict tests ---

func TestDict(t *testing.T) {
	t.Run("key-value pairs", func(t *testing.T) {
		result := dict("a", 1, "b", "two")
		assert.Equal(t, 1, result["a"])
		assert.Equal(t, "two", result["b"])
	})
	t.Run("odd number of args ignores last", func(t *testing.T) {
		result := dict("a", 1, "orphan")
		assert.Equal(t, 1, result["a"])
		_, ok := result["orphan"]
		assert.False(t, ok)
	})
	t.Run("non-string key skipped", func(t *testing.T) {
		result := dict(42, "value")
		assert.Empty(t, result)
	})
	t.Run("empty args", func(t *testing.T) {
		result := dict()
		assert.Empty(t, result)
	})
}

// --- alertClass tests ---

func TestAlertClass(t *testing.T) {
	fm := funcMap()
	alertClassFn := fm["alertClass"].(func(messages.Level) string)

	t.Run("error maps to danger", func(t *testing.T) {
		assert.Equal(t, "danger", alertClassFn(messages.Error))
	})
	t.Run("success passes through", func(t *testing.T) {
		assert.Equal(t, "success", alertClassFn(messages.Success))
	})
	t.Run("info passes through", func(t *testing.T) {
		assert.Equal(t, "info", alertClassFn(messages.Info))
	})
	t.Run("warning passes through", func(t *testing.T) {
		assert.Equal(t, "warning", alertClassFn(messages.Warning))
	})
}

// --- funcMap add/sub tests ---

func TestFuncMapAddSub(t *testing.T) {
	fm := funcMap()
	addFn := fm["add"].(func(int, int) int)
	subFn := fm["sub"].(func(int, int) int)

	assert.Equal(t, 5, addFn(2, 3))
	assert.Equal(t, 1, subFn(3, 2))
}

// --- ConfirmDelete tests ---

func TestDefaultRenderer_ConfirmDelete(t *testing.T) {
	r := DefaultRenderer[testItem]()
	item := &testItem{ID: 7, Name: "ToDelete"}
	cfg := modeladmin.RenderConfig{
		Slug:              "items",
		DisplayName:       "Item",
		DisplayPluralName: "Items",
		ListFields:        []string{"ID", "Name"},
		ListFieldLabels:   []string{"ID", "Name"},
		IDField:           "ID",
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items/7/delete", nil)
	w := httptest.NewRecorder()

	err := r.ConfirmDelete(w, req, item, cfg)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "ToDelete")
}

// --- executeTemplate error path ---

func TestExecuteTemplate_UnknownTemplate(t *testing.T) {
	_, err := executeTemplate("nonexistent/template", func(key string) string { return key }, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "execute template nonexistent/template")
}

// --- renderWithLayout without layout ---

func TestRenderWithLayout_NoLayout(t *testing.T) {
	// No layout set in context — should render bare content.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/items", nil)
	w := httptest.NewRecorder()

	err := renderWithLayout(w, req, "Items", template.HTML("<p>bare</p>"))
	require.NoError(t, err)
	assert.Contains(t, w.Body.String(), "<p>bare</p>")
}

// --- hasFieldError tests ---

func TestHasFieldError(t *testing.T) {
	t.Run("nil error returns false", func(t *testing.T) {
		assert.False(t, hasFieldError(nil, "name"))
	})
	t.Run("field present", func(t *testing.T) {
		ve := &burrow.ValidationError{
			Errors: []burrow.FieldError{{Field: "name", Tag: "required"}},
		}
		assert.True(t, hasFieldError(ve, "name"))
	})
	t.Run("field absent", func(t *testing.T) {
		ve := &burrow.ValidationError{
			Errors: []burrow.FieldError{{Field: "email", Tag: "required"}},
		}
		assert.False(t, hasFieldError(ve, "name"))
	})
}
