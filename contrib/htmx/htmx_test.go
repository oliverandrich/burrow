package htmx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newGetRequest() *http.Request {
	return httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
}

func TestRequest_IsHTMX(t *testing.T) {
	r := newGetRequest()
	assert.False(t, Request(r).IsHTMX())

	r.Header.Set("HX-Request", "true")
	assert.True(t, Request(r).IsHTMX())
}

func TestRequest_IsBoosted(t *testing.T) {
	r := newGetRequest()
	assert.False(t, Request(r).IsBoosted())

	r.Header.Set("HX-Boosted", "true")
	assert.True(t, Request(r).IsBoosted())
}

func TestRequest_Target(t *testing.T) {
	r := newGetRequest()
	assert.Empty(t, Request(r).Target())

	r.Header.Set("HX-Target", "content")
	assert.Equal(t, "content", Request(r).Target())
}

func TestRequest_Trigger(t *testing.T) {
	r := newGetRequest()
	assert.Empty(t, Request(r).Trigger())

	r.Header.Set("HX-Trigger", "btn-save")
	assert.Equal(t, "btn-save", Request(r).Trigger())
}

func TestRequest_TriggerName(t *testing.T) {
	r := newGetRequest()
	assert.Empty(t, Request(r).TriggerName())

	r.Header.Set("HX-Trigger-Name", "save")
	assert.Equal(t, "save", Request(r).TriggerName())
}

func TestRequest_Prompt(t *testing.T) {
	r := newGetRequest()
	assert.Empty(t, Request(r).Prompt())

	r.Header.Set("HX-Prompt", "yes")
	assert.Equal(t, "yes", Request(r).Prompt())
}

func TestRequest_CurrentURL(t *testing.T) {
	r := newGetRequest()
	assert.Empty(t, Request(r).CurrentURL())

	r.Header.Set("HX-Current-URL", "http://example.com/page")
	assert.Equal(t, "http://example.com/page", Request(r).CurrentURL())
}

func TestRequest_HistoryRestore(t *testing.T) {
	r := newGetRequest()
	assert.False(t, Request(r).HistoryRestore())

	r.Header.Set("HX-History-Restore-Request", "true")
	assert.True(t, Request(r).HistoryRestore())
}

func TestRedirect(t *testing.T) {
	w := httptest.NewRecorder()
	Redirect(w, "/admin/users")
	assert.Equal(t, "/admin/users", w.Header().Get("HX-Redirect"))
}

func TestRefresh(t *testing.T) {
	w := httptest.NewRecorder()
	Refresh(w)
	assert.Equal(t, "true", w.Header().Get("HX-Refresh"))
}

func TestTrigger(t *testing.T) {
	w := httptest.NewRecorder()
	Trigger(w, "showMessage")
	assert.Equal(t, "showMessage", w.Header().Get("HX-Trigger"))
}

func TestTriggerAfterSettle(t *testing.T) {
	w := httptest.NewRecorder()
	TriggerAfterSettle(w, "settled")
	assert.Equal(t, "settled", w.Header().Get("HX-Trigger-After-Settle"))
}

func TestTriggerAfterSwap(t *testing.T) {
	w := httptest.NewRecorder()
	TriggerAfterSwap(w, "swapped")
	assert.Equal(t, "swapped", w.Header().Get("HX-Trigger-After-Swap"))
}

func TestPushURL(t *testing.T) {
	w := httptest.NewRecorder()
	PushURL(w, "/new-url")
	assert.Equal(t, "/new-url", w.Header().Get("HX-Push-Url"))
}

func TestReplaceURL(t *testing.T) {
	w := httptest.NewRecorder()
	ReplaceURL(w, "/replaced")
	assert.Equal(t, "/replaced", w.Header().Get("HX-Replace-Url"))
}

func TestReswap(t *testing.T) {
	w := httptest.NewRecorder()
	Reswap(w, "outerHTML")
	assert.Equal(t, "outerHTML", w.Header().Get("HX-Reswap"))
}

func TestRetarget(t *testing.T) {
	w := httptest.NewRecorder()
	Retarget(w, "#other")
	assert.Equal(t, "#other", w.Header().Get("HX-Retarget"))
}

func TestLocation(t *testing.T) {
	w := httptest.NewRecorder()
	Location(w, "/somewhere")
	assert.Equal(t, "/somewhere", w.Header().Get("HX-Location"))
}
