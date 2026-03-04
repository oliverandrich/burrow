package templates

import (
	"testing"

	"codeberg.org/oliverandrich/burrow/contrib/authmail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface check.
var _ authmail.Renderer = DefaultRenderer()

func TestDefaultRendererVerificationHTML(t *testing.T) {
	r := DefaultRenderer()
	subject, html, err := r.RenderVerificationHTML("https://example.com/verify?token=abc123")

	require.NoError(t, err)
	assert.Equal(t, "Verify your email", subject)
	assert.Contains(t, html, "https://example.com/verify?token=abc123")
	assert.Contains(t, html, "Verify Email")
	assert.Contains(t, html, "<html>")
}

func TestDefaultRendererVerificationText(t *testing.T) {
	r := DefaultRenderer()
	subject, text, err := r.RenderVerificationText("https://example.com/verify?token=abc123")

	require.NoError(t, err)
	assert.Equal(t, "Verify your email", subject)
	assert.Contains(t, text, "https://example.com/verify?token=abc123")
}

func TestDefaultRendererInviteHTML(t *testing.T) {
	r := DefaultRenderer()
	subject, html, err := r.RenderInviteHTML("https://example.com/register?invite=xyz789")

	require.NoError(t, err)
	assert.Equal(t, "You've been invited", subject)
	assert.Contains(t, html, "https://example.com/register?invite=xyz789")
	assert.Contains(t, html, "Accept Invite")
	assert.Contains(t, html, "<html>")
}

func TestDefaultRendererInviteText(t *testing.T) {
	r := DefaultRenderer()
	subject, text, err := r.RenderInviteText("https://example.com/register?invite=xyz789")

	require.NoError(t, err)
	assert.Equal(t, "You've been invited", subject)
	assert.Contains(t, text, "https://example.com/register?invite=xyz789")
}
