//go:build !windows

package burrow

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudflare/tableflip"
)

func startServer(ctx context.Context, handler http.Handler, cfg *Config, registry *Registry) error {
	setup, err := configureTLS(cfg)
	if err != nil {
		return fmt.Errorf("configure TLS: %w", err)
	}

	// Skip tableflip when running as PID 1 (e.g. in Docker containers).
	// The parent would exit after fork, causing the container runtime to
	// stop the container even though the child is still running.
	if os.Getpid() == 1 {
		slog.Info("running as PID 1, graceful restart disabled")
		return startServerSimple(ctx, setup, handler, cfg, registry)
	}

	upg, upgErr := tableflip.New(tableflip.Options{
		PIDFile: cfg.Server.PIDFile,
	})
	if upgErr != nil {
		slog.Warn("tableflip unavailable, graceful restart disabled", "error", upgErr)
		return startServerSimple(ctx, setup, handler, cfg, registry)
	}
	defer upg.Stop()

	return startServerTableflip(ctx, upg, setup, handler, cfg, registry)
}

func startServerTableflip(
	ctx context.Context,
	upg *tableflip.Upgrader,
	setup *tlsSetup,
	handler http.Handler,
	cfg *Config,
	registry *Registry,
) error {
	// Trigger an upgrade on SIGHUP.
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGHUP)
		for range sig {
			if err := upg.Upgrade(); err != nil {
				slog.Error("upgrade failed", "error", err)
			}
		}
	}()

	// Create listeners via tableflip (inherited on restart).
	ln, err := upg.Listen("tcp", setup.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", setup.addr, err)
	}

	var httpLn net.Listener
	if setup.httpHandler != nil {
		httpLn, err = upg.Listen("tcp", setup.httpAddr)
		if err != nil {
			return fmt.Errorf("listen %s: %w", setup.httpAddr, err)
		}
	}

	return serveAndWait(ctx, ln, httpLn, setup, handler, cfg, registry, upg.Ready, upg.Exit(), "upgrade/signal received, shutting down")
}

// startServerSimple is the fallback when tableflip is unavailable.
// It uses plain listeners without graceful restart support.
func startServerSimple(
	ctx context.Context,
	setup *tlsSetup,
	handler http.Handler,
	cfg *Config,
	registry *Registry,
) error {
	var lc net.ListenConfig

	ln, err := lc.Listen(context.Background(), "tcp", setup.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", setup.addr, err)
	}

	var httpLn net.Listener
	if setup.httpHandler != nil {
		httpLn, err = lc.Listen(context.Background(), "tcp", setup.httpAddr)
		if err != nil {
			return fmt.Errorf("listen %s: %w", setup.httpAddr, err)
		}
	}

	return serveAndWait(ctx, ln, httpLn, setup, handler, cfg, registry, nil, signalDone(syscall.SIGINT, syscall.SIGTERM), "signal received, shutting down")
}

// serveAndWait wraps listeners with TLS if configured, starts serving, waits
// for a shutdown trigger (context cancellation, done channel, or server error),
// then performs graceful shutdown.
func serveAndWait(
	ctx context.Context,
	ln net.Listener,
	httpLn net.Listener,
	setup *tlsSetup,
	handler http.Handler,
	cfg *Config,
	registry *Registry,
	onReady func() error,
	done <-chan struct{},
	doneMsg string,
) error {
	if setup.tlsConfig != nil {
		ln = tls.NewListener(ln, setup.tlsConfig)
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errChan := make(chan error, 2)

	var httpServer *http.Server
	if httpLn != nil {
		httpServer = &http.Server{
			Handler:           setup.httpHandler,
			ReadHeaderTimeout: 10 * time.Second,
		}

		go func() {
			slog.Info("http redirect server listening", "addr", setup.httpAddr)
			if serveErr := httpServer.Serve(httpLn); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				errChan <- serveErr
			}
		}()
	}

	go func() {
		slog.Info("server listening", "addr", ln.Addr(), "tls", setup.tlsConfig != nil)
		if serveErr := server.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errChan <- serveErr
		}
	}()

	if onReady != nil {
		if err := onReady(); err != nil {
			return fmt.Errorf("ready callback: %w", err)
		}
	}

	select {
	case <-ctx.Done():
		slog.Info("context cancelled, shutting down")
	case <-done:
		slog.Info(doneMsg)
	case err := <-errChan:
		return err
	}

	return shutdownServers(cfg, registry, server, httpServer)
}

// signalDone returns a channel that is closed when one of the given signals
// is received.
func signalDone(signals ...os.Signal) <-chan struct{} {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, signals...)

	done := make(chan struct{})

	go func() {
		<-sig
		close(done)
	}()

	return done
}
