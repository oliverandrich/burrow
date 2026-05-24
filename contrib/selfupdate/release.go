package selfupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Release is the subset of a Gitea/GitHub release payload that
// selfupdate needs. Tag is the SCM tag (with leading "v" preserved),
// Assets are the downloadable artefacts attached to the release.
type Release struct {
	TagName string
	Assets  []ReleaseAsset
}

// ReleaseAsset is a single file attached to a release.
type ReleaseAsset struct {
	Name string
	URL  string
}

// maxJSONResponseBytes caps the size of release-JSON responses so a
// hostile or buggy host cannot OOM the process by streaming an
// unbounded body.
const maxJSONResponseBytes = 1 << 20 // 1 MiB

// userAgent is sent with every request so Gitea/Forgejo proxies that
// drop UA-less traffic still serve us, and so the maintainers of the
// upstream host can see who is calling them.
const userAgent = "burrow-selfupdate"

// newDefaultHTTPClient returns an http.Client with sensible overall
// and connection timeouts so a hung remote does not freeze the
// `update` command. Tests and proxied environments can override via
// [WithHTTPClient].
func newDefaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

// releaseClient knows how to fetch a release from a Gitea-shaped
// host (GitHub, Codeberg, Forgejo). It uses no authentication —
// public-repo Releases endpoints are open on all three.
type releaseClient struct {
	apiBase string
	owner   string
	repo    string
	http    *http.Client
}

func newReleaseClient(host, owner, repo string, hc *http.Client) *releaseClient {
	if hc == nil {
		hc = newDefaultHTTPClient()
	}
	return &releaseClient{
		apiBase: normaliseHost(host),
		owner:   owner,
		repo:    repo,
		http:    hc,
	}
}

// FetchLatest returns the most recent non-draft, non-prerelease.
func (c *releaseClient) FetchLatest(ctx context.Context) (Release, error) {
	return c.fetch(ctx, "/repos/"+url.PathEscape(c.owner)+"/"+url.PathEscape(c.repo)+"/releases/latest")
}

// FetchByTag pins to a specific tag (e.g. "v1.2.3"), enabling
// `<app> update --to v1.2.3` style rollbacks.
func (c *releaseClient) FetchByTag(ctx context.Context, tag string) (Release, error) {
	return c.fetch(ctx, "/repos/"+url.PathEscape(c.owner)+"/"+url.PathEscape(c.repo)+"/releases/tags/"+url.PathEscape(tag))
}

func (c *releaseClient) fetch(ctx context.Context, path string) (Release, error) {
	requestURL := c.apiBase + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return Release{}, fmt.Errorf("selfupdate: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("selfupdate: GET %s: %w", requestURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return Release{}, fmt.Errorf("selfupdate: no release found at %s — check repo/host coords", requestURL)
	}
	if resp.StatusCode == http.StatusForbidden && strings.Contains(resp.Header.Get("X-RateLimit-Remaining"), "0") {
		reset := resp.Header.Get("X-RateLimit-Reset")
		return Release{}, fmt.Errorf("selfupdate: %s returned HTTP 403 (rate-limited); reset at unix %s", requestURL, reset)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Release{}, fmt.Errorf("selfupdate: %s returned HTTP %d: %s", requestURL, resp.StatusCode, body)
	}

	var raw struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxJSONResponseBytes)).Decode(&raw); err != nil {
		return Release{}, fmt.Errorf("selfupdate: decode %s: %w", requestURL, err)
	}
	if raw.TagName == "" {
		return Release{}, errors.New("selfupdate: release payload missing tag_name")
	}

	rel := Release{TagName: raw.TagName, Assets: make([]ReleaseAsset, len(raw.Assets))}
	for i, a := range raw.Assets {
		rel.Assets[i] = ReleaseAsset{Name: a.Name, URL: a.URL}
	}
	return rel, nil
}

// normaliseHost translates a user-supplied host string into an API
// base URL. Known special-case: github.com (any case) maps to
// api.github.com; Gitea-derived hosts (Codeberg, Forgejo) serve
// under <host>/api/v1. An explicit http:// scheme is preserved — used
// by tests with httptest.Server. Otherwise https:// is assumed.
func normaliseHost(host string) string {
	scheme := "https://"
	h := strings.TrimSuffix(host, "/")
	switch {
	case hasPrefixFold(h, "http://"):
		scheme = "http://"
		h = h[len("http://"):]
	case hasPrefixFold(h, "https://"):
		h = h[len("https://"):]
	}
	switch strings.ToLower(h) {
	case "github.com", "api.github.com":
		return scheme + "api.github.com"
	}
	return scheme + h + "/api/v1"
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}
