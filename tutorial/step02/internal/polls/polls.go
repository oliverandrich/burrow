// Package polls implements a polls application for the burrow tutorial.
// It provides questions with multiple choices that users can vote on.
package polls

import (
	"context"
	"embed"
	"io/fs"
	"time"

	"codeberg.org/oliverandrich/burrow"
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
// App
// --------------------------------------------------------------------------

//go:embed migrations
var migrationFS embed.FS

// App is the polls burrow application.
type App struct {
	repo *Repository
}

// New creates a new polls app.
func New() *App {
	return &App{}
}

func (a *App) Name() string { return "polls" }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.repo = NewRepository(cfg.DB)
	return nil
}

func (a *App) MigrationFS() fs.FS {
	sub, _ := fs.Sub(migrationFS, "migrations")
	return sub
}
