package secure

import (
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/app"
	"github.com/stretchr/testify/assert"
)

func TestSecureHTTPSCapable(t *testing.T) {
	httpsCfg := &burrow.AppConfig{Config: &burrow.Config{}}
	httpsCfg.Config.Server.BaseURL = "https://example.com"

	httpCfg := func(mode string) *burrow.AppConfig {
		c := &burrow.AppConfig{Config: &burrow.Config{}}
		c.Config.Server.BaseURL = "http://localhost:8080"
		c.Config.Server.Forwarded = app.ForwardedConfig{Mode: mode}
		return c
	}

	tests := []struct {
		name       string
		cfg        *burrow.AppConfig
		forwardedX bool
		want       bool
	}{
		{"https base url", httpsCfg, false, true},
		{"http base, forwarded explicit private", httpCfg("private"), true, true},
		{"http base, forwarded explicit loopback", httpCfg("loopback"), true, true},
		{"http base, forwarded default (not explicit)", httpCfg("private"), false, false},
		{"http base, forwarded explicit off", httpCfg("off"), true, false},
		{"http base, no forwarded", httpCfg(""), false, false},
		{"nil config", &burrow.AppConfig{}, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, secureHTTPSCapable(tc.cfg, tc.forwardedX))
		})
	}
}

// Note: that secureHTTPSCapable==true populates HSTS opts (configure(true) emits
// Strict-Transport-Security on an SSL request) is already pinned by
// TestHSTSEnabledForHTTPS; no need to duplicate that here.
