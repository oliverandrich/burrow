//go:build windows

package burrow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func startServer(ctx context.Context, handler http.Handler, cfg *Config, registry *Registry) error {
	setup, err := configureTLS(cfg)
	if err != nil {
		return fmt.Errorf("configure TLS: %w", err)
	}

	server := &http.Server{
		Addr:              setup.addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		TLSConfig:         setup.tlsConfig,
	}

	errChan := make(chan error, 2)

	// Start ACME HTTP challenge/redirect server if needed.
	var httpServer *http.Server
	if setup.httpHandler != nil {
		httpServer = &http.Server{
			Addr:              setup.httpAddr,
			Handler:           setup.httpHandler,
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			slog.Info("http redirect server listening", "addr", setup.httpAddr)
			if listenErr := httpServer.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
				errChan <- listenErr
			}
		}()
	}

	go func() {
		slog.Info("server listening", "addr", setup.addr, "tls", setup.tlsConfig != nil)
		var listenErr error
		if setup.tlsConfig != nil {
			listenErr = server.ListenAndServeTLS("", "")
		} else {
			listenErr = server.ListenAndServe()
		}
		if listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			errChan <- listenErr
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
