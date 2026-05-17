package jobs

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/den"
	"github.com/urfave/cli/v3"
)

//go:embed translations
var translationFS embed.FS

//go:embed templates/*.html
var htmlTemplateFS embed.FS

// Option configures the jobs app.
type Option func(*App)

// App implements the jobs contrib app.
type App struct {
	defaultDB  *den.DB
	ownDB      *den.DB
	repo       *Repository
	registry   *burrow.Registry
	handlers   map[string]burrow.JobHandlerFunc
	retries    map[string]int
	priorities map[string]int
	worker     *Worker
	cancelFunc context.CancelFunc
	workerCfg  WorkerConfig
}

// New creates a new jobs app with the given options.
func New(opts ...Option) *App {
	a := &App{
		handlers:   make(map[string]burrow.JobHandlerFunc),
		retries:    make(map[string]int),
		priorities: make(map[string]int),
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func (a *App) Name() string { return "jobs" }

// Documents returns the document types for this app.
func (a *App) Documents() []any {
	return []any{&Job{}}
}

func (a *App) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "jobs-database-dsn",
			Usage:   "Database URL for a separate jobs database, e.g. sqlite:///jobs.db (empty = use shared DB)",
			Sources: burrow.FlagSources(configSource, "JOBS_DATABASE_DSN", "jobs.database_dsn"),
		},
		&cli.IntFlag{
			Name:    "jobs-workers",
			Value:   2,
			Usage:   "Number of concurrent job worker goroutines",
			Sources: burrow.FlagSources(configSource, "JOBS_WORKERS", "jobs.workers"),
		},
		&cli.DurationFlag{
			Name:    "jobs-poll-interval",
			Value:   time.Second,
			Usage:   "Interval between job queue polls",
			Sources: burrow.FlagSources(configSource, "JOBS_POLL_INTERVAL", "jobs.poll_interval"),
		},
		&cli.DurationFlag{
			Name:    "jobs-retry-base-delay",
			Value:   30 * time.Second,
			Usage:   "Base delay for exponential retry backoff (delay * 2^(attempt-1))",
			Sources: burrow.FlagSources(configSource, "JOBS_RETRY_BASE_DELAY", "jobs.retry_base_delay"),
		},
	}
}

func (a *App) Configure(cfg *burrow.AppConfig, cmd *cli.Command) error {
	a.defaultDB = cfg.DB
	a.registry = cfg.Registry

	// Determine effective database: separate DB if configured, shared DB otherwise.
	effectiveDB, err := a.resolveDB(context.Background(), cmd.String("jobs-database-dsn"))
	if err != nil {
		return err
	}

	a.repo = NewRepository(effectiveDB)

	// Store worker config for PostConfigure (which starts the worker).
	a.workerCfg = DefaultWorkerConfig()
	a.workerCfg.NumWorkers = int(cmd.Int("jobs-workers"))
	a.workerCfg.PollInterval = cmd.Duration("jobs-poll-interval")
	a.workerCfg.RetryBaseDelay = cmd.Duration("jobs-retry-base-delay")
	return nil
}

// PostConfigure discovers HasJobs implementors and registers their handlers.
func (a *App) PostConfigure(_ *burrow.AppConfig, _ *cli.Command) error {
	if a.registry != nil {
		for _, app := range a.registry.Apps() {
			if hj, ok := app.(burrow.HasJobs); ok {
				hj.RegisterJobs(a)
			}
		}
	}
	return nil
}

// Start creates the worker pool and launches background goroutines.
func (a *App) Start(srv *burrow.Server) error {
	a.worker = NewWorker(a.repo, a.handlers, a.workerCfg, srv.TemplateExecutor())
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelFunc = cancel
	go a.worker.Start(ctx)
	return nil
}

