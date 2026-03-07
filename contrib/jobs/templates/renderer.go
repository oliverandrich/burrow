package templates

import (
	"fmt"
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/i18n"
	"codeberg.org/oliverandrich/burrow/contrib/jobs"
)

// DefaultAdminRenderer returns an AdminRenderer that uses the built-in HTML
// templates for admin pages (job list, job detail).
func DefaultAdminRenderer() jobs.AdminRenderer {
	return &defaultAdminRenderer{}
}

type defaultAdminRenderer struct{}

func (d *defaultAdminRenderer) AdminJobsListPage(w http.ResponseWriter, r *http.Request, j []jobs.Job, page burrow.PageResult, activeStatus string) error {
	return burrow.RenderTemplate(w, r, http.StatusOK, "jobs/admin_list", map[string]any{
		"Title":        i18n.T(r.Context(), "admin-jobs-title"),
		"Jobs":         j,
		"Page":         page,
		"ActiveStatus": activeStatus,
	})
}

func (d *defaultAdminRenderer) AdminJobDetailPage(w http.ResponseWriter, r *http.Request, job *jobs.Job) error {
	return burrow.RenderTemplate(w, r, http.StatusOK, "jobs/admin_detail", map[string]any{
		"Title": i18n.T(r.Context(), "admin-job-detail-title") + fmt.Sprintf("%d", job.ID),
		"Job":   job,
	})
}
