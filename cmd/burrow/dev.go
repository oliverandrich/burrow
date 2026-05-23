package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oliverandrich/burrow/internal/dev"
	"github.com/urfave/cli/v3"
)

func devCommand() *cli.Command {
	return &cli.Command{
		Name:  "dev",
		Usage: "Run the app with live reload — watcher + Tailwind co-watcher in one process",
		Description: `Boot the application via ` + "`go run`" + ` and restart it whenever a watched
file changes. When the project's conventional Tailwind layout
(` + "`tailwind.css`" + ` at the root plus exactly one
` + "`internal/<app>/static/`" + ` directory) is detected, a parallel
` + "`tailwindcss --watch`" + ` child rebuilds the CSS bundle incrementally.

Auto-discovery resolves the entry-point (single ` + "`cmd/<name>/main.go`" + `)
and Tailwind paths from the conventional burrow project layout.
Override with --app, --css-in, --css-out, or disable Tailwind via
--no-tailwind. Pass --no-env-file to skip both reading and
auto-generation of the dev env file.`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "init-env",
				Usage: "Create the env file with default dev keys if missing, then exit (no dev loop). Used by the scaffold's mise setup task.",
			},
			&cli.StringFlag{
				Name:  "app",
				Usage: "Path to the cmd/<name> directory (auto-detected when there is exactly one).",
			},
			&cli.StringFlag{
				Name:  "css-in",
				Usage: "Tailwind input CSS (auto-detected as tailwind.css when present).",
			},
			&cli.StringFlag{
				Name:  "css-out",
				Usage: "Tailwind output CSS (auto-detected from internal/<app>/static/).",
			},
			&cli.BoolFlag{
				Name:  "no-tailwind",
				Usage: "Disable the Tailwind --watch co-process even if tailwind.css is present.",
			},
			&cli.StringFlag{
				Name:  "env-file",
				Usage: "Env file to read into the child process. Auto-created with default dev keys when missing.",
				Value: ".env",
			},
			&cli.BoolFlag{
				Name:  "no-env-file",
				Usage: "Skip env-file reading and auto-generation.",
			},
			&cli.DurationFlag{
				Name:  "debounce",
				Usage: "Quiet window before a restart fires after a file change.",
				Value: 300 * time.Millisecond,
			},
		},
		Action: runDev,
	}
}

func runDev(ctx context.Context, cmd *cli.Command) error {
	projectRoot, err := dev.ProjectRoot(ctx)
	if err != nil {
		return fmt.Errorf("burrow dev: %w", err)
	}

	cfg, err := dev.Discover(projectRoot)
	if err != nil {
		// --init-env only cares about the project root and env file —
		// don't fail on a missing/ambiguous app entry-point.
		if !cmd.Bool("init-env") {
			return fmt.Errorf("burrow dev: %w", err)
		}
		cfg = dev.Config{ProjectRoot: projectRoot}
	}

	if v := cmd.String("app"); v != "" {
		cfg.AppPath = v
	}
	if v := cmd.String("css-in"); v != "" {
		cfg.CSSIn = v
	}
	if v := cmd.String("css-out"); v != "" {
		cfg.CSSOut = v
	}
	if cmd.Bool("no-tailwind") {
		cfg.CSSIn = ""
		cfg.CSSOut = ""
	}
	if cmd.Bool("no-env-file") {
		cfg.EnvFile = ""
	} else if v := cmd.String("env-file"); v != "" {
		cfg.EnvFile = v
	}
	if v := cmd.Duration("debounce"); v > 0 {
		cfg.Debounce = v
	}

	if cmd.Bool("init-env") {
		if cfg.EnvFile == "" {
			cfg.EnvFile = ".env"
		}
		return dev.EnsureEnv(cfg, os.Stdout)
	}

	// Translate Ctrl-C / SIGTERM into ctx cancellation so the
	// supervisors get a chance to stop their children cleanly.
	signalCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return dev.Run(signalCtx, cfg)
}
