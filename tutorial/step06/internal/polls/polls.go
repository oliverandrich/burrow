// Package polls implements a polls application for the burrow tutorial.
// Step 6 adds an admin panel with ModelAdmin for questions.
package polls

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/admin/modeladmin"
	matpl "codeberg.org/oliverandrich/burrow/contrib/admin/modeladmin/templates"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"codeberg.org/oliverandrich/burrow/contrib/messages"
	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"
)

// --------------------------------------------------------------------------
// Models
// --------------------------------------------------------------------------

type Question struct {
	bun.BaseModel `bun:"table:questions,alias:q"`
	PublishedAt   time.Time `bun:",notnull,default:current_timestamp" verbose:"Published"`
	Text          string    `bun:",notnull" verbose:"Question"`
	Choices       []Choice  `bun:"rel:has-many,join:id=question_id"`
	ID            int64     `bun:",pk,autoincrement" verbose:"ID"`
}

type Choice struct {
	bun.BaseModel `bun:"table:choices,alias:c"`
	Question      *Question `bun:"rel:belongs-to,join:question_id=id"`
	Text          string    `bun:",notnull" verbose:"Choice"`
	ID            int64     `bun:",pk,autoincrement" verbose:"ID"`
	QuestionID    int64     `bun:",notnull" verbose:"Question"`
	Votes         int       `bun:",notnull,default:0" verbose:"Votes"`
}

// --------------------------------------------------------------------------
// Repository
// --------------------------------------------------------------------------

type Repository struct {
	db *bun.DB
}

func NewRepository(db *bun.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListQuestions(ctx context.Context) ([]Question, error) {
	var questions []Question
	err := r.db.NewSelect().
		Model(&questions).
		Order("published_at DESC", "id DESC").
		Scan(ctx)
	return questions, err
}

func (r *Repository) GetQuestion(ctx context.Context, id int64) (*Question, error) {
	question := new(Question)
	err := r.db.NewSelect().
		Model(question).
		Relation("Choices").
		Where("q.id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return question, nil
}

func (r *Repository) IncrementVotes(ctx context.Context, choiceID int64) error {
	_, err := r.db.NewUpdate().
		Model((*Choice)(nil)).
		Set("votes = votes + 1").
		Where("id = ?", choiceID).
		Exec(ctx)
	return err
}

// --------------------------------------------------------------------------
// Handlers
// --------------------------------------------------------------------------

type Handlers struct {
	repo *Repository
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
	questions, err := h.repo.ListQuestions(r.Context())
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list questions")
	}
	return burrow.RenderTemplate(w, r, http.StatusOK, "polls/list", map[string]any{
		"Title":     "Polls",
		"Questions": questions,
	})
}

func (h *Handlers) Detail(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid question ID")
	}
	question, err := h.repo.GetQuestion(r.Context(), id)
	if err != nil {
		return burrow.NewHTTPError(http.StatusNotFound, "question not found")
	}
	return burrow.RenderTemplate(w, r, http.StatusOK, "polls/detail", map[string]any{
		"Title":    question.Text,
		"Question": question,
	})
}

func (h *Handlers) Vote(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	questionID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid question ID")
	}

	choiceIDStr := r.FormValue("choice")
	if choiceIDStr == "" {
		if addErr := messages.AddError(w, r, "You didn't select a choice."); addErr != nil {
			return addErr
		}
		http.Redirect(w, r, fmt.Sprintf("/polls/%d", questionID), http.StatusSeeOther)
		return nil
	}

	choiceID, err := strconv.ParseInt(choiceIDStr, 10, 64)
	if err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid choice ID")
	}

	if err := h.repo.IncrementVotes(r.Context(), choiceID); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to record vote")
	}

	if err := messages.AddSuccess(w, r, "Your vote has been recorded!"); err != nil {
		return err
	}
	http.Redirect(w, r, fmt.Sprintf("/polls/%d/results", questionID), http.StatusSeeOther)
	return nil
}

func (h *Handlers) Results(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid question ID")
	}
	question, err := h.repo.GetQuestion(r.Context(), id)
	if err != nil {
		return burrow.NewHTTPError(http.StatusNotFound, "question not found")
	}
	return burrow.RenderTemplate(w, r, http.StatusOK, "polls/results", map[string]any{
		"Title":    fmt.Sprintf("Results: %s", question.Text),
		"Question": question,
	})
}

// --------------------------------------------------------------------------
// App
// --------------------------------------------------------------------------

//go:embed migrations
var migrationFS embed.FS

//go:embed templates
var templateFS embed.FS

type App struct {
	repo           *Repository
	handlers       *Handlers
	questionsAdmin *modeladmin.ModelAdmin[Question]
}

func New() *App { return &App{} }

func (a *App) Name() string { return "polls" }

func (a *App) Dependencies() []string { return []string{"auth"} }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.repo = NewRepository(cfg.DB)
	a.handlers = &Handlers{repo: a.repo}

	a.questionsAdmin = &modeladmin.ModelAdmin[Question]{
		Slug:              "questions",
		DisplayName:       "Question",
		DisplayPluralName: "Questions",
		DB:                cfg.DB,
		Renderer:          matpl.DefaultRenderer[Question](),
		CanCreate:         true,
		CanEdit:           true,
		CanDelete:         true,
		ListFields:        []string{"ID", "Text", "PublishedAt"},
		OrderBy:           "published_at DESC, id DESC",
	}

	return nil
}

func (a *App) MigrationFS() fs.FS {
	sub, _ := fs.Sub(migrationFS, "migrations")
	return sub
}

func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return sub
}

func (a *App) NavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{Label: "Polls", URL: "/polls", Position: 10},
	}
}

func (a *App) AdminNavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{Label: "Questions", URL: "/admin/questions", Position: 30},
	}
}

func (a *App) Routes(r chi.Router) {
	r.Route("/polls", func(r chi.Router) {
		r.Get("/", burrow.Handle(a.handlers.List))
		r.Get("/{id}", burrow.Handle(a.handlers.Detail))
		r.Get("/{id}/results", burrow.Handle(a.handlers.Results))

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth())
			r.Post("/{id}/vote", burrow.Handle(a.handlers.Vote))
		})
	})
}

func (a *App) AdminRoutes(r chi.Router) {
	r.Route("/questions", func(r chi.Router) {
		r.Get("/", burrow.Handle(a.questionsAdmin.HandleList))
		r.Get("/new", burrow.Handle(a.questionsAdmin.HandleNew))
		r.Post("/new", burrow.Handle(a.questionsAdmin.HandleNew))
		r.Get("/{id}", burrow.Handle(a.questionsAdmin.HandleDetail))
		r.Post("/{id}", burrow.Handle(a.questionsAdmin.HandleDetail))
		r.Get("/{id}/delete", burrow.Handle(a.questionsAdmin.HandleDelete))
		r.Post("/{id}/delete", burrow.Handle(a.questionsAdmin.HandleDelete))
	})
}
