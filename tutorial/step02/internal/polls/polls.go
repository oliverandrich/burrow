// Package polls implements a polls application for the burrow tutorial.
// It provides questions with multiple choices that users can vote on.
package polls

import (
	"context"
	"fmt"
	"time"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"
	"github.com/oliverandrich/den/where"
	"github.com/urfave/cli/v3"
)

// --------------------------------------------------------------------------
// Models
// --------------------------------------------------------------------------

// Question represents a poll question.
type Question struct {
	document.Base
	PublishedAt time.Time `json:"published_at" den:"index"`
	Text        string    `json:"text"`
	Choices     []Choice  `json:"-"`
}

// Choice represents a possible answer to a question.
type Choice struct {
	document.Base
	QuestionID string `json:"question_id" den:"index"`
	Text       string `json:"text"`
	Votes      int    `json:"votes"`
}

// --------------------------------------------------------------------------
// Repository
// --------------------------------------------------------------------------

// Repository provides database access for polls.
type Repository struct {
	db *den.DB
}

// NewRepository creates a new polls repository.
func NewRepository(db *den.DB) *Repository {
	return &Repository{db: db}
}

// ListQuestions returns all questions ordered by publication date.
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

// GetQuestion returns a single question with its choices.
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

// IncrementVotes adds one vote to the given choice.
func (r *Repository) IncrementVotes(ctx context.Context, choiceID string) error {
	choice, err := den.FindByID[Choice](ctx, r.db, choiceID)
	if err != nil {
		return fmt.Errorf("find choice %s: %w", choiceID, err)
	}
	choice.Votes++
	return den.Save(ctx, r.db, choice)
}

// --------------------------------------------------------------------------
// App
// --------------------------------------------------------------------------

// App is the polls burrow application.
type App struct {
	repo *Repository
}

// New creates a new polls app.
func New() *App {
	return &App{}
}

func (a *App) Name() string { return "polls" }

func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
	a.repo = NewRepository(cfg.DB)
	return nil
}

func (a *App) Documents() []any {
	return []any{&Question{}, &Choice{}}
}
