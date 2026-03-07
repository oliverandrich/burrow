package pages

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ burrow.App             = (*App)(nil)
	_ burrow.HasRoutes       = (*App)(nil)
	_ burrow.HasNavItems     = (*App)(nil)
	_ burrow.HasMiddleware   = (*App)(nil)
	_ burrow.HasTranslations = (*App)(nil)
	_ burrow.HasTemplates    = (*App)(nil)
	_ burrow.HasFuncMap      = (*App)(nil)
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

func TestTranslationFS(t *testing.T) {
	app := New()
	fsys := app.TranslationFS()
	require.NotNil(t, fsys)

	matches, err := fs.Glob(fsys, "translations/*.toml")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(matches), 2, "expected at least en and de translation files")
}

func TestMiddleware_InjectsRequestPath(t *testing.T) {
	app := New()
	mws := app.Middleware()
	require.Len(t, mws, 1)

	var captured string
	handler := mws[0](http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured, _ = r.Context().Value(ctxKeyRequestPath{}).(string)
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, "/notes", captured)
}

func TestVisibleNavItems_FiltersAuthOnly(t *testing.T) {
	ctx := context.Background()
	ctx = burrow.WithNavItems(ctx, []burrow.NavItem{
		{Label: "Home", URL: "/", Position: 1},
		{Label: "Notes", URL: "/notes", AuthOnly: true, Position: 2},
	})

	items := visibleNavItems(ctx)

	assert.Len(t, items, 1)
	assert.Equal(t, "Home", items[0].Label)
}

func TestVisibleNavItems_ShowsAuthOnlyWhenAuthenticated(t *testing.T) {
	ctx := context.Background()
	ctx = burrow.WithNavItems(ctx, []burrow.NavItem{
		{Label: "Home", URL: "/", Position: 1},
		{Label: "Notes", URL: "/notes", AuthOnly: true, Position: 2},
	})
	ctx = auth.WithUser(ctx, &auth.User{ID: 1, Username: "test"})

	items := visibleNavItems(ctx)

	assert.Len(t, items, 2)
}

func TestVisibleNavItems_FiltersAdminOnly(t *testing.T) {
	ctx := context.Background()
	ctx = burrow.WithNavItems(ctx, []burrow.NavItem{
		{Label: "Admin", URL: "/admin", AdminOnly: true},
	})
	ctx = auth.WithUser(ctx, &auth.User{ID: 1, Username: "test", Role: "user"})

	items := visibleNavItems(ctx)

	assert.Empty(t, items)
}

func TestVisibleNavItems_ShowsAdminOnlyForAdmins(t *testing.T) {
	ctx := context.Background()
	ctx = burrow.WithNavItems(ctx, []burrow.NavItem{
		{Label: "Admin", URL: "/admin", AdminOnly: true},
	})
	ctx = auth.WithUser(ctx, &auth.User{ID: 1, Username: "admin", Role: "admin"})

	items := visibleNavItems(ctx)

	assert.Len(t, items, 1)
}

func TestNavLinkClass_ActiveOnExactMatch(t *testing.T) {
	assert.Equal(t, "nav-link active", navLinkClass("/", "/"))
}

func TestNavLinkClass_HomeNotActiveOnSubpath(t *testing.T) {
	assert.Equal(t, "nav-link", navLinkClass("/notes", "/"))
}

func TestNavLinkClass_PrefixMatch(t *testing.T) {
	assert.Equal(t, "nav-link active", navLinkClass("/notes/1", "/notes"))
}

func TestNavLinkClass_NoMatch(t *testing.T) {
	assert.Equal(t, "nav-link", navLinkClass("/settings", "/notes"))
}

func TestNavLinkClass_EmptyCurrentPath(t *testing.T) {
	assert.Equal(t, "nav-link", navLinkClass("", "/notes"))
}

func TestLayout_ReturnsNonNil(t *testing.T) {
	fn := Layout()
	assert.NotNil(t, fn)
}

func TestLogo_ReturnsNonEmpty(t *testing.T) {
	html := Logo()
	assert.NotEmpty(t, html)
	assert.Contains(t, string(html), "Burrow")
}

func TestFuncMap_ContainsExpectedEntries(t *testing.T) {
	app := New()
	fm := app.FuncMap()

	expectedKeys := []string{
		"iconHouse", "iconKey", "iconPuzzle", "iconLightning",
		"iconGear", "iconBoxArrowRight", "iconBoxArrowInRight",
		"alertClass",
	}
	for _, key := range expectedKeys {
		assert.Contains(t, fm, key)
	}
}

func TestAlertClass(t *testing.T) {
	app := New()
	fm := app.FuncMap()
	alertClassFn := fm["alertClass"].(func(string) string)

	assert.Equal(t, "danger", alertClassFn("error"))
	assert.Equal(t, "info", alertClassFn("info"))
	assert.Equal(t, "success", alertClassFn("success"))
	assert.Equal(t, "warning", alertClassFn("warning"))
}

func TestTemplateFS_ReturnsNonNil(t *testing.T) {
	app := New()
	fsys := app.TemplateFS()
	assert.NotNil(t, fsys)
}
