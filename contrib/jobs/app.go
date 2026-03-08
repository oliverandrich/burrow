package jobs

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"time"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/admin/modeladmin"
	"codeberg.org/oliverandrich/burrow/contrib/bsicons"
	"github.com/go-chi/chi/v5"
	"github.com/urfave/cli/v3"
)

//go:embed migrations
var migrationFS embed.FS

//go:embed translations
var translationFS embed.FS

//go:embed templates/*.html
var htmlTemplateFS embed.FS

// JobOption configures job handler registration.
type JobOption func(*handlerConfig)

type handlerConfig struct {
	maxRetries int
}

// WithMaxRetries sets the maximum number of retries for a job type.
func WithMaxRetries(n int) JobOption {
	return func(c *handlerConfig) {
		c.maxRetries = n
	}
}

// Option configures the jobs app.
type Option func(*App)

// App implements the jobs contrib app.
type App struct {
	repo       *Repository
	handlers   map[string]HandlerFunc
	retries    map[string]int
	worker     *Worker
	cancelFunc context.CancelFunc
	jobsAdmin  *modeladmin.ModelAdmin[Job]
}

// New creates a new jobs app with the given options.
func New(opts ...Option) *App {
	a := &App{
		handlers: make(map[string]HandlerFunc),
		retries:  make(map[string]int),
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func (a *App) Name() string { return "jobs" }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.repo = NewRepository(cfg.DB)

	a.jobsAdmin = &modeladmin.ModelAdmin[Job]{
		Slug:              "jobs",
		DisplayName:       "Job",
		DisplayPluralName: "Jobs",
		DB:                cfg.DB,
		Renderer:          newJobsRenderer(),
		CanCreate:         false,
		CanEdit:           false,
		CanDelete:         true,
		ListFields:        []string{"ID", "Type", "Status", "Attempts", "CreatedAt"},
		OrderBy:           "created_at DESC, id DESC",
		PageSize:          25,
		EmptyMessageKey:   "admin-jobs-empty",
		Filters: []modeladmin.FilterDef{
			{Field: "status", Label: "Status", Type: "select", Choices: statusChoices()},
		},
		RowActions: []modeladmin.RowAction{
			{
				Slug:     "retry",
				Label:    "admin-jobs-action-retry",
				Icon:     bsicons.ArrowCounterclockwise(),
				Class:    "btn-outline-success",
				Handler:  retryHandler(a.repo),
				ShowWhen: isRetryable,
			},
			{
				Slug:     "cancel",
				Label:    "admin-jobs-action-cancel",
				Icon:     bsicons.XCircle(),
				Class:    "btn-outline-warning",
				Confirm:  "admin-jobs-cancel-confirm",
				Handler:  cancelHandler(a.repo),
				ShowWhen: isCancellable,
			},
		},
	}
	return nil
}

func (a *App) MigrationFS() fs.FS {
	sub, _ := fs.Sub(migrationFS, "migrations")
	return sub
}

func (a *App) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
	return []cli.Flag{
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
	}
}

func (a *App) Configure(cmd *cli.Command) error {
	cfg := DefaultWorkerConfig()
	cfg.NumWorkers = int(cmd.Int("jobs-workers"))
	cfg.PollInterval = cmd.Duration("jobs-poll-interval")

	ctx, cancel := context.WithCancel(context.Background())
	a.cancelFunc = cancel
	a.worker = NewWorker(a.repo, a.handlers, cfg)

	go a.worker.Start(ctx)
	return nil
}

// Shutdown stops the worker pool and waits for in-flight jobs to finish.
func (a *App) Shutdown(_ context.Context) error {
	if a.cancelFunc != nil {
		a.cancelFunc()
	}
	if a.worker != nil {
		<-a.worker.Done()
	}
	return nil
}

// Handle registers a handler function for a job type. Call this during
// your app's Register() phase, before Configure() starts the workers.
func (a *App) Handle(typeName string, fn HandlerFunc, opts ...JobOption) {
	cfg := handlerConfig{maxRetries: 3}
	for _, o := range opts {
		o(&cfg)
	}
	a.handlers[typeName] = fn
	a.retries[typeName] = cfg.maxRetries
}

// Enqueue adds a job to the queue for immediate processing.
// The payload is marshaled to JSON. The type must be registered via Handle().
func (a *App) Enqueue(ctx context.Context, typeName string, payload any) (*Job, error) {
	return a.EnqueueAt(ctx, typeName, payload, time.Now())
}

// EnqueueAt adds a job to the queue scheduled for a specific time.
// The payload is marshaled to JSON. The type must be registered via Handle().
func (a *App) EnqueueAt(ctx context.Context, typeName string, payload any, runAt time.Time) (*Job, error) {
	if _, ok := a.handlers[typeName]; !ok {
		return nil, fmt.Errorf("jobs: unknown type %q (not registered via Handle)", typeName)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("jobs: marshal payload for %q: %w", typeName, err)
	}

	maxRetries := a.retries[typeName]
	return a.repo.Enqueue(ctx, typeName, string(data), maxRetries, runAt)
}

// AdminRoutes registers admin routes for job management.
func (a *App) AdminRoutes(r chi.Router) {
	if a.jobsAdmin != nil {
		a.jobsAdmin.Routes(r)
	}
}

// AdminNavItems returns navigation items for the admin panel.
func (a *App) AdminNavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{
			Label:     "Jobs",
			LabelKey:  "admin-nav-jobs",
			URL:       "/admin/jobs",
			Icon:      bsicons.ListTask(),
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
		"prettyJSON":                prettyJSON,
		"jobStatus":                 func(j Job) string { return string(j.Status) },
		"string":                    func(v any) string { return fmt.Sprint(v) },
		"iconArrowCounterclockwise": func(class ...string) template.HTML { return bsicons.ArrowCounterclockwise(class...) },
		"iconXCircle":               func(class ...string) template.HTML { return bsicons.XCircle(class...) },
		"iconTrash":                 func(class ...string) template.HTML { return bsicons.Trash(class...) },
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

// Compile-time interface assertions.
var (
	_ burrow.App             = (*App)(nil)
	_ burrow.Migratable      = (*App)(nil)
	_ burrow.Configurable    = (*App)(nil)
	_ burrow.HasShutdown     = (*App)(nil)
	_ burrow.HasAdmin        = (*App)(nil)
	_ burrow.HasTranslations = (*App)(nil)
	_ burrow.HasTemplates    = (*App)(nil)
	_ burrow.HasFuncMap      = (*App)(nil)
)
