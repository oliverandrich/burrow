package smtpmail

import (
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppName(t *testing.T) {
	a := New()
	assert.Equal(t, "authmail-smtp", a.Name())
}

func TestAppRegister(t *testing.T) {
	a := New()
	err := a.Register(&burrow.AppConfig{})
	require.NoError(t, err)
}

func TestAppFlags(t *testing.T) {
	a := New()
	flags := a.Flags()
	assert.Len(t, flags, 6)
}

func TestAppWithRenderer(t *testing.T) {
	r := &stubRenderer{}
	a := New(WithRenderer(r))
	assert.NotNil(t, a.renderer)
}

func TestAppMailerNilBeforeConfigure(t *testing.T) {
	a := New()
	assert.Nil(t, a.Mailer())
}
