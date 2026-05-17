package admin

import (
	"bytes"
	"html/template"
	"io/fs"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAdminPaginationTemplate pins the admin/pagination render contract.
func TestAdminPaginationTemplate(t *testing.T) {
	app := New()
	sub, err := fs.Sub(app.TemplateFS(), ".")
	require.NoError(t, err)

	funcMap := template.FuncMap{
		"pageURL":     burrow.PageURL,
		"pageNumbers": burrow.PageNumbers,
		"add":         func(a, b int) int { return a + b },
		"sub":         func(a, b int) int { return a - b },
	}
	// Parse only pagination.html so this test stays decoupled from the
	// funcmap surface of admin's other templates.
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(sub, "pagination.html")
	require.NoError(t, err, "pagination.html must exist and parse")
	require.NotNil(t, tmpl.Lookup("admin/pagination"), `admin/pagination define must exist`)

	t.Run("renders nav for multi-page result", func(t *testing.T) {
		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "admin/pagination", map[string]any{
			"Page":     burrow.PageResult{Page: 2, TotalPages: 5, HasMore: true},
			"BasePath": "/admin/test",
			"RawQuery": "q=hello",
		})
		require.NoError(t, err)
		out := buf.String()

		assert.Contains(t, out, `aria-label="Page navigation"`)
		assert.Contains(t, out, `/admin/test?page=1&amp;q=hello`, "Previous link should point at page 1 (HTML-escaped)")
		assert.Contains(t, out, `/admin/test?page=3&amp;q=hello`, "Next link should point at page 3 (HTML-escaped)")
		assert.Contains(t, out, `aria-current="page" class="rounded border border-emerald-500`, "current page must be marked")
		assert.Contains(t, out, `>2</span>`, "current page marker must wrap page number 2")
	})

	t.Run("renders nothing for single-page result", func(t *testing.T) {
		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "admin/pagination", map[string]any{
			"Page":     burrow.PageResult{Page: 1, TotalPages: 1, HasMore: false},
			"BasePath": "/admin/test",
			"RawQuery": "",
		})
		require.NoError(t, err)
		assert.Empty(t, bytes.TrimSpace(buf.Bytes()), "TotalPages <= 1 must produce no markup")
	})

	t.Run("disables Previous on first page", func(t *testing.T) {
		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "admin/pagination", map[string]any{
			"Page":     burrow.PageResult{Page: 1, TotalPages: 3, HasMore: true},
			"BasePath": "/admin/test",
			"RawQuery": "",
		})
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, `aria-disabled="true">&laquo;`, "Previous arrow must be in a disabled span")
		assert.NotContains(t, out, `aria-disabled="true">&raquo;`, "Next arrow must still be a working link")
	})

	t.Run("disables Next on last page", func(t *testing.T) {
		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "admin/pagination", map[string]any{
			"Page":     burrow.PageResult{Page: 3, TotalPages: 3, HasMore: false},
			"BasePath": "/admin/test",
			"RawQuery": "",
		})
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, `aria-disabled="true">&raquo;`, "Next arrow must be in a disabled span")
		assert.NotContains(t, out, `aria-disabled="true">&laquo;`, "Previous arrow must still be a working link")
	})

	t.Run("renders ellipsis for large page counts", func(t *testing.T) {
		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, "admin/pagination", map[string]any{
			"Page":     burrow.PageResult{Page: 10, TotalPages: 20, HasMore: true},
			"BasePath": "/admin/test",
			"RawQuery": "",
		})
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, `&hellip;`, "ellipsis branch (pageNumbers returns -1) must render")
		assert.Contains(t, out, `aria-hidden="true"`, "ellipsis must be aria-hidden")
	})
}
