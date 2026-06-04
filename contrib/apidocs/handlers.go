package apidocs

import (
	"net/http"

	"github.com/oliverandrich/burrow"
)

// handlePage renders the standalone documentation page. It uses RenderFragment
// (not Render) on purpose: the Scalar page is a complete HTML document and must
// not be wrapped in the application layout.
func (a *App) handlePage(w http.ResponseWriter, r *http.Request) error {
	html, err := burrow.RenderFragment(r.Context(), "apidocs/index", map[string]any{
		"SpecURL": a.specURL,
		"Title":   a.title,
	})
	if err != nil {
		return err
	}
	return burrow.HTML(w, http.StatusOK, string(html))
}
