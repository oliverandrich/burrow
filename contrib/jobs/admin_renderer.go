package jobs

import (
	"fmt"
	"net/http"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/admin/modeladmin"
	matpl "github.com/oliverandrich/burrow/contrib/admin/modeladmin/templates"
	"github.com/oliverandrich/burrow/forms"
)

// newJobsRenderer returns a ModelAdmin renderer for Jobs that delegates
// list/form to the default ModelAdmin renderer but uses
// a custom detail template with Payload and LastError cards.
func newJobsRenderer() modeladmin.Renderer[Job] {
	return &jobsRenderer{
		base: matpl.DefaultRenderer[Job](),
	}
}

type jobsRenderer struct {
	base modeladmin.Renderer[Job]
}

func (r *jobsRenderer) List(w http.ResponseWriter, req *http.Request, items []Job, page burrow.PageResult, cfg modeladmin.RenderConfig) error {
	return r.base.List(w, req, items, page, cfg)
}

func (r *jobsRenderer) Detail(w http.ResponseWriter, req *http.Request, item *Job, cfg modeladmin.RenderConfig) error {
	return burrow.RenderTemplate(w, req, http.StatusOK, "jobs/admin_detail", map[string]any{
		"Job": item,
		"Cfg": cfg,
	})
}

func (r *jobsRenderer) Form(w http.ResponseWriter, req *http.Request, item *Job, fields []forms.BoundField, cfg modeladmin.RenderConfig) error {
	return r.base.Form(w, req, item, fields, cfg)
}

func (r *jobsRenderer) ConfirmDelete(_ http.ResponseWriter, _ *http.Request, _ *Job, _ modeladmin.RenderConfig) error {
	return fmt.Errorf("confirm delete not supported for jobs")
}