// resolveDB opens a separate database if dsn is non-empty, registers document
// types on it, and returns it. Otherwise it returns the shared defaultDB.
func (a *App) resolveDB(ctx context.Context, dsn string) (*den.DB, error) {
	if dsn == "" {
		return a.defaultDB, nil
	}

	db, err := burrow.OpenDB(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("jobs: open separate database: %w", err)
	}
	a.ownDB = db

	// Register document types on the separate DB
	if err := den.Register(ctx, db, a.Documents()...); err != nil {
		return nil, fmt.Errorf("jobs: register documents on separate database: %w", err)
	}

	slog.Info("jobs: using separate database", "dsn", dsn)
	return db, nil
}

// Shutdown stops the worker pool, waits for in-flight jobs to finish,
// and closes the separate database connection if one was opened.
func (a *App) Shutdown(_ context.Context) error {
	if a.cancelFunc != nil {
		a.cancelFunc()
	}
	if a.worker != nil {
		<-a.worker.Done()
	}
	if a.ownDB != nil {
		if err := a.ownDB.Close(); err != nil {
			return fmt.Errorf("jobs: close separate database: %w", err)
		}
	}
	return nil
}

// Handle registers a handler function for a job type.
func (a *App) Handle(typeName string, fn burrow.JobHandlerFunc, opts ...burrow.JobOption) {
	cfg := burrow.JobConfig{MaxRetries: 3}
	for _, o := range opts {
		o(&cfg)
	}
	a.handlers[typeName] = fn
	a.retries[typeName] = cfg.MaxRetries
	a.priorities[typeName] = cfg.Priority
}

// Enqueue adds a job to the queue for immediate processing.
func (a *App) Enqueue(ctx context.Context, typeName string, payload any) (string, error) {
	return a.EnqueueAt(ctx, typeName, payload, time.Now())
}

// EnqueueAt adds a job to the queue scheduled for a specific time.
func (a *App) EnqueueAt(ctx context.Context, typeName string, payload any, runAt time.Time) (string, error) {
	if _, ok := a.handlers[typeName]; !ok {
		return "", fmt.Errorf("jobs: unknown type %q (not registered via Handle)", typeName)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("jobs: marshal payload for %q: %w", typeName, err)
	}

	maxRetries := a.retries[typeName]
	priority := a.priorities[typeName]
	job, err := a.repo.Enqueue(ctx, typeName, string(data), maxRetries, priority, runAt)
	if err != nil {
		return "", err
	}
	return job.ID, nil
}

// Dequeue cancels a pending job by its ID.
func (a *App) Dequeue(ctx context.Context, id string) error {
	return a.repo.Cancel(ctx, id)
}

// AdminRoutes registers admin routes for job management.
func (a *App) AdminRoutes(r chi.Router) {
	r.Get("/jobs", burrow.Handle(a.adminListJobs))
	r.Get("/jobs/{id}", burrow.Handle(a.adminJobDetail))
	r.Delete("/jobs/{id}", burrow.Handle(a.adminDeleteJob))
	r.Post("/jobs/{id}/retry", burrow.Handle(retryHandler(a.repo)))
	r.Post("/jobs/{id}/cancel", burrow.Handle(cancelHandler(a.repo)))
}

// AdminNavItems returns navigation items for the admin panel.
func (a *App) AdminNavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{
			Label:     "Jobs",
			LabelKey:  "admin-nav-jobs",
			URL:       "/admin/jobs",
			Icon:      "jobs/icon_list_task",
			Position:  40,
			AdminOnly: true,
		},
	}
}

// TemplateFS returns the embedded HTML template files.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(htmlTemplateFS, "templates")
	return sub
}

// FuncMap returns static template functions for jobs templates.
func (a *App) FuncMap() template.FuncMap {
	return template.FuncMap{
		"prettyJSON": prettyJSON,
		"jobStatus":  func(j Job) string { return string(j.Status) },
		"string":     func(v any) string { return fmt.Sprint(v) },
	}
}

// prettyJSON formats a JSON string with indentation, or returns it as-is if invalid.
func prettyJSON(s string) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(s), "", "  "); err != nil {
		return s
	}
	return buf.String()
}

// TranslationFS returns the embedded translation files.
func (a *App) TranslationFS() fs.FS { return translationFS }
