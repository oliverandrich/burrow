// Package polls implements a polls application for the burrow tutorial.
// Step 4 adds voting with CSRF protection and flash messages.
package polls

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"
	"github.com/oliverandrich/den/where"
	"github.com/urfave/cli/v3"
)

// --------------------------------------------------------------------------
// Models
// --------------------------------------------------------------------------

type Question struct {
	document.Base
	PublishedAt time.Time `json:"published_at" den:"index"`
	Text        string    `json:"text"`
	Choices     []Choice  `json:"-"`
}

type Choice struct {
	document.Base
	QuestionID string `json:"question_id" den:"index"`
	Text       string `json:"text"`
	Votes      int    `json:"votes"`
}

// --------------------------------------------------------------------------
// Repository
// --------------------------------------------------------------------------

type Repository struct {
	db *den.DB
}

func NewRepository(db *den.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListQuestions(ctx context.Context) ([]Question, error) {
	ptrs, err := den.NewQuery[Question](r.db).
		Sort("published_at", den.Desc).
		Sort("_id", den.Desc).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list questions: %w", err)
	}
	questions := make([]Question, len(ptrs))
	for i, p := range ptrs {
		questions[i] = *p
	}
	return questions, nil
}

func (r *Repository) GetQuestion(ctx context.Context, id string) (*Question, error) {
	question, err := den.FindByID[Question](ctx, r.db, id)
	if err != nil {
		return nil, fmt.Errorf("get question %s: %w", id, err)
	}
	choicePtrs, err := den.NewQuery[Choice](r.db, where.Field("question_id").Eq(id)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("get choices for question %s: %w", id, err)
	}
	choices := make([]Choice, len(choicePtrs))
	for i, p := range choicePtrs {
		choices[i] = *p
	}
	question.Choices = choices
	return question, nil
}

// CreateQuestion inserts a new question.
func (r *Repository) CreateQuestion(ctx context.Context, q *Question) error {
	return den.Save(ctx, r.db, q)
}

// CreateChoice inserts a new choice for a question.
func (r *Repository) CreateChoice(ctx context.Context, c *Choice) error {
	return den.Save(ctx, r.db, c)
}

func (r *Repository) IncrementVotes(ctx context.Context, choiceID string) error {
	choice, err := den.FindByID[Choice](ctx, r.db, choiceID)
	if err != nil {
		return fmt.Errorf("find choice %s: %w", choiceID, err)
	}
	choice.Votes++
	return den.Save(ctx, r.db, choice)
}

// --------------------------------------------------------------------------
// Handlers
// --------------------------------------------------------------------------

func (a *App) List(w http.ResponseWriter, r *http.Request) error {
	questions, err := a.repo.ListQuestions(r.Context())
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list questions")
	}
	return burrow.Render(w, r, http.StatusOK, "polls/list", map[string]any{
		"Title":     "Polls",
		"Questions": questions,
	})
}

func (a *App) Detail(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	question, err := a.repo.GetQuestion(r.Context(), id)
	if err != nil {
		return burrow.NewHTTPError(http.StatusNotFound, "question not found")
	}
	return burrow.Render(w, r, http.StatusOK, "polls/detail", map[string]any{
		"Title":    question.Text,
		"Question": question,
	})
}

func (a *App) Vote(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	questionID := chi.URLParam(r, "id")

	choiceID := r.FormValue("choice")
	if choiceID == "" {
		if addErr := messages.AddError(w, r, "You didn't select a choice."); addErr != nil {
			return addErr
		}
		http.Redirect(w, r, fmt.Sprintf("/polls/%s", questionID), http.StatusSeeOther) //nolint:gosec // questionID comes from chi URLParam, path-routed and safe
		return nil
	}

	if err := a.repo.IncrementVotes(r.Context(), choiceID); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to record vote")
	}

	if err := messages.AddSuccess(w, r, "Your vote has been recorded!"); err != nil {
		return err
	}
	http.Redirect(w, r, fmt.Sprintf("/polls/%s/results", questionID), http.StatusSeeOther) //nolint:gosec // questionID comes from chi URLParam, path-routed and safe
	return nil
}

func (a *App) Results(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	question, err := a.repo.GetQuestion(r.Context(), id)
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

//go:embed templates
var templateFS embed.FS

type App struct {
	repo *Repository
}

func New() *App { return &App{} }

func (a *App) Name() string { return "polls" }

func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
	a.repo = NewRepository(cfg.DB)
	return nil
}

func (a *App) Documents() []document.Document {
	return []document.Document{&Question{}, &Choice{}}
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
		r.Get("/", burrow.Handle(a.List))
		r.Get("/{id}", burrow.Handle(a.Detail))
		r.Post("/{id}/vote", burrow.Handle(a.Vote))
		r.Get("/{id}/results", burrow.Handle(a.Results))
	})
}

// Seed inserts a few example questions when the server is started with --seed.
// Implements [burrow.Seedable].
func (a *App) Seed(ctx context.Context) error {
	samples := []struct {
		text    string
		choices []string
	}{
		{"What's your favourite Go web framework?", []string{"Burrow", "Gin", "Echo", "net/http alone"}},
		{"How long have you been writing Go?", []string{"<1 year", "1–3 years", "3–5 years", "5+ years"}},
		{"Which IDE do you prefer for Go?", []string{"VS Code", "GoLand", "Vim/Neovim", "Cursor"}},
	}
	for _, s := range samples {
		q := &Question{Text: s.text, PublishedAt: time.Now()}
		if err := a.repo.CreateQuestion(ctx, q); err != nil {
			return fmt.Errorf("seed question %q: %w", s.text, err)
		}
		for _, ct := range s.choices {
			if err := a.repo.CreateChoice(ctx, &Choice{QuestionID: q.ID, Text: ct}); err != nil {
				return fmt.Errorf("seed choice %q: %w", ct, err)
			}
		}
	}
	return nil
}
