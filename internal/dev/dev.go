package dev

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// Run blocks until ctx is cancelled, supervising the application child
// process. On each debounced file change it:
//
//  1. SIGTERMs the running app (waits up to KillTimeout, then SIGKILL).
//  2. Runs `burrow tailwind` once to regenerate the CSS bundle (when
//     cfg.CSSIn and cfg.CSSOut are both set). This is intentionally
//     sequential, not a parallel `--watch` co-process: the Go binary
//     embeds `app.min.css` at compile time via `//go:embed`, so a
//     running parallel rebuild would not be picked up until the next
//     restart anyway.
//  3. Spawns `go run cfg.AppPath` again (Go's build cache keeps warm
//     rebuilds fast).
//
// If the app exits on its own (build error, panic, port-in-use) the
// supervisor state is reset and a notice is logged — the next file
// change triggers a fresh attempt.
//
// On ctx cancellation Run gracefully stops the child (SIGTERM →
// cfg.KillTimeout → SIGKILL) before returning.
func Run(ctx context.Context, cfg Config) error {
	if err := applyDefaults(&cfg); err != nil {
		return err
	}

	// Help users out who clearly *meant* to enable Tailwind (the file
	// is there) but auto-discovery couldn't pin a unique output dir.
	if cfg.CSSIn == "" && cfg.CSSOut == "" {
		if _, statErr := os.Stat(filepath.Join(cfg.ProjectRoot, "tailwind.css")); statErr == nil {
			logf(cfg.Stdout, "burrow dev: tailwind.css found but co-build is disabled — pass --css-out to enable\n")
		}
	}

	envOverrides, err := loadOrGenerateEnv(cfg, cfg.Stdout)
	if err != nil {
		return err
	}
	childEnv := mergeEnv(os.Environ(), envOverrides)

	tailwindRunner, err := buildTailwindRunner(cfg)
	if err != nil {
		return err
	}

	app := newSupervisor(ctx, "app", cfg.ProjectRoot, childEnv, cfg, "go", "run", cfg.AppPath)

	if err := tailwindRunner(ctx); err != nil {
		return fmt.Errorf("dev: initial tailwind build: %w", err)
	}
	if err := app.Start(); err != nil {
		return err
	}
	logf(cfg.Stdout, "burrow dev: app started (%s)\n", cfg.AppPath)

	w, err := newWatcher(cfg)
	if err != nil {
		_ = app.Stop()
		return fmt.Errorf("dev: watcher: %w", err)
	}

	logf(cfg.Stdout, "burrow dev: watching %s (debounce %s)\n", cfg.ProjectRoot, cfg.Debounce)

	var wg sync.WaitGroup

	// Crash watcher: detect unsolicited app exits, log them, and reset
	// supervisor state so the next file event can Start cleanly.
	wg.Go(func() { monitorAppExits(ctx, app, cfg.Stderr) })

	// File watcher: on each debounced trigger, rebuild CSS (sequential)
	// and restart the app.
	wg.Go(func() {
		w.Run(ctx, func() {
			logf(cfg.Stdout, "burrow dev: change detected — rebuilding\n")
			if err := tailwindRunner(ctx); err != nil {
				logf(cfg.Stderr, "burrow dev: tailwind rebuild failed: %v\n", err)
				// Continue: the app restart may still be useful for
				// Go-only changes; the failed CSS just stays stale.
			}
			if err := app.Restart(); err != nil {
				logf(cfg.Stderr, "burrow dev: restart failed: %v\n", err)
			}
		})
	})

	<-ctx.Done()
	logf(cfg.Stdout, "burrow dev: shutting down\n")

	stopErr := app.Stop()
	_ = w.Close()
	wg.Wait()
	return stopErr
}

// buildTailwindRunner returns a closure that re-executes the running
// burrow binary as `burrow tailwind -i <in> -o <out>` (no --watch). When
// cfg.CSSIn or cfg.CSSOut is empty the runner is a no-op — the user has
// opted out (--no-tailwind, or auto-discovery did not find the
// conventional layout).
func buildTailwindRunner(cfg Config) (func(context.Context) error, error) {
	if cfg.CSSIn == "" || cfg.CSSOut == "" {
		return func(context.Context) error { return nil }, nil
	}
	burrowBin, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("dev: locate burrow binary for tailwind: %w", err)
	}
	return func(ctx context.Context) error {
		c := exec.CommandContext(ctx, burrowBin, "tailwind", "-i", cfg.CSSIn, "-o", cfg.CSSOut) //nolint:gosec // dev-tooling: paths are project-owner configuration.
		c.Dir = cfg.ProjectRoot
		c.Stdout = cfg.Stdout
		c.Stderr = cfg.Stderr
		return c.Run()
	}, nil
}

// monitorAppExits observes the app supervisor's current child for
// unsolicited exits (panics, build errors, port-in-use). When one is
// observed it logs the error and resets the supervisor state so the
// next watcher trigger can Start cleanly. Exits cleanly when ctx is
// cancelled.
func monitorAppExits(ctx context.Context, s *supervisor, log io.Writer) {
	for {
		exited, ok := s.currentExitedChan()
		if !ok {
			// No child running. Wait for one to start, or for shutdown.
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				continue
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-exited:
			did, err := s.handleUnsolicitedExit(exited)
			if !did {
				// A Restart raced ahead while we were unblocking. The
				// fresh child is the supervisor's current concern; do
				// not log over its start or touch its lifecycle.
				continue
			}
			// Skip the log if shutdown is in flight — the exit was
			// almost certainly our own SIGTERM and the user has
			// already seen `burrow dev: shutting down`.
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				logf(log, "burrow dev: app exited: %v — save a file to retry\n", err)
			} else {
				logf(log, "burrow dev: app exited cleanly — save a file to retry\n")
			}
		}
	}
}

// logf is a fmt.Fprintf wrapper that swallows the always-nil error of
// the common io.Writer destinations (os.Stdout, bytes.Buffer). Used
// throughout the package so error-check linters stay happy without
// polluting every log line.
func logf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func applyDefaults(cfg *Config) error {
	if cfg.ProjectRoot == "" {
		return errors.New("dev: ProjectRoot is required")
	}
	if cfg.AppPath == "" {
		return errors.New("dev: AppPath is required")
	}
	if len(cfg.WatchExts) == 0 {
		cfg.WatchExts = append([]string{}, defaultWatchExts...)
	}
	if len(cfg.ExcludeDirs) == 0 {
		cfg.ExcludeDirs = append([]string{}, defaultExcludeDirs...)
	}
	// Ensure the Tailwind output directory is excluded from watching —
	// without this the post-build CSS write would queue a fsnotify
	// event during the next debounce window and trigger a needless
	// extra restart. Done here (not in Discover) so that --css-out
	// flag overrides are honoured.
	if cfg.CSSOut != "" {
		outDir := filepath.Dir(cfg.CSSOut)
		if outDir != "" && outDir != "." && !slices.Contains(cfg.ExcludeDirs, outDir) {
			cfg.ExcludeDirs = append(cfg.ExcludeDirs, outDir)
		}
	}
	if cfg.Debounce <= 0 {
		cfg.Debounce = defaultDebounce
	}
	if cfg.KillTimeout <= 0 {
		cfg.KillTimeout = defaultKillTimeout
	}
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	return nil
}
