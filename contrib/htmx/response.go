package htmx

import (
	"net/http"

	"github.com/oliverandrich/burrow"
)

// StatusStopPolling is the HTTP status code (286) that instructs htmx to stop polling.
const StatusStopPolling = 286

// Reselect sets the HX-Reselect header to change the content selection
// for the current response.
func Reselect(w http.ResponseWriter, selector string) {
	w.Header().Set("HX-Reselect", selector)
}

// SmartRedirect issues an HX-Redirect for htmx requests or a standard
// 303 redirect for normal requests. For htmx requests it also writes a
// 200 status so the browser processes the HX-Redirect header.
func SmartRedirect(w http.ResponseWriter, r *http.Request, url string) {
	if Request(r).IsHTMX() {
		Redirect(w, url)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// RenderOrRedirect renders a template fragment for htmx requests,
// or issues a 303 redirect for standard requests.
func RenderOrRedirect(w http.ResponseWriter, r *http.Request, redirectURL string, templateName string, data map[string]any) error {
	if Request(r).IsHTMX() {
		return burrow.Render(w, r, http.StatusOK, templateName, data)
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	return nil
}
