package jobs

import (
	"errors"
	"net/http"
	"strconv"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/admin/modeladmin"
	"github.com/go-chi/chi/v5"
)

// statusChoices returns filter choices for all job statuses.
func statusChoices() []modeladmin.Choice {
	return []modeladmin.Choice{
		{Value: string(StatusPending), Label: "Pending", LabelKey: "admin-jobs-filter-pending"},
		{Value: string(StatusRunning), Label: "Running", LabelKey: "admin-jobs-filter-running"},
		{Value: string(StatusFailed), Label: "Failed", LabelKey: "admin-jobs-filter-failed"},
		{Value: string(StatusDead), Label: "Dead", LabelKey: "admin-jobs-filter-dead"},
		{Value: string(StatusCompleted), Label: "Completed", LabelKey: "admin-jobs-filter-completed"},
	}
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
		w.Header().Set("HX-Redirect", "/admin/jobs/"+strconv.FormatInt(id, 10))
		w.WriteHeader(http.StatusOK)
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
		w.Header().Set("HX-Redirect", "/admin/jobs/"+strconv.FormatInt(id, 10))
		w.WriteHeader(http.StatusOK)
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
func parseJobID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return 0, burrow.NewHTTPError(http.StatusBadRequest, "invalid job id")
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
