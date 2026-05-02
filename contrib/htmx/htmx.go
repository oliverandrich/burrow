// Package htmx provides request detection and response helpers for htmx,
// inspired by django-htmx. It also serves the htmx JavaScript file as a
// static asset via the burrow staticfiles contrib app.
package htmx

import (
	"encoding/json"
	"net/http"
)

// HtmxRequest holds parsed htmx request headers.
type HtmxRequest struct {
	r *http.Request
}

// Request parses htmx-specific headers from the HTTP request.
func Request(r *http.Request) HtmxRequest {
	return HtmxRequest{r: r}
}

// IsHTMX returns true if the request was made by htmx.
func (h HtmxRequest) IsHTMX() bool { return h.r.Header.Get("HX-Request") == "true" }

// IsBoosted returns true if the request is via an hx-boost element.
func (h HtmxRequest) IsBoosted() bool { return h.r.Header.Get("HX-Boosted") == "true" }

// Target returns the id of the target element if it exists.
func (h HtmxRequest) Target() string { return h.r.Header.Get("HX-Target") }

// Trigger returns the id of the triggered element if it exists.
func (h HtmxRequest) Trigger() string { return h.r.Header.Get("HX-Trigger") }

// TriggerName returns the name of the triggered element if it exists.
func (h HtmxRequest) TriggerName() string { return h.r.Header.Get("HX-Trigger-Name") }

// Prompt returns the user response to an hx-prompt.
func (h HtmxRequest) Prompt() string { return h.r.Header.Get("HX-Prompt") }

// CurrentURL returns the current URL of the browser.
func (h HtmxRequest) CurrentURL() string { return h.r.Header.Get("HX-Current-URL") }

// HistoryRestore returns true if the request is for history restoration
// after a miss in the local history cache.
func (h HtmxRequest) HistoryRestore() bool {
	return h.r.Header.Get("HX-History-Restore-Request") == "true"
}

// --- Response helpers ---

// Redirect sets the HX-Redirect header, causing htmx to do a client-side redirect.
func Redirect(w http.ResponseWriter, url string) {
	w.Header().Set("HX-Redirect", url)
}

// Refresh sets the HX-Refresh header, causing a full page refresh.
func Refresh(w http.ResponseWriter) {
	w.Header().Set("HX-Refresh", "true")
}

// Trigger sets the HX-Trigger header to trigger a client-side event.
func Trigger(w http.ResponseWriter, event string) {
	w.Header().Set("HX-Trigger", event)
}

// TriggerAfterSettle sets the HX-Trigger-After-Settle header.
func TriggerAfterSettle(w http.ResponseWriter, event string) {
	w.Header().Set("HX-Trigger-After-Settle", event)
}

// TriggerAfterSwap sets the HX-Trigger-After-Swap header.
func TriggerAfterSwap(w http.ResponseWriter, event string) {
	w.Header().Set("HX-Trigger-After-Swap", event)
}

// PushURL sets the HX-Push-Url header to push a new URL into the history stack.
func PushURL(w http.ResponseWriter, url string) {
	w.Header().Set("HX-Push-Url", url)
}

// ReplaceURL sets the HX-Replace-Url header to replace the current URL.
func ReplaceURL(w http.ResponseWriter, url string) {
	w.Header().Set("HX-Replace-Url", url)
}

// Reswap sets the HX-Reswap header to override the swap strategy.
func Reswap(w http.ResponseWriter, strategy string) {
	w.Header().Set("HX-Reswap", strategy)
}

// Retarget sets the HX-Retarget header to change the target element.
func Retarget(w http.ResponseWriter, selector string) {
	w.Header().Set("HX-Retarget", selector)
}

// Location sets the HX-Location header for client-side navigation
// without a full page reload.
func Location(w http.ResponseWriter, url string) {
	w.Header().Set("HX-Location", url)
}

// OpenDialog tells the client to open the <dialog> element with the given id
// once the response has been swapped into the DOM. Pairs with the
// htmx/dialog_script template, which the standard mucss and bootstrap nav
// layouts include automatically; if you build a custom layout, include the
// template yourself.
//
// An optional dialogClass replaces the dialog element's className before the
// dialog is opened — useful for size variants like µCSS's "modal-lg" /
// "modal-sm" / "modal-fullscreen", which target the dialog itself rather
// than its inner content. Passing the empty string clears any previously
// applied class.
//
//	htmx.OpenDialog(w, "modal")
//	htmx.OpenDialog(w, "modal", "modal-lg")
//
// Sets HX-Trigger-After-Swap to {"openDialog":"<id>"} or
// {"openDialog":{"id":"<id>","class":"<class>"}} when a class is given.
func OpenDialog(w http.ResponseWriter, id string, dialogClass ...string) {
	if len(dialogClass) == 0 {
		setDialogTrigger(w, "openDialog", id)
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"openDialog": map[string]string{"id": id, "class": dialogClass[0]},
	})
	w.Header().Set("HX-Trigger-After-Swap", string(payload))
}

// CloseDialog tells the client to close the <dialog> element with the given
// id. See [OpenDialog] for the script-include requirement.
//
//	htmx.CloseDialog(w, "modal")
//
// Sets HX-Trigger-After-Swap to {"closeDialog":"<id>"}.
func CloseDialog(w http.ResponseWriter, id string) {
	setDialogTrigger(w, "closeDialog", id)
}

func setDialogTrigger(w http.ResponseWriter, event, id string) {
	payload, _ := json.Marshal(map[string]string{event: id})
	w.Header().Set("HX-Trigger-After-Swap", string(payload))
}
