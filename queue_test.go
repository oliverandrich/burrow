package burrow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithMaxRetries(t *testing.T) {
	cfg := JobConfig{MaxRetries: 3}
	opt := WithMaxRetries(10)
	opt(&cfg)
	assert.Equal(t, 10, cfg.MaxRetries)
}
