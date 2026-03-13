package auth

import (
	"context"
	"html/template"
	"testing"

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
	assert.Empty(t, LogoFromContext(context.Background()))
}

func TestWithLogo(t *testing.T) {
	logo := template.HTML(`<img src="logo.png"/>`)
	ctx := WithLogo(context.Background(), logo)

	got := LogoFromContext(ctx)
	assert.Equal(t, logo, got)
}
