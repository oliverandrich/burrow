// Package polls implements a polls application for the burrow tutorial.
package polls

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/oliverandrich/burrow"
	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"
)

// --------------------------------------------------------------------------
// Models
// --------------------------------------------------------------------------

// Question represents a poll question.
type Question struct {
	bun.BaseModel `bun:"table:questions,alias:q"`
	PublishedAt   time.Time `bun:",notnull,default:current_timestamp"`
	Text          string    `bun:",notnull"`
	Choices       []Choice  `bun:"rel:has-many,join:id=question_id"`
	ID            int64     `bun:",pk,autoincrement"`
}

// Choice represents a possible answer to a question.
type Choice struct {
	bun.BaseModel `bun:"table:choices,alias:c"`
	Question      *Question `bun:"rel:belongs-to,join:question_id=id"`
	Text          string    `bun:",notnull"`
	ID            int64     `bun:",pk,autoincrement"`
	QuestionID    int64     `bun:",notnull"`
	Votes         int       `bun:",notnull,default:0"`
}

// --------------------------------------------------------------------------
// Repository
// --------------------------------------------------------------------------

// Repository provides database access for polls.
type Repository struct {
	db *bun.DB
}

// NewRepository creates a new polls repository.
func NewRepository(db *bun.DB) *Repository {
	return &Repository{db: db}
}

// ListQuestions returns all questions ordered by publication date.
func (r *Repository) ListQuestions(ctx context.Context) ([]Question, error) {
	var questions []Question
	err := r.db.NewSelect().
		Model(&questions).
		Order("published_at DESC", "id DESC").
		Scan(ctx)
	return questions, err
}

// GetQuestion returns a single question with its choices.
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

// CreateQuestion inserts a new question.
func (r *Repository) CreateQuestion(ctx context.Context, q *Question) error {
	_, err := r.db.NewInsert().Model(q).Exec(ctx)
	return err
}

// CreateChoice inserts a new choice for a question.
func (r *Repository) CreateChoice(ctx context.Context, c *Choice) error {
	_, err := r.db.NewInsert().Model(c).Exec(ctx)
	return err
}

// IncrementVotes adds one vote to the given choice.
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

// Handlers contains the HTTP handlers for the polls app.
type Handlers struct {
	repo *Repository
}

// List renders all questions.
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

// Detail renders a single question with its choices.
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

// Results renders the voting results for a question.
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

// App is the polls burrow application.
type App struct {
	repo     *Repository
	handlers *Handlers
}

// New creates a new polls app.
func New() *App {
	return &App{}
}

func (a *App) Name() string { return "polls" }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.repo = NewRepository(cfg.DB)
	a.handlers = &Handlers{repo: a.repo}
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

func (a *App) FuncMap() template.FuncMap {
	return template.FuncMap{
		"itoa": strconv.Itoa,
	}
}

func (a *App) NavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{Label: "Polls", URL: "/polls", Position: 10},
	}
}

func (a *App) Routes(r chi.Router) {
	r.Route("/polls", func(r chi.Router) {
		r.Get("/", burrow.Handle(a.handlers.List))
		r.Get("/{id}", burrow.Handle(a.handlers.Detail))
		r.Get("/{id}/results", burrow.Handle(a.handlers.Results))
	})
}
