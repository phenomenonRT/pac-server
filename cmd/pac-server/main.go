package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pac-server/internal/config"
	"pac-server/internal/server"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfgPath := configPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	app := server.New(cfg, cfgPath, logger)
	httpServer := &http.Server{
		Handler:           app.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	addrs := cfg.Addresses()
	listeners := make([]net.Listener, 0, len(addrs))
	for _, addr := range addrs {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			logger.Error("listen", "addr", addr, "error", err)
			os.Exit(1)
		}
		listeners = append(listeners, ln)
	}

	serveErrs := make(chan error, len(listeners))
	for i, ln := range listeners {
		addr := addrs[i]
		ln := ln
		go func() {
			logger.Info("pac server started", "addr", addr, "pac", "/proxy.pac")
			if err := httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serveErrs <- err
				return
			}
			serveErrs <- nil
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
	case err := <-serveErrs:
		if err != nil {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("pac server stopped")
}

func configPath() string {
	if path := os.Getenv("PAC_CONFIG"); path != "" {
		return path
	}
	return "config.json"
}
