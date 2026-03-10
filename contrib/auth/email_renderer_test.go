package auth

import (
	"context"
	"testing"

	"github.com/oliverandrich/burrow/contrib/authmail"
	"github.com/oliverandrich/burrow/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface check.
var _ authmail.Renderer = DefaultEmailRenderer()

func TestDefaultEmailRendererVerificationHTMLFallback(t *testing.T) {
	r := DefaultEmailRenderer()
	subject, html, err := r.RenderVerificationHTML(context.Background(), "https://example.com/verify?token=abc123")

	require.NoError(t, err)
	assert.Equal(t, "email-verification-subject", subject)
	assert.Contains(t, html, "https://example.com/verify?token=abc123")
	assert.Contains(t, html, "email-verification-heading")
	assert.Contains(t, html, "email-verification-button")
	assert.Contains(t, html, "<html>")
}

func TestDefaultEmailRendererVerificationTextFallback(t *testing.T) {
	r := DefaultEmailRenderer()
	subject, text, err := r.RenderVerificationText(context.Background(), "https://example.com/verify?token=abc123")

	require.NoError(t, err)
	assert.Equal(t, "email-verification-subject", subject)
	assert.Contains(t, text, "https://example.com/verify?token=abc123")
	assert.Contains(t, text, "email-verification-heading")
}

func TestDefaultEmailRendererInviteHTMLFallback(t *testing.T) {
	r := DefaultEmailRenderer()
	subject, html, err := r.RenderInviteHTML(context.Background(), "https://example.com/register?invite=xyz789")

	require.NoError(t, err)
	assert.Equal(t, "email-invite-subject", subject)
	assert.Contains(t, html, "https://example.com/register?invite=xyz789")
	assert.Contains(t, html, "email-invite-heading")
	assert.Contains(t, html, "email-invite-button")
	assert.Contains(t, html, "<html>")
}

func TestDefaultEmailRendererInviteTextFallback(t *testing.T) {
	r := DefaultEmailRenderer()
	subject, text, err := r.RenderInviteText(context.Background(), "https://example.com/register?invite=xyz789")

	require.NoError(t, err)
	assert.Equal(t, "email-invite-subject", subject)
	assert.Contains(t, text, "https://example.com/register?invite=xyz789")
	assert.Contains(t, text, "email-invite-heading")
}

// newLocalizedCtx creates a context with i18n localizer for the given language,
// using the auth translation files.
func newLocalizedCtx(t *testing.T, lang string) context.Context {
	t.Helper()
	bundle, err := i18n.NewTestBundle("en", translationFS)
	require.NoError(t, err)
	return bundle.WithLocale(context.Background(), lang)
}

func TestDefaultEmailRendererVerificationHTMLWithLocale(t *testing.T) {
	ctx := newLocalizedCtx(t, "en")
	r := DefaultEmailRenderer()
	subject, html, err := r.RenderVerificationHTML(ctx, "https://example.com/verify?token=abc123")

	require.NoError(t, err)
	assert.Equal(t, "Verify your email", subject)
	assert.Contains(t, html, "Verify your email")
	assert.Contains(t, html, "Verify Email")
	assert.Contains(t, html, "https://example.com/verify?token=abc123")
}

func TestDefaultEmailRendererVerificationHTMLWithGermanLocale(t *testing.T) {
	ctx := newLocalizedCtx(t, "de")
	r := DefaultEmailRenderer()
	subject, html, err := r.RenderVerificationHTML(ctx, "https://example.com/verify?token=abc123")

	require.NoError(t, err)
	assert.Equal(t, "E-Mail-Adresse bestätigen", subject)
	assert.Contains(t, html, "E-Mail-Adresse bestätigen")
	assert.Contains(t, html, "E-Mail bestätigen")
}

func TestDefaultEmailRendererInviteHTMLWithGermanLocale(t *testing.T) {
	ctx := newLocalizedCtx(t, "de")
	r := DefaultEmailRenderer()
	subject, html, err := r.RenderInviteHTML(ctx, "https://example.com/register?invite=xyz789")

	require.NoError(t, err)
	assert.Equal(t, "Sie wurden eingeladen", subject)
	assert.Contains(t, html, "Sie wurden eingeladen")
	assert.Contains(t, html, "Einladung annehmen")
}

func TestDefaultEmailRendererVerificationTextWithLocale(t *testing.T) {
	ctx := newLocalizedCtx(t, "en")
	r := DefaultEmailRenderer()
	subject, text, err := r.RenderVerificationText(ctx, "https://example.com/verify?token=abc123")

	require.NoError(t, err)
	assert.Equal(t, "Verify your email", subject)
	assert.Contains(t, text, "Verify your email")
	assert.Contains(t, text, "https://example.com/verify?token=abc123")
}

func TestDefaultEmailRendererInviteTextWithGermanLocale(t *testing.T) {
	ctx := newLocalizedCtx(t, "de")
	r := DefaultEmailRenderer()
	subject, text, err := r.RenderInviteText(ctx, "https://example.com/register?invite=xyz789")

	require.NoError(t, err)
	assert.Equal(t, "Sie wurden eingeladen", subject)
	assert.Contains(t, text, "Sie wurden eingeladen")
}

func TestDefaultEmailRendererVerificationText(t *testing.T) {
	r := DefaultEmailRenderer()

	subject, text, err := r.RenderVerificationText(context.Background(), "http://localhost/verify")
	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.Contains(t, text, "http://localhost/verify")
}

func TestDefaultEmailRendererInviteText(t *testing.T) {
	r := DefaultEmailRenderer()

	subject, text, err := r.RenderInviteText(context.Background(), "http://localhost/register")
	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.Contains(t, text, "http://localhost/register")
}
