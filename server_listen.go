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

	if setup.tlsConfig != nil {
		ln = tls.NewListener(ln, setup.tlsConfig)
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errChan := make(chan error, 2)

	// Start ACME HTTP challenge/redirect server if needed.
	var httpServer *http.Server
	if setup.httpHandler != nil {
		httpLn, listenErr := upg.Listen("tcp", setup.httpAddr)
		if listenErr != nil {
			return fmt.Errorf("listen %s: %w", setup.httpAddr, listenErr)
		}

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

	// Signal ready — allows the parent process (if any) to stop accepting.
	if err := upg.Ready(); err != nil {
		return fmt.Errorf("tableflip ready: %w", err)
	}

	select {
	case <-ctx.Done():
		slog.Info("context cancelled, shutting down")
	case <-upg.Exit():
		slog.Info("upgrade/signal received, shutting down")
	case err := <-errChan:
		return err
	}

	return shutdownServers(cfg, registry, server, httpServer)
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

	if setup.tlsConfig != nil {
		ln = tls.NewListener(ln, setup.tlsConfig)
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errChan := make(chan error, 2)

	var httpServer *http.Server
	if setup.httpHandler != nil {
		httpLn, listenErr := lc.Listen(context.Background(), "tcp", setup.httpAddr)
		if listenErr != nil {
			return fmt.Errorf("listen %s: %w", setup.httpAddr, listenErr)
		}

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
		slog.Info("server listening", "addr", setup.addr, "tls", setup.tlsConfig != nil)
		if serveErr := server.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errChan <- serveErr
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		slog.Info("context cancelled, shutting down")
	case <-quit:
		slog.Info("signal received, shutting down")
	case err := <-errChan:
		return err
	}

	return shutdownServers(cfg, registry, server, httpServer)
}
