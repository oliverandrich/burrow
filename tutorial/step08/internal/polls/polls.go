// Package polls implements a polls application for the burrow tutorial.
// Step 8 adds an admin panel for questions and choices via HasAdmin.
package polls

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"
	"github.com/oliverandrich/den/where"
	"github.com/urfave/cli/v3"
)

// notFoundOrServerError discriminates between "no such row" (HTTP 404)
// and any other repository error (HTTP 500 — DB connection failure,
// disk error, etc.). The repository wraps `den.ErrNotFound` via `%w`,
// so `errors.Is` traverses the chain to find it. Without this check a
// transient DB outage would silently surface as 404 to the user.
func notFoundOrServerError(err error, notFoundMsg, serverMsg string) error {
	if errors.Is(err, den.ErrNotFound) {
		return burrow.NewHTTPError(http.StatusNotFound, notFoundMsg)
	}
	return burrow.NewHTTPError(http.StatusInternalServerError, serverMsg)
}

// --------------------------------------------------------------------------
// Models
// --------------------------------------------------------------------------

type Question struct {
	document.Base
	PublishedAt time.Time `json:"published_at" den:"index" verbose:"Published"`
	Text        string    `json:"text" verbose:"Question"`
	Choices     []Choice  `json:"-"`
}

type Choice struct {
	document.Base
	QuestionID string `json:"question_id" den:"index" verbose:"Question"`
	Text       string `json:"text" verbose:"Choice"`
	Votes      int    `json:"votes" verbose:"Votes"`
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

func (r *Repository) ListQuestionsPaged(ctx context.Context, pr burrow.PageRequest) ([]Question, burrow.PageResult, error) {
	ptrs, count, err := den.NewQuery[Question](r.db).
		Sort("_id", den.Desc).
		Limit(pr.Limit).
		Skip(pr.Offset()).
		AllWithCount(ctx)
	if err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("list questions paged: %w", err)
	}

	questions := make([]Question, len(ptrs))
	for i, p := range ptrs {
		questions[i] = *p
	}
	return questions, burrow.OffsetResult(pr, int(count)), nil
}

// GetQuestionByID fetches a question without its choices (single query).
// Use this in admin handlers that only need to verify existence or update
// the question text — they don't need to pay for the second choices query.
func (r *Repository) GetQuestionByID(ctx context.Context, id string) (*Question, error) {
	question, err := den.FindByID[Question](ctx, r.db, id)
	if err != nil {
		return nil, fmt.Errorf("get question %s: %w", id, err)
	}
	return question, nil
}

// GetQuestion fetches a question and its choices (two queries).
func (r *Repository) GetQuestion(ctx context.Context, id string) (*Question, error) {
	question, err := r.GetQuestionByID(ctx, id)
	if err != nil {
		return nil, err
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
	return den.Insert(ctx, r.db, q)
}

// CreateChoice inserts a new choice for a question.
func (r *Repository) CreateChoice(ctx context.Context, c *Choice) error {
	return den.Insert(ctx, r.db, c)
}

// SearchQuestionsPaged searches questions by their text with pagination.
func (r *Repository) SearchQuestionsPaged(ctx context.Context, query string, pr burrow.PageRequest) ([]Question, burrow.PageResult, error) {
	ptrs, count, err := den.NewQuery[Question](r.db, where.Field("text").StringContains(query)).
		Sort("_id", den.Desc).
		Limit(pr.Limit).
		Skip(pr.Offset()).
		AllWithCount(ctx)
	if err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("search questions: %w", err)
	}
	questions := make([]Question, len(ptrs))
	for i, p := range ptrs {
		questions[i] = *p
	}
	return questions, burrow.OffsetResult(pr, int(count)), nil
}

// UpdateQuestion saves changes to an existing question.
func (r *Repository) UpdateQuestion(ctx context.Context, q *Question) error {
	return den.Update(ctx, r.db, q)
}

// DeleteQuestion removes a question and all its choices.
func (r *Repository) DeleteQuestion(ctx context.Context, id string) error {
	if _, err := den.DeleteMany[Choice](ctx, r.db, []where.Condition{where.Field("question_id").Eq(id)}); err != nil {
		return fmt.Errorf("delete choices for %s: %w", id, err)
	}
	q, err := r.GetQuestionByID(ctx, id)
	if err != nil {
		return err
	}
	return den.Delete(ctx, r.db, q)
}

// UpdateChoice saves changes to an existing choice.
func (r *Repository) UpdateChoice(ctx context.Context, c *Choice) error {
	return den.Update(ctx, r.db, c)
}

