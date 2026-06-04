package jobs

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/htmx"
)

// adminListJobs handles GET /admin/jobs — paginated job list with status filter.
func (a *App) adminListJobs(w http.ResponseWriter, r *http.Request) error {
	pr := burrow.ParsePageRequest(r)
	status := JobStatus(r.URL.Query().Get("status"))

	jobs, page, err := a.repo.ListPaged(r.Context(), pr, status)
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list jobs")
	}

	workerStatus, err := a.workerStatus(r.Context())
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to load worker status")
	}

	return burrow.Render(w, r, http.StatusOK, "jobs/admin_list", map[string]any{
		"Jobs":     jobs,
		"Page":     page,
		"Status":   string(status),
		"RawQuery": r.URL.RawQuery,
		"Worker":   workerStatus,
	})
}

// adminJobDetail handles GET /admin/jobs/{id} — job detail page.
func (a *App) adminJobDetail(w http.ResponseWriter, r *http.Request) error {
	id, err := parseJobID(r)
	if err != nil {
		return err
	}

	job, err := a.repo.FindByID(r.Context(), id)
	if err != nil {
		return mapRepoError(err)
	}

	return burrow.Render(w, r, http.StatusOK, "jobs/admin_detail", map[string]any{
		"Job": job,
	})
}

// adminDeleteJob handles DELETE /admin/jobs/{id} — delete a job.
func (a *App) adminDeleteJob(w http.ResponseWriter, r *http.Request) error {
	id, err := parseJobID(r)
	if err != nil {
		return err
	}

	if err := a.repo.Delete(r.Context(), id); err != nil {
		return mapRepoError(err)
	}

	htmx.SmartRedirect(w, r, "/admin/jobs")
	return nil
}

// retryHandler returns a HandlerFunc that retries a dead/failed job.
func retryHandler(repo *Repository) burrow.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := parseJobID(r)
		if err != nil {
			return err
		}
		if err := repo.Retry(r.Context(), id); err != nil {
			return mapRepoError(err)
		}
		htmx.SmartRedirect(w, r, "/admin/jobs/"+id)
		return nil
	}
}

// cancelHandler returns a HandlerFunc that cancels a pending/running/failed job.
func cancelHandler(repo *Repository) burrow.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := parseJobID(r)
		if err != nil {
			return err
		}
		if err := repo.Cancel(r.Context(), id); err != nil {
			return mapRepoError(err)
		}
		htmx.SmartRedirect(w, r, "/admin/jobs/"+id)
		return nil
	}
}

// isRetryable returns true if a job can be retried.
func isRetryable(item any) bool {
	if j, ok := item.(Job); ok {
		return j.Status == StatusFailed || j.Status == StatusDead
	}
	return false
}

// isCancellable returns true if a job can be cancelled.
func isCancellable(item any) bool {
	if j, ok := item.(Job); ok {
		return j.Status == StatusPending || j.Status == StatusRunning || j.Status == StatusFailed
	}
	return false
}

// parseJobID extracts the job ID from the URL parameter.
func parseJobID(r *http.Request) (string, error) {
	id := chi.URLParam(r, "id")
	if id == "" {
		return "", burrow.NewHTTPError(http.StatusBadRequest, "missing job id")
	}
	return id, nil
}

// mapRepoError converts repository errors to appropriate HTTP errors.
func mapRepoError(err error) error {
	switch {
	case isNotFound(err):
		return burrow.NewHTTPError(http.StatusNotFound, "job not found")
	case isInvalidStatus(err):
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid status for this operation")
	default:
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to process job")
	}
}

func isNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func isInvalidStatus(err error) bool {
	return errors.Is(err, ErrInvalidStatus)
}
