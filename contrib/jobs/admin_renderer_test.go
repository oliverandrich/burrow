package jobs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/admin/modeladmin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJobsRenderer(t *testing.T) {
	r := newJobsRenderer()
	require.NotNil(t, r)

	jr, ok := r.(*jobsRenderer)
	require.True(t, ok)
	assert.NotNil(t, jr.base)
}

func TestJobsRenderer_ConfirmDelete(t *testing.T) {
	r := newJobsRenderer()
	err := r.ConfirmDelete(nil, nil, &Job{}, modeladmin.RenderConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "confirm delete not supported")
}

func TestJobsRenderer_Detail_NoTemplateExecutor(t *testing.T) {
	// Detail calls burrow.RenderTemplate which requires a template executor in context.
	// Without one it should return an error.
	r := newJobsRenderer()
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/jobs/1", nil)
	err := r.Detail(w, req, &Job{ID: 1, Type: "test"}, modeladmin.RenderConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no template executor")
}

func TestJobsRenderer_List_Delegates(t *testing.T) {
	// List delegates to base renderer. We just verify it doesn't panic
	// and the base renderer is called (it succeeds with empty data).
	r := newJobsRenderer()
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/jobs", nil)
	_ = r.List(w, req, []Job{}, burrow.PageResult{}, modeladmin.RenderConfig{})
}

func TestJobsRenderer_Form_Delegates(t *testing.T) {
	r := newJobsRenderer()
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/jobs/1/edit", nil)
	_ = r.Form(w, req, &Job{}, nil, nil, modeladmin.RenderConfig{})
}