// DeleteChoice removes a single choice.
func (r *Repository) DeleteChoice(ctx context.Context, id string) error {
	c, err := den.FindByID[Choice](ctx, r.db, id)
	if err != nil {
		return fmt.Errorf("find choice %s: %w", id, err)
	}
	return den.Delete(ctx, r.db, c)
}

// GetChoice fetches a choice by id.
func (r *Repository) GetChoice(ctx context.Context, id string) (*Choice, error) {
	c, err := den.FindByID[Choice](ctx, r.db, id)
	if err != nil {
		return nil, fmt.Errorf("get choice %s: %w", id, err)
	}
	return c, nil
}

func (r *Repository) IncrementVotes(ctx context.Context, choiceID string) error {
	choice, err := den.FindByID[Choice](ctx, r.db, choiceID)
	if err != nil {
		return fmt.Errorf("find choice %s: %w", choiceID, err)
	}
	choice.Votes++
	return den.Update(ctx, r.db, choice)
}

// --------------------------------------------------------------------------
// Handlers
// --------------------------------------------------------------------------

func (a *App) List(w http.ResponseWriter, r *http.Request) error {
	pr := burrow.ParsePageRequest(r)
	questions, page, err := a.repo.ListQuestionsPaged(r.Context(), pr)
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list questions")
	}

	data := map[string]any{
		"Title":     "Polls",
		"Questions": questions,
		"Page":      page,
	}

	// For HTMX infinite scroll requests, return only the items fragment.
	if htmx.Request(r).IsHTMX() && pr.Page > 1 {
		return burrow.Render(w, r, http.StatusOK, "polls/list_page", data)
	}

	return burrow.Render(w, r, http.StatusOK, "polls/list", data)
}

func (a *App) Detail(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	question, err := a.repo.GetQuestion(r.Context(), id)
	if err != nil {
		return notFoundOrServerError(err, "question not found", "failed to load question")
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
		htmx.SmartRedirect(w, r, fmt.Sprintf("/polls/%s", questionID))
		return nil
	}

	if err := a.repo.IncrementVotes(r.Context(), choiceID); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to record vote")
	}

	if err := messages.AddSuccess(w, r, "Your vote has been recorded!"); err != nil {
		return err
	}
	htmx.SmartRedirect(w, r, fmt.Sprintf("/polls/%s/results", questionID))
	return nil
}

func (a *App) Results(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	question, err := a.repo.GetQuestion(r.Context(), id)
	if err != nil {
		return notFoundOrServerError(err, "question not found", "failed to load question")
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

func (a *App) Dependencies() []string { return []string{"auth"} }

func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
	a.repo = NewRepository(cfg.DB)

	return nil
}

func (a *App) Documents() []any {
	return []any{&Question{}, &Choice{}}
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
		r.Get("/{id}/results", burrow.Handle(a.Results))

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth())
			r.Post("/{id}/vote", burrow.Handle(a.Vote))
		})
	})
}

// --------------------------------------------------------------------------
// Admin (HasAdmin)
// --------------------------------------------------------------------------

// AdminNavItems contributes one nav item to the admin sidebar.
// Implements [burrow.HasAdmin].
func (a *App) AdminNavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{Label: "Polls", URL: "/admin/polls", Position: 50},
	}
}

// AdminRoutes mounts the admin views under /admin/polls.
// The chi router passed in is already prefixed with /admin and protected
// by the admin auth middleware.
// Implements [burrow.HasAdmin].
func (a *App) AdminRoutes(r chi.Router) {
	r.Route("/polls", func(r chi.Router) {
		r.Get("/", burrow.Handle(a.adminList))
		r.Get("/new", burrow.Handle(a.adminNew))
		r.Post("/", burrow.Handle(a.adminCreate))
		r.Get("/{id}", burrow.Handle(a.adminEdit))
		r.Post("/{id}", burrow.Handle(a.adminUpdate))
		r.Delete("/{id}", burrow.Handle(a.adminDelete))
		r.Post("/{id}/choices", burrow.Handle(a.adminAddChoice))
		r.Post("/{id}/choices/{choiceID}", burrow.Handle(a.adminUpdateChoice))
		r.Delete("/{id}/choices/{choiceID}", burrow.Handle(a.adminDeleteChoice))
	})
}

