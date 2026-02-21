package auth

import (
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
