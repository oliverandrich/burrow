// Package polls implements a polls application for the burrow tutorial.
// Step 4 adds voting with CSRF protection and flash messages.
package polls

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/uptrace/bun"
)

// --------------------------------------------------------------------------
// Models
// --------------------------------------------------------------------------

type Question struct {
	bun.BaseModel `bun:"table:questions,alias:q"`
	PublishedAt   time.Time `bun:",notnull,default:current_timestamp"`
	Text          string    `bun:",notnull"`
	Choices       []Choice  `bun:"rel:has-many,join:id=question_id"`
	ID            int64     `bun:",pk,autoincrement"`
}

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

func (r *Repository) CreateQuestion(ctx context.Context, q *Question) error {
	_, err := r.db.NewInsert().Model(q).Exec(ctx)
	return err
}

func (r *Repository) CreateChoice(ctx context.Context, c *Choice) error {
	_, err := r.db.NewInsert().Model(c).Exec(ctx)
	return err
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
	return burrow.Render(w, r, http.StatusOK, "polls/list", map[string]any{
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
	return burrow.Render(w, r, http.StatusOK, "polls/detail", map[string]any{
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
	return burrow.Render(w, r, http.StatusOK, "polls/results", map[string]any{
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
	repo     *Repository
	handlers *Handlers
}

func New() *App { return &App{} }

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

func (a *App) NavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{Label: "Polls", URL: "/polls", Position: 10},
	}
}

func (a *App) Routes(r chi.Router) {
	r.Route("/polls", func(r chi.Router) {
		r.Get("/", burrow.Handle(a.handlers.List))
		r.Get("/{id}", burrow.Handle(a.handlers.Detail))
		r.Post("/{id}/vote", burrow.Handle(a.handlers.Vote))
		r.Get("/{id}/results", burrow.Handle(a.handlers.Results))
	})
}
