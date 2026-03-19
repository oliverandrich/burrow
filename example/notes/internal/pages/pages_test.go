package pages

import (
	"html/template"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ burrow.App             = (*App)(nil)
	_ burrow.HasRoutes       = (*App)(nil)
	_ burrow.HasNavItems     = (*App)(nil)
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

func TestHome_UsesRenderTemplate(t *testing.T) {
	exec := burrow.TemplateExecutor(func(_ *http.Request, name string, data map[string]any) (template.HTML, error) {
		if name == "test-layout" {
			return template.HTML("<layout>" + string(data["Content"].(template.HTML)) + "</layout>"), nil
		}
		return template.HTML("<p>" + name + "</p>"), nil
	})

	t.Run("normal request wraps in layout", func(t *testing.T) {
		ctx := burrow.WithTemplateExecutor(t.Context(), exec)
		ctx = burrow.WithLayout(ctx, "test-layout")
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		err := home(w, req)
		require.NoError(t, err)
		assert.Contains(t, w.Body.String(), "<layout>")
	})

	t.Run("HTMX request returns fragment only", func(t *testing.T) {
		ctx := burrow.WithTemplateExecutor(t.Context(), exec)
		ctx = burrow.WithLayout(ctx, "test-layout")
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		err := home(w, req)
		require.NoError(t, err)
		assert.Contains(t, w.Body.String(), "pages/home")
		assert.NotContains(t, w.Body.String(), "<layout>")
	})
}

func TestLogo_ReturnsNonEmpty(t *testing.T) {
	html := Logo()
	assert.NotEmpty(t, html)
	assert.Contains(t, string(html), "Burrow")
}

func TestFuncMap_ContainsExpectedEntries(t *testing.T) {
	app := New()
	fm := app.FuncMap()
	assert.Contains(t, fm, "alertClass")
}

func TestAlertClass(t *testing.T) {
	app := New()
	fm := app.FuncMap()
	alertClassFn := fm["alertClass"].(func(messages.Level) string)

	assert.Equal(t, "danger", alertClassFn(messages.Error))
	assert.Equal(t, "info", alertClassFn(messages.Info))
	assert.Equal(t, "success", alertClassFn(messages.Success))
	assert.Equal(t, "warning", alertClassFn(messages.Warning))
}

func TestTemplateFS_ReturnsNonNil(t *testing.T) {
	app := New()
	fsys := app.TemplateFS()
	assert.NotNil(t, fsys)
}

func TestRegister_RegistersIcons(t *testing.T) {
	app := New()
	cfg := &burrow.AppConfig{}
	require.NoError(t, app.Register(cfg))

	icons := cfg.IconFuncs()
	assert.Contains(t, icons, "iconHouse")
	assert.Contains(t, icons, "iconKey")
	assert.Contains(t, icons, "iconPuzzle")
	assert.Contains(t, icons, "iconLightning")
	assert.Contains(t, icons, "iconGear")
	assert.Contains(t, icons, "iconBoxArrowRight")
	assert.Contains(t, icons, "iconBoxArrowInRight")
}

func TestRoutes_RegistersHomeRoute(t *testing.T) {
	app := New()

	exec := burrow.TemplateExecutor(func(_ *http.Request, name string, data map[string]any) (template.HTML, error) {
		if name == "test-layout" {
			return template.HTML(string(data["Content"].(template.HTML))), nil
		}
		return template.HTML("<p>" + name + "</p>"), nil
	})

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithTemplateExecutor(r.Context(), exec)
			ctx = burrow.WithLayout(ctx, "test-layout")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	app.Routes(r)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "pages/home")
}
