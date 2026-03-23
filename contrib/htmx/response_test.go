package htmx

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusStopPolling(t *testing.T) {
	assert.Equal(t, 286, StatusStopPolling)
}

func TestReselect(t *testing.T) {
	w := httptest.NewRecorder()
	Reselect(w, "#content")
	assert.Equal(t, "#content", w.Header().Get("HX-Reselect"))
}

func TestSmartRedirect_HTMX(t *testing.T) {
	w := httptest.NewRecorder()
	r := newGetRequest()
	r.Header.Set("HX-Request", "true")

	SmartRedirect(w, r, "/dashboard")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "/dashboard", w.Header().Get("HX-Redirect"))
}

func TestSmartRedirect_NonHTMX(t *testing.T) {
	w := httptest.NewRecorder()
	r := newGetRequest()

	SmartRedirect(w, r, "/dashboard")

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/dashboard", w.Header().Get("Location"))
	assert.Empty(t, w.Header().Get("HX-Redirect"))
}

func TestRenderOrRedirect_HTMX(t *testing.T) {
	w := httptest.NewRecorder()
	r := newGetRequest()
	r.Header.Set("HX-Request", "true")

	// Inject a mock template executor that returns fixed HTML.
	ctx := r.Context()
	ctx = burrow.WithTemplateExecutor(ctx, func(_ *http.Request, name string, _ map[string]any) (template.HTML, error) {
		return template.HTML("<div>" + name + "</div>"), nil //nolint:gosec // test-only, name is not user input
	})
	r = r.WithContext(ctx)

	data := map[string]any{"key": "value"}
	err := RenderOrRedirect(w, r, "/fallback", "notes/fragment", data)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "notes/fragment")
}

func TestRenderOrRedirect_NonHTMX(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/notes", nil)

	data := map[string]any{"key": "value"}
	err := RenderOrRedirect(w, r, "/notes", "notes/fragment", data)

	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/notes", w.Header().Get("Location"))
}
