package csrf

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenContext(t *testing.T) {
	ctx := context.Background()
	ctx = WithToken(ctx, "abc123")

	assert.Equal(t, "abc123", Token(ctx))
}

func TestTokenMissing(t *testing.T) {
	ctx := context.Background()
	assert.Empty(t, Token(ctx))
}
