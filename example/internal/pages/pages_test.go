package pages

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ burrow.App           = (*App)(nil)
	_ burrow.HasRoutes     = (*App)(nil)
	_ burrow.HasNavItems   = (*App)(nil)
	_ burrow.HasMiddleware = (*App)(nil)
)

func TestAppName(t *testing.T) {
	app := New()
	assert.Equal(t, "pages", app.Name())
}

func TestNavItems(t *testing.T) {
	app := New()
	items := app.NavItems()
	require.Len(t, items, 1)
	assert.Equal(t, "Home", items[0].Label)
	assert.Equal(t, "/", items[0].URL)
	assert.Equal(t, 1, items[0].Position)
}

func TestMiddleware(t *testing.T) {
	app := New()
	mw := app.Middleware()
	require.Len(t, mw, 1)
}

func TestHomeHandler_WithLayout(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := burrow.WithLayout(req.Context(), func(title string, content templ.Component) templ.Component {
		return content
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := home(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Welcome to Burrow")
}

func TestHomeHandler_WithoutLayout(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	err := home(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Welcome to Burrow")
}
