package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserFromContext(t *testing.T) {
	user := &User{ID: 42, Username: "alice"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	got := GetUser(req)
	require.NotNil(t, got)
	assert.Equal(t, int64(42), got.ID)
	assert.True(t, IsAuthenticated(req))
}

func TestGetUserFromEmptyContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.Nil(t, GetUser(req))
	assert.False(t, IsAuthenticated(req))
}

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
