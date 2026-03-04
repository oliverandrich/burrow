package templates

import (
	"fmt"
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/i18n"
	"codeberg.org/oliverandrich/burrow/contrib/jobs"
	"github.com/a-h/templ"
)

// DefaultAdminRenderer returns an AdminRenderer that uses the built-in Templ
// templates for admin pages (job list, job detail).
func DefaultAdminRenderer() jobs.AdminRenderer {
	return &defaultAdminRenderer{}
}

type defaultAdminRenderer struct{}

func (d *defaultAdminRenderer) AdminJobsListPage(w http.ResponseWriter, r *http.Request, j []jobs.Job, page burrow.PageResult, activeStatus string) error {
	return renderWithLayout(w, r, i18n.T(r.Context(), "admin-jobs-title"), adminJobsListPage(j, page, activeStatus))
}

func (d *defaultAdminRenderer) AdminJobDetailPage(w http.ResponseWriter, r *http.Request, job *jobs.Job) error {
	title := i18n.T(r.Context(), "admin-job-detail-title") + fmt.Sprintf("%d", job.ID)
	return renderWithLayout(w, r, title, adminJobDetailPage(job))
}

// renderWithLayout wraps content in the layout from context, or renders bare content.
func renderWithLayout(w http.ResponseWriter, r *http.Request, title string, content templ.Component) error {
	layout := burrow.Layout(r.Context())
	if layout != nil {
		return burrow.Render(w, r, http.StatusOK, layout(title, content))
	}
	return burrow.Render(w, r, http.StatusOK, content)
}

// itoa converts an int64 to a string.
func itoa(id int64) string {
	return fmt.Sprintf("%d", id)
}
