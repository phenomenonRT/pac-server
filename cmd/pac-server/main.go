package main

import (
	"context"
	"errors"
	"log/slog"
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
		Addr:              cfg.Address(),
		Handler:           app.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("pac server started", "addr", cfg.Address(), "pac", "/proxy.pac")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

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