func (a *App) adminList(w http.ResponseWriter, r *http.Request) error {
	pr := burrow.ParsePageRequest(r)
	searchTerm := r.URL.Query().Get("q")

	var (
		questions []Question
		page      burrow.PageResult
		err       error
	)
	if searchTerm != "" {
		questions, page, err = a.repo.SearchQuestionsPaged(r.Context(), searchTerm, pr)
	} else {
		questions, page, err = a.repo.ListQuestionsPaged(r.Context(), pr)
	}
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list questions")
	}

	return burrow.Render(w, r, http.StatusOK, "polls/admin_list", map[string]any{
		"Title":      "Manage Polls",
		"Questions":  questions,
		"Page":       page,
		"SearchTerm": searchTerm,
		"RawQuery":   r.URL.RawQuery,
	})
}

func (a *App) adminNew(w http.ResponseWriter, r *http.Request) error {
	return burrow.Render(w, r, http.StatusOK, "polls/admin_form", map[string]any{
		"Title":    "New Poll",
		"Question": &Question{PublishedAt: time.Now()},
		"IsNew":    true,
	})
}

func (a *App) adminCreate(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		return burrow.NewHTTPError(http.StatusUnprocessableEntity, "question text is required")
	}

	q := &Question{Text: text, PublishedAt: time.Now()}
	if err := a.repo.CreateQuestion(r.Context(), q); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to create question")
	}

	for _, ct := range r.Form["choice"] {
		ct = strings.TrimSpace(ct)
		if ct == "" {
			continue
		}
		if err := a.repo.CreateChoice(r.Context(), &Choice{QuestionID: q.ID, Text: ct}); err != nil {
			return burrow.NewHTTPError(http.StatusInternalServerError, "failed to create choice")
		}
	}

	if err := messages.AddSuccess(w, r, "Poll created."); err != nil {
		return err
	}
	htmx.SmartRedirect(w, r, "/admin/polls/"+q.ID)
	return nil
}

func (a *App) adminEdit(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	question, err := a.repo.GetQuestion(r.Context(), id)
	if err != nil {
		return notFoundOrServerError(err, "question not found", "failed to load question")
	}
	return burrow.Render(w, r, http.StatusOK, "polls/admin_form", map[string]any{
		"Title":    "Edit Poll",
		"Question": question,
		"IsNew":    false,
	})
}

func (a *App) adminUpdate(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	id := chi.URLParam(r, "id")
	question, err := a.repo.GetQuestionByID(r.Context(), id)
	if err != nil {
		return notFoundOrServerError(err, "question not found", "failed to load question")
	}

	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		return burrow.NewHTTPError(http.StatusUnprocessableEntity, "question text is required")
	}
	question.Text = text
	if err := a.repo.UpdateQuestion(r.Context(), question); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to update question")
	}

	if err := messages.AddSuccess(w, r, "Poll saved."); err != nil {
		return err
	}
	htmx.SmartRedirect(w, r, "/admin/polls/"+id)
	return nil
}

func (a *App) adminDelete(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	if err := a.repo.DeleteQuestion(r.Context(), id); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to delete question")
	}
	if err := messages.AddSuccess(w, r, "Poll deleted."); err != nil {
		return err
	}
	htmx.SmartRedirect(w, r, "/admin/polls")
	return nil
}

func (a *App) adminAddChoice(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	id := chi.URLParam(r, "id")
	if _, err := a.repo.GetQuestionByID(r.Context(), id); err != nil {
		return notFoundOrServerError(err, "question not found", "failed to load question")
	}

	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		return burrow.NewHTTPError(http.StatusUnprocessableEntity, "choice text is required")
	}
	if err := a.repo.CreateChoice(r.Context(), &Choice{QuestionID: id, Text: text}); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to add choice")
	}

	htmx.SmartRedirect(w, r, "/admin/polls/"+id)
	return nil
}

func (a *App) adminUpdateChoice(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	id := chi.URLParam(r, "id")
	choiceID := chi.URLParam(r, "choiceID")
	choice, err := a.repo.GetChoice(r.Context(), choiceID)
	if err != nil {
		return notFoundOrServerError(err, "choice not found", "failed to load choice")
	}

	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		return burrow.NewHTTPError(http.StatusUnprocessableEntity, "choice text is required")
	}
	choice.Text = text
	if err := a.repo.UpdateChoice(r.Context(), choice); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to update choice")
	}

	htmx.SmartRedirect(w, r, "/admin/polls/"+id)
	return nil
}

func (a *App) adminDeleteChoice(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	choiceID := chi.URLParam(r, "choiceID")
	if err := a.repo.DeleteChoice(r.Context(), choiceID); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to delete choice")
	}
	htmx.SmartRedirect(w, r, "/admin/polls/"+id)
	return nil
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
