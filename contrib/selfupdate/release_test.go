package selfupdate

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleaseClient_FetchLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/repos/me/myapp/releases/latest", r.URL.Path)
		assert.Equal(t, userAgent, r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "v1.2.3",
			"name": "v1.2.3",
			"assets": [
				{"name": "myapp-1.2.3-linux-x86_64.tar.gz", "browser_download_url": "https://example.com/a.tgz"},
				{"name": "checksums.txt",                   "browser_download_url": "https://example.com/c.txt"}
			]
		}`))
	}))
	t.Cleanup(srv.Close)

	c := newReleaseClient(srv.URL, "me", "myapp", srv.Client())
	rel, err := c.FetchLatest(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "v1.2.3", rel.TagName)
	assert.Len(t, rel.Assets, 2)
	assert.Equal(t, "myapp-1.2.3-linux-x86_64.tar.gz", rel.Assets[0].Name)
	assert.Equal(t, "https://example.com/c.txt", rel.Assets[1].URL)
}

func TestReleaseClient_FetchByTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/repos/me/myapp/releases/tags/v1.0.0", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.0.0","assets":[]}`))
	}))
	t.Cleanup(srv.Close)

	c := newReleaseClient(srv.URL, "me", "myapp", srv.Client())
	rel, err := c.FetchByTag(t.Context(), "v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", rel.TagName)
}

func TestReleaseClient_FetchByTag_EscapesSpecialChars(t *testing.T) {
	var seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// URL.Path is already unescaped, but we record what the
		// client sent verbatim by checking the raw path.
		seenPath = r.URL.EscapedPath()
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	c := newReleaseClient(srv.URL, "me", "myapp", srv.Client())
	_, _ = c.FetchByTag(t.Context(), "v1.0.0?injected=1")
	assert.Contains(t, seenPath, "v1.0.0%3Finjected=1",
		"`?` in --to must be percent-escaped so it doesn't start a query string")
}

func TestReleaseClient_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	c := newReleaseClient(srv.URL, "me", "myapp", srv.Client())
	_, err := c.FetchLatest(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no release")
}

func TestReleaseClient_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1234567890")
		http.Error(w, "rate limit exceeded", http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	c := newReleaseClient(srv.URL, "me", "myapp", srv.Client())
	_, err := c.FetchLatest(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate-limited")
	assert.Contains(t, err.Error(), "1234567890")
}

func TestReleaseClient_JSONResponseBodyCapped(t *testing.T) {
	huge := strings.Repeat("x", int(maxJSONResponseBytes)+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.0.0","assets":[{"name":"junk","browser_download_url":"` + huge + `"}]}`))
	}))
	t.Cleanup(srv.Close)

	c := newReleaseClient(srv.URL, "me", "myapp", srv.Client())
	_, err := c.FetchLatest(t.Context())
	require.Error(t, err, "oversized JSON body must be rejected")
}

func TestNormaliseHost(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"github.com", "https://api.github.com"},
		{"https://github.com", "https://api.github.com"},
		{"api.github.com", "https://api.github.com"},
		{"GitHub.com", "https://api.github.com"},
		{"HTTPS://GITHUB.COM", "https://api.github.com"},
		{"codeberg.org", "https://codeberg.org/api/v1"},
		{"https://codeberg.org", "https://codeberg.org/api/v1"},
		{"forgejo.example.com", "https://forgejo.example.com/api/v1"},
		{"https://forgejo.example.com/", "https://forgejo.example.com/api/v1"},
		{"http://localhost:8080", "http://localhost:8080/api/v1"},
		{"HTTP://localhost:8080", "http://localhost:8080/api/v1"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, normaliseHost(tt.in))
		})
	}
}
