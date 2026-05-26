// Smoke tests for the burrow facade — every wrapper round-trips through the
// sub-package it delegates to. The real logic each wrapper covers is tested
// against the sub-package directly in its own test suite.
package burrow_test

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/registry"
)

// smokeApp is the minimum-viable App used across these smoke tests.
type smokeApp struct{ name string }

func (a smokeApp) Name() string { return a.name }

func TestNewServer_RegistersApps(t *testing.T) {
	a := smokeApp{name: "alpha"}
	srv := burrow.NewServer(a)

	require.NotNil(t, srv)
	require.NotNil(t, srv.Registry())

	apps := registry.Apps(srv.Registry())
	require.Len(t, apps, 1)
	assert.Equal(t, "alpha", apps[0].Name())
}

func TestNewHTTPError_PreservesFields(t *testing.T) {
	err := burrow.NewHTTPError(http.StatusNotFound, "item not found")

	require.NotNil(t, err)
	assert.Equal(t, http.StatusNotFound, err.Code)
	assert.Equal(t, "item not found", err.Message)
	assert.Equal(t, "item not found", err.Error())
}

func TestHandle_ReturnsHTTPHandlerFunc(t *testing.T) {
	called := false
	fn := func(w http.ResponseWriter, _ *http.Request) error {
		called = true
		return burrow.Text(w, http.StatusOK, "ok")
	}

	h := burrow.Handle(fn)
	require.NotNil(t, h)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	h(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestParsePageRequest_Defaults(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
	pr := burrow.ParsePageRequest(req)

	assert.Equal(t, 20, pr.Limit, "default limit is 20")
	assert.Equal(t, 0, pr.Page, "missing ?page yields 0 (not used)")
}

func TestPageURL_PreservesQuery(t *testing.T) {
	got := burrow.PageURL("/items", "search=foo&filter=open", 3)
	assert.Contains(t, got, "search=foo")
	assert.Contains(t, got, "filter=open")
	assert.Contains(t, got, "page=3")
}

func TestContextHelpers_RoundTrip(t *testing.T) {
	ctx := context.Background()

	t.Run("layout", func(t *testing.T) {
		ctx := burrow.WithLayout(ctx, "myapp/layout")
		assert.Equal(t, "myapp/layout", burrow.Layout(ctx))
	})

	t.Run("nav items", func(t *testing.T) {
		items := []burrow.NavItem{{Label: "Home", URL: "/"}}
		ctx := burrow.WithNavItems(ctx, items)
		assert.Equal(t, items, burrow.NavItems(ctx))
	})

	t.Run("request path", func(t *testing.T) {
		ctx := burrow.WithRequestPath(ctx, "/admin/users")
		assert.Equal(t, "/admin/users", burrow.RequestPath(ctx))
	})

	t.Run("template executor", func(t *testing.T) {
		exec := func(_ context.Context, name string, _ map[string]any) (template.HTML, error) {
			return template.HTML(name), nil //nolint:gosec // smoke test
		}
		ctx := burrow.WithTemplateExecutor(ctx, exec)

		got := burrow.TemplateExec(ctx)
		require.NotNil(t, got)

		out, err := got(ctx, "frag", nil)
		require.NoError(t, err)
		assert.Equal(t, template.HTML("frag"), out)
	})

	t.Run("context value", func(t *testing.T) {
		type key struct{}
		ctx := burrow.WithContextValue(ctx, key{}, "value")
		got, ok := burrow.ContextValue[string](ctx, key{})
		require.True(t, ok)
		assert.Equal(t, "value", got)
	})
}

func TestClientIP_RoundTripThroughChiMiddleware(t *testing.T) {
	// burrow.ClientIP and burrow.ClientIPAddr re-export chi's GetClientIP /
	// GetClientIPAddr so contribs can read the framework-wide source of
	// client IP without importing chi/v5/middleware directly. Verify the
	// re-export by running a real chi middleware in front of a handler that
	// reads through the burrow facade.
	r := chi.NewRouter()
	r.Use(chimw.ClientIPFromHeader("X-Real-IP"))
	var typedAddrValid bool
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		typedAddrValid = burrow.ClientIPAddr(req.Context()).IsValid()
		_, _ = w.Write([]byte(burrow.ClientIP(req.Context())))
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "198.51.100.7")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, "198.51.100.7", rec.Body.String(),
		"burrow.ClientIP must return the value chi's ClientIPFromHeader middleware stored")
	assert.True(t, typedAddrValid, "burrow.ClientIPAddr must also be populated by chi's middleware")
}

func TestClientIP_EmptyWhenNoMiddleware(t *testing.T) {
	// Without chi's ClientIP middleware in the chain, burrow.ClientIP should
	// return the zero value — contribs depending on it must implement a
	// fallback (ratelimit does, via r.RemoteAddr).
	ctx := context.Background()
	assert.Empty(t, burrow.ClientIP(ctx))
	assert.False(t, burrow.ClientIPAddr(ctx).IsValid())
}

func TestAuthHelpers_NoChecker_ReturnFalse(t *testing.T) {
	ctx := context.Background()
	assert.False(t, burrow.IsAuthenticated(ctx))
	assert.False(t, burrow.IsStaff(ctx))
	assert.False(t, burrow.IsAdmin(ctx))
}

func TestAuthHelpers_WithChecker(t *testing.T) {
	ctx := burrow.WithAuthChecker(context.Background(), burrow.AuthChecker{
		IsAuthenticated: func() bool { return true },
		IsStaff:         func() bool { return true },
		IsAdmin:         func() bool { return false },
	})
	assert.True(t, burrow.IsAuthenticated(ctx))
	assert.True(t, burrow.IsStaff(ctx))
	assert.False(t, burrow.IsAdmin(ctx))
}

func TestDefineTask_BuildsDefinition(t *testing.T) {
	type payload struct{ N int }

	def := burrow.DefineTask("smoke-add", func(_ context.Context, _ payload) error {
		return nil
	}, burrow.WithMaxRetries(3), burrow.WithPriority(5))

	require.NotNil(t, def)
	assert.Equal(t, "smoke-add", def.Name())
}

func TestDefineResultTask_BuildsDefinition(t *testing.T) {
	type payload struct{ N int }
	type result struct{ V int }

	def := burrow.DefineResultTask("smoke-sum", func(_ context.Context, p payload) (result, error) {
		return result{V: p.N * 2}, nil
	})

	require.NotNil(t, def)
	assert.Equal(t, "smoke-sum", def.Name())
}

func TestRenderFragment_NoExecutor_ErrNoTemplateExecutor(t *testing.T) {
	_, err := burrow.RenderFragment(context.Background(), "any", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, burrow.ErrNoTemplateExecutor)
}
