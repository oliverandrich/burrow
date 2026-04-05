// Package notes is an example custom app demonstrating the burrow framework.
package notes

import (
	"embed"
	"io/fs"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/admin/modeladmin"
	matpl "github.com/oliverandrich/burrow/contrib/admin/modeladmin/templates"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/burrow/contrib/bsicons"
	"github.com/urfave/cli/v3"
)

//go:embed translations
var translationFS embed.FS

//go:embed templates
var noteTemplateFS embed.FS

// App implements the notes contrib app.
type App struct {
	repo       *Repository
	notesAdmin *modeladmin.ModelAdmin[Note]
}

// New creates a new notes app.
func New() *App {
	return &App{}
}

func (a *App) Name() string { return "notes" }

func (a *App) Dependencies() []string { return []string{"auth"} }

func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
	cfg.RegisterIconFunc("iconPlusLg", bsicons.PlusLg)
	cfg.RegisterIconFunc("iconPencil", bsicons.Pencil)
	cfg.RegisterIconFunc("iconJournalText", bsicons.JournalText)

	a.repo = NewRepository(cfg.DB)

	a.notesAdmin = &modeladmin.ModelAdmin[Note]{
		Slug:              "notes",
		DisplayName:       "Note",
		DisplayPluralName: "Notes",
		DB:                cfg.DB,
		Renderer:          matpl.DefaultRenderer[Note](),
		ListFields:        []string{"Title", "Content", "UserID"},
		CanDelete:         true,
	}
	return nil
}

func (a *App) TranslationFS() fs.FS { return translationFS }

// Documents returns the Den document types registered by this app.
func (a *App) Documents() []any {
	return []any{&Note{}}
}

// TemplateFS returns the embedded HTML template files.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(noteTemplateFS, "templates")
	return sub
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

// AdminRoutes registers admin routes for notes management.
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
	r.Route("/notes", func(r chi.Router) {
		r.Use(auth.RequireAuth())
		r.Get("/", burrow.Handle(a.List))
		r.Get("/new", burrow.Handle(a.New))
		r.Post("/", burrow.Handle(a.Create))
		r.Get("/{id}", burrow.Handle(a.Edit))
		r.Post("/{id}", burrow.Handle(a.Update))
		r.Delete("/{id}", burrow.Handle(a.Delete))
	})
}
