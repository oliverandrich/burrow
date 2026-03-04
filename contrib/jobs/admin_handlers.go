package jobs

import (
	"errors"
	"net/http"
	"strconv"

	"codeberg.org/oliverandrich/burrow"
	"github.com/go-chi/chi/v5"
)

// AdminRenderer defines how admin job pages are rendered.
// Projects implement this to provide their own template rendering
// for the admin area.
type AdminRenderer interface {
	AdminJobsListPage(w http.ResponseWriter, r *http.Request, jobs []Job, page burrow.PageResult, activeStatus string) error
	AdminJobDetailPage(w http.ResponseWriter, r *http.Request, job *Job) error
}

// adminHandlers holds the admin HTTP handlers for job management.
type adminHandlers struct {
	repo     *Repository
	renderer AdminRenderer
}

// newAdminHandlers creates admin handlers with the given repo and renderer.
func newAdminHandlers(repo *Repository, renderer AdminRenderer) *adminHandlers {
	return &adminHandlers{repo: repo, renderer: renderer}
}

// ListPage renders the paginated admin job list with optional status filter.
func (h *adminHandlers) ListPage(w http.ResponseWriter, r *http.Request) error {
	pr := burrow.ParsePageRequest(r)
	statusFilter := JobStatus(r.URL.Query().Get("status"))

	jobs, page, err := h.repo.ListPaged(r.Context(), pr, statusFilter)
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list jobs")
	}

	return h.renderer.AdminJobsListPage(w, r, jobs, page, string(statusFilter))
}

// DetailPage renders a single job's detail page.
func (h *adminHandlers) DetailPage(w http.ResponseWriter, r *http.Request) error {
	id, err := parseJobID(r)
	if err != nil {
		return err
	}

	job, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		return mapRepoError(err)
	}

	return h.renderer.AdminJobDetailPage(w, r, job)
}

// DeleteJob hard-deletes a job and redirects back to the list.
func (h *adminHandlers) DeleteJob(w http.ResponseWriter, r *http.Request) error {
	id, err := parseJobID(r)
	if err != nil {
		return err
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		return mapRepoError(err)
	}

	w.Header().Set("HX-Redirect", "/admin/jobs")
	w.WriteHeader(http.StatusOK)
	return nil
}

// RetryJob resets a dead/failed job to pending.
func (h *adminHandlers) RetryJob(w http.ResponseWriter, r *http.Request) error {
	id, err := parseJobID(r)
	if err != nil {
		return err
	}

	if err := h.repo.Retry(r.Context(), id); err != nil {
		return mapRepoError(err)
	}

	w.Header().Set("HX-Redirect", "/admin/jobs/"+strconv.FormatInt(id, 10))
	w.WriteHeader(http.StatusOK)
	return nil
}

// CancelJob marks a pending/running/failed job as dead.
func (h *adminHandlers) CancelJob(w http.ResponseWriter, r *http.Request) error {
	id, err := parseJobID(r)
	if err != nil {
		return err
	}

	if err := h.repo.Cancel(r.Context(), id); err != nil {
		return mapRepoError(err)
	}

	w.Header().Set("HX-Redirect", "/admin/jobs/"+strconv.FormatInt(id, 10))
	w.WriteHeader(http.StatusOK)
	return nil
}

// parseJobID extracts the job ID from the URL parameter.
func parseJobID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return 0, burrow.NewHTTPError(http.StatusBadRequest, "invalid job id")
	}
	return id, nil
}

// mapRepoError converts repository errors to appropriate HTTP errors.
func mapRepoError(err error) error {
	if errors.Is(err, ErrNotFound) {
		return burrow.NewHTTPError(http.StatusNotFound, "job not found")
	}
	if errors.Is(err, ErrInvalidStatus) {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid status for this operation")
	}
	return burrow.NewHTTPError(http.StatusInternalServerError, "failed to process job")
}
