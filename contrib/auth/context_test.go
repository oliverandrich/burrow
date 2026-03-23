package auth

import (
	"context"
	"html/template"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentUser(t *testing.T) {
	user := &User{ID: 7, Username: "bob"}
	ctx := WithUser(context.Background(), user)

	got := CurrentUser(ctx)
	require.NotNil(t, got)
	assert.Equal(t, int64(7), got.ID)
	assert.Equal(t, "bob", got.Username)
}

func TestCurrentUserEmpty(t *testing.T) {
	assert.Nil(t, CurrentUser(context.Background()))
}

func TestIsAuthenticated(t *testing.T) {
	assert.False(t, IsAuthenticated(context.Background()))

	ctx := WithUser(context.Background(), &User{ID: 1})
	assert.True(t, IsAuthenticated(ctx))
}

func TestMustCurrentUser(t *testing.T) {
	user := &User{ID: 42, Username: "alice"}
	ctx := WithUser(context.Background(), user)

	got := MustCurrentUser(ctx)
	require.NotNil(t, got)
	assert.Equal(t, int64(42), got.ID)
	assert.Equal(t, "alice", got.Username)
}

func TestMustCurrentUserPanicsWithoutUser(t *testing.T) {
	assert.PanicsWithValue(t,
		"auth: MustCurrentUser called without authenticated user — is RequireAuth middleware applied?",
		func() { MustCurrentUser(context.Background()) },
	)
}

func TestLogoEmpty(t *testing.T) {
	assert.Empty(t, Logo(context.Background()))
}

func TestWithLogo(t *testing.T) {
	logo := template.HTML(`<img src="logo.png"/>`)
	ctx := WithLogo(context.Background(), logo)

	got := Logo(ctx)
	assert.Equal(t, logo, got)
}
