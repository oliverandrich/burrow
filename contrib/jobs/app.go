package jobs

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"time"

	"codeberg.org/oliverandrich/burrow"
	"github.com/urfave/cli/v3"
)

//go:embed migrations
var migrationFS embed.FS

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

// App implements the jobs contrib app.
type App struct {
	repo       *Repository
	handlers   map[string]HandlerFunc
	retries    map[string]int
	worker     *Worker
	cancelFunc context.CancelFunc
}

// New creates a new jobs app.
func New() *App {
	return &App{
		handlers: make(map[string]HandlerFunc),
		retries:  make(map[string]int),
	}
}

func (a *App) Name() string { return "jobs" }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.repo = NewRepository(cfg.DB)
	return nil
}

func (a *App) MigrationFS() fs.FS {
	sub, _ := fs.Sub(migrationFS, "migrations")
	return sub
}

func (a *App) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.IntFlag{
			Name:    "jobs-workers",
			Value:   2,
			Usage:   "Number of concurrent job worker goroutines",
			Sources: cli.EnvVars("JOBS_WORKERS"),
		},
		&cli.DurationFlag{
			Name:    "jobs-poll-interval",
			Value:   time.Second,
			Usage:   "Interval between job queue polls",
			Sources: cli.EnvVars("JOBS_POLL_INTERVAL"),
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

// Compile-time interface assertions.
var (
	_ burrow.App          = (*App)(nil)
	_ burrow.Migratable   = (*App)(nil)
	_ burrow.Configurable = (*App)(nil)
	_ burrow.HasShutdown  = (*App)(nil)
)
