package auth

import (
	"bytes"
	"html/template"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAdminUserFormIsHxPost pins the JS-required convention: admin forms
// submit via hx-post, not a traditional method=post. The form fallback was
// dead code because contrib/auth is WebAuthn-only and contrib/admin sits
// behind RequireStaff — both demand JS already. See docs/contrib/admin.md
// ("JavaScript required") for the full rationale.
func TestAdminUserFormIsHxPost(t *testing.T) {
	body, err := os.ReadFile("templates/admin_user_form.html")
	require.NoError(t, err)
	src := string(body)

	assert.Contains(t, src, `<form hx-post="/admin/users/{{ .User.ID }}"`,
		"admin_user_form must submit via hx-post on the form element (admin mandates JS)")
	assert.NotContains(t, src, `<form method="post" action="/admin/users/`,
		"admin_user_form must not fall back to method=post (dead code)")
	assert.NotContains(t, src, `<form method="POST" action="/admin/users/`,
		"admin_user_form must not fall back to method=POST (dead code)")
}

// TestRecoveryCodesAckFormIsHxPost pins the same convention for the
// recovery-codes acknowledgement form: the user is logged in (just
// regenerated codes) and JS is already required by WebAuthn.
func TestRecoveryCodesAckFormIsHxPost(t *testing.T) {
	body, err := os.ReadFile("templates/recovery.html")
	require.NoError(t, err)
	src := string(body)

	assert.Contains(t, src, `<form hx-post="/auth/recovery-codes/ack">`,
		"recovery-codes ack form must submit via hx-post on the form element")
	assert.NotContains(t, src, `<form method="POST" action="/auth/recovery-codes/ack"`,
		"recovery-codes ack form must not use method=POST")
	assert.NotContains(t, src, `<form method="post" action="/auth/recovery-codes/ack"`,
		"recovery-codes ack form must not use method=post")
}

// TestAuthLayoutHasNoscriptFallback pins that the public auth layout renders
// a <noscript> block explaining that passkeys require JavaScript — the
// alternative is a silent dead-end where the WebAuthn button does nothing.
// See docs/contrib/auth.md ("JavaScript required") for the rationale.
func TestAuthLayoutHasNoscriptFallback(t *testing.T) {
	funcMap := template.FuncMap{
		"t":         func(key string) string { return key },
		"lang":      func() string { return "en" },
		"staticURL": func(string) string { return "" },
	}
	body, err := os.ReadFile("templates/layout.html")
	require.NoError(t, err)
	tmpl, err := template.New("auth/layout").Funcs(funcMap).Parse(string(body))
	require.NoError(t, err, "auth/layout must parse cleanly")

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "auth/layout", map[string]any{
		"Title":   "Sign in",
		"Content": template.HTML(`<button>passkey</button>`), //nolint:gosec // test
	})
	require.NoError(t, err)
	out := buf.String()

	// Extract the <noscript>…</noscript> region and assert keys are inside it,
	// so a misplaced {{ t "..." }} outside the block would fail the pin.
	open := strings.Index(out, "<noscript>")
	close := strings.Index(out, "</noscript>")
	require.Greater(t, open, -1, "auth/layout must contain a <noscript> opening tag so JS-off users see a message rather than a silently-broken passkey button")
	require.Greater(t, close, open, "auth/layout must close its <noscript> block")
	region := out[open:close]

	assert.Contains(t, region, "auth-noscript-title",
		"auth-noscript-title key must be rendered inside the <noscript> block")
	assert.Contains(t, region, "auth-noscript-body",
		"auth-noscript-body key must be rendered inside the <noscript> block")
	assert.Contains(t, region, "auth-noscript-link-text",
		"auth-noscript-link-text key must be rendered inside the <noscript> block")
	assert.Contains(t, region, `rel="noopener noreferrer"`,
		"the external how-to-enable-JS link must carry rel=noopener noreferrer")
}
