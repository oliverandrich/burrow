package auth

import (
	"context"
	"io"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserFromContext(t *testing.T) {
	user := &User{ID: 7, Username: "bob"}
	ctx := WithUser(context.Background(), user)

	got := UserFromContext(ctx)
	require.NotNil(t, got)
	assert.Equal(t, int64(7), got.ID)
	assert.Equal(t, "bob", got.Username)
}

func TestUserFromContextEmpty(t *testing.T) {
	assert.Nil(t, UserFromContext(context.Background()))
}

func TestIsAuthenticated(t *testing.T) {
	assert.False(t, IsAuthenticated(context.Background()))

	ctx := WithUser(context.Background(), &User{ID: 1})
	assert.True(t, IsAuthenticated(ctx))
}

func TestLogoFromContextEmpty(t *testing.T) {
	assert.Nil(t, LogoFromContext(context.Background()))
}

func TestWithLogo(t *testing.T) {
	logo := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := io.WriteString(w, "<img src=\"logo.png\"/>")
		return err
	})
	ctx := WithLogo(context.Background(), logo)

	got := LogoFromContext(ctx)
	require.NotNil(t, got)
}

func TestAdminEditFlags(t *testing.T) {
	ctx := context.Background()

	// Defaults to false when not set.
	assert.False(t, IsAdminEditSelf(ctx))
	assert.False(t, IsAdminEditLastAdmin(ctx))

	// Set flags.
	ctx = withAdminEditFlags(ctx, true, true)
	assert.True(t, IsAdminEditSelf(ctx))
	assert.True(t, IsAdminEditLastAdmin(ctx))

	// Different values.
	ctx = withAdminEditFlags(context.Background(), false, true)
	assert.False(t, IsAdminEditSelf(ctx))
	assert.True(t, IsAdminEditLastAdmin(ctx))
}
