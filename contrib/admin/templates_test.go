package admin

import (
	"html/template"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAdminLayoutParses pins that admin/layout.html is well-formed Go
// html/template. Unit tests elsewhere stub the executor, so without this
// check a missing `{{ end }}` (or an introduced syntax error during the
// hx-post migration) would only surface at server boot.
func TestAdminLayoutParses(t *testing.T) {
	funcMap := template.FuncMap{
		"t":             func(...any) string { return "" },
		"lang":          func() string { return "en" },
		"staticURL":     func(string) string { return "" },
		"csrfToken":     func() string { return "" },
		"csrfHxHeaders": func() template.HTMLAttr { return "" },
		"currentUser":   func() any { return nil },
		"messages":      func() []any { return nil },
	}
	body, err := os.ReadFile("templates/layout.html")
	require.NoError(t, err)
	tmpl, err := template.New("admin/layout").Funcs(funcMap).Parse(string(body))
	require.NoError(t, err, "admin/layout must parse cleanly")
	assert.NotNil(t, tmpl.Lookup("admin/layout"))
}

// TestAdminLayoutLogoutFormIsHxPost pins the JS-required convention: the
// admin layout's logout form submits via hx-post under the body's hx-boost
// container, and does not opt out of boosting. The previous form fallback
// was dead code because every admin page is gated behind WebAuthn login.
// See docs/contrib/admin.md ("JavaScript required") for the full rationale.
func TestAdminLayoutLogoutFormIsHxPost(t *testing.T) {
	body, err := os.ReadFile("templates/layout.html")
	require.NoError(t, err)
	src := string(body)

	assert.Contains(t, src, `<form hx-post="/auth/logout">`,
		"admin layout logout form must submit via hx-post on the form element (admin mandates JS)")
	assert.NotContains(t, src, `<form method="post" action="/auth/logout"`,
		"admin layout logout form must not fall back to method=post (dead code)")
	assert.NotContains(t, src, `hx-boost="false"`,
		"admin layout must not opt out of hx-boost — the body container drives nav and form submission consistently")
}
