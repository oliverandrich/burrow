package auth

import (
	"os"
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
