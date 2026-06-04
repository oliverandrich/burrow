package app

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestIsHTTPS(t *testing.T) {
	t.Run("forwarded-proto https flag", func(t *testing.T) {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		r = r.WithContext(WithForwardedProto(r.Context(), "https"))
		assert.True(t, RequestIsHTTPS(r))
	})

	t.Run("forwarded-proto http flag", func(t *testing.T) {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		r = r.WithContext(WithForwardedProto(r.Context(), "http"))
		assert.False(t, RequestIsHTTPS(r))
	})

	t.Run("real TLS, no flag", func(t *testing.T) {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		r.TLS = &tls.ConnectionState{}
		assert.True(t, RequestIsHTTPS(r))
	})

	t.Run("plain http, no flag", func(t *testing.T) {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		assert.False(t, RequestIsHTTPS(r))
	})
}

func TestForwardedProto_EmptyWhenUnset(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	assert.Empty(t, ForwardedProto(r.Context()))
}

func TestValidateForwarded(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ForwardedConfig
		wantErr string // substring; "" = no error
	}{
		{"default empty mode ok", ForwardedConfig{}, ""},
		{"private ok no cidrs", ForwardedConfig{Mode: "private"}, ""},
		{"loopback ok", ForwardedConfig{Mode: "loopback"}, ""},
		{"off ok", ForwardedConfig{Mode: "off"}, ""},
		{"trusted-cidrs valid", ForwardedConfig{Mode: "trusted-cidrs", TrustedCIDRs: []string{"10.0.0.0/8"}}, ""},
		{"trusted-cidrs empty fails", ForwardedConfig{Mode: "trusted-cidrs"}, "requires --forwarded-trusted-cidrs"},
		{"trusted-cidrs invalid cidr fails", ForwardedConfig{Mode: "trusted-cidrs", TrustedCIDRs: []string{"garbage"}}, "invalid CIDR"},
		{"cidrs without trusted-cidrs mode fails", ForwardedConfig{Mode: "private", TrustedCIDRs: []string{"10.0.0.0/8"}}, "only valid with --forwarded-mode=trusted-cidrs"},
		{"unknown mode fails", ForwardedConfig{Mode: "wat"}, "unknown forwarded mode"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{}
			c.Server.Forwarded = tc.cfg
			err := c.ValidateForwarded(nil)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
