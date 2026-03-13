// Package notes is an example custom app demonstrating the burrow framework.
package notes

import (
	"embed"
	"html/template"
	"io/fs"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/admin/modeladmin"
	matpl "github.com/oliverandrich/burrow/contrib/admin/modeladmin/templates"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/burrow/contrib/bsicons"
)

//go:embed migrations
var migrationFS embed.FS

//go:embed translations
var translationFS embed.FS

//go:embed templates
var noteTemplateFS embed.FS

// App implements the notes contrib app.
type App struct {
	repo       *Repository
	handlers   *Handlers
	notesAdmin *modeladmin.ModelAdmin[Note]
}

// New creates a new notes app.
func New() *App {
	return &App{}
}

func (a *App) Name() string { return "notes" }

func (a *App) Dependencies() []string { return []string{"auth"} }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.repo = NewRepository(cfg.DB)
	a.handlers = NewHandlers(a.repo)
	a.notesAdmin = &modeladmin.ModelAdmin[Note]{
		Slug:              "notes",
		DisplayName:       "Note",
		DisplayPluralName: "Notes",
		EmptyMessageKey:   "admin-notes-empty",
		DB:                cfg.DB,
		Renderer:          matpl.DefaultRenderer[Note](),
		CanCreate:         true,
		CanEdit:           true,
		CanDelete:         true,
		ListFields:        []string{"ID", "Title", "Content", "User", "CreatedAt"},
		Relations:         []string{"User"},
		ReadOnlyFields:    []string{"User", "CreatedAt"},
		SearchFields:      []string{"title", "content"},
		OrderBy:           "n.created_at DESC, n.id DESC",
	}
	a.notesAdmin.RowActions = []modeladmin.RowAction{
		{
			Slug:    "delete",
			Label:   "modeladmin-delete",
			Icon:    bsicons.Trash(),
			Method:  "DELETE",
			Class:   "btn-outline-danger",
			Confirm: "modeladmin-delete-confirm",
			Handler: a.notesAdmin.HandleDelete,
		},
	}
	return nil
}

func (a *App) TranslationFS() fs.FS { return translationFS }

func (a *App) MigrationFS() fs.FS {
	sub, _ := fs.Sub(migrationFS, "migrations")
	return sub
}

// TemplateFS returns the embedded HTML template files.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(noteTemplateFS, "templates")
	return sub
}

// FuncMap returns template functions used by notes templates.
func (a *App) FuncMap() template.FuncMap {
	return template.FuncMap{
		"iconPlusLg":      func(class ...string) template.HTML { return bsicons.PlusLg(class...) },
		"iconPencil":      func(class ...string) template.HTML { return bsicons.Pencil(class...) },
		"iconJournalText": func(class ...string) template.HTML { return bsicons.JournalText(class...) },
	}
}

func (a *App) NavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{
			Label:    "Notes",
			URL:      "/notes",
			Icon:     bsicons.JournalText(),
			Position: 20,
			AuthOnly: true,
		},
	}
}

func (a *App) AdminRoutes(r chi.Router) {
	if a.notesAdmin == nil {
		return
	}
	a.notesAdmin.Routes(r)
}

func (a *App) AdminNavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{
			Label:     "Notes",
			LabelKey:  "admin-nav-notes",
			URL:       "/admin/notes",
			Icon:      bsicons.JournalText(),
			Position:  30,
			AdminOnly: true,
		},
	}
}

func (a *App) Routes(r chi.Router) {
	if a.handlers == nil {
		return
	}
	h := a.handlers

	r.Route("/notes", func(r chi.Router) {
		r.Use(auth.RequireAuth())
		r.Get("/", burrow.Handle(h.List))
		r.Get("/new", burrow.Handle(h.New))
		r.Post("/", burrow.Handle(h.Create))
		r.Get("/{id}/edit", burrow.Handle(h.Edit))
		r.Post("/{id}", burrow.Handle(h.Update))
		r.Delete("/{id}", burrow.Handle(h.Delete))
	})
}
