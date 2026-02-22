package layout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"github.com/stretchr/testify/assert"
)

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
	ctx := context.WithValue(context.Background(), ctxKeyRequestPath{}, "/")

	assert.Equal(t, "nav-link active", navLinkClass(ctx, "/"))
}

func TestNavLinkClass_HomeNotActiveOnSubpath(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKeyRequestPath{}, "/notes")

	assert.Equal(t, "nav-link", navLinkClass(ctx, "/"))
}

func TestNavLinkClass_PrefixMatch(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKeyRequestPath{}, "/notes/1")

	assert.Equal(t, "nav-link active", navLinkClass(ctx, "/notes"))
}

func TestNavLinkClass_NoMatch(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKeyRequestPath{}, "/settings")

	assert.Equal(t, "nav-link", navLinkClass(ctx, "/notes"))
}

func TestNavLinkClass_EmptyContext(t *testing.T) {
	ctx := context.Background()

	assert.Equal(t, "nav-link", navLinkClass(ctx, "/notes"))
}

func TestMiddleware_InjectsRequestPath(t *testing.T) {
	mw := Middleware()
	var captured string
	handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured, _ = r.Context().Value(ctxKeyRequestPath{}).(string)
	}))

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, "/notes", captured)
}

func TestLayout_ReturnsNonNil(t *testing.T) {
	fn := Layout()
	assert.NotNil(t, fn)
}
