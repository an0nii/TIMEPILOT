package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timepilot/internal/config"
	"timepilot/internal/httpapi"
	"timepilot/internal/migrate"
	"timepilot/internal/planner"
	"timepilot/internal/store"
)

var version = "dev"

func main() {
	logger := newLogger(os.Stdout, version)
	processType := "server"
	if len(os.Args) > 1 {
		processType = os.Args[1]
	}
	logger = logger.With("process.type", processType)
	if err := run(os.Args[1:], logger); err != nil {
		logger.Error("process stopped", "error", err)
		os.Exit(1)
	}
}

func newLogger(output io.Writer, serviceVersion string) *slog.Logger {
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) != 0 {
				return attr
			}
			switch attr.Key {
			case slog.TimeKey:
				attr.Key = "timestamp"
			case slog.LevelKey:
				attr.Key = "level"
				attr.Value = slog.StringValue(strings.ToLower(attr.Value.String()))
			case slog.MessageKey:
				attr.Key = "message"
			}
			return attr
		},
	})
	return slog.New(handler).With(
		"service.name", "timepilot",
		"service.version", serviceVersion,
	)
}

func run(args []string, logger *slog.Logger) error {
	command := "server"
	if len(args) > 0 {
		command = args[0]
	}

	if command == "healthcheck" {
		url := "http://localhost:8080/readyz"
		if len(args) > 1 {
			url = args[1]
		}
		return checkHealth(url)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create database pool: %w", err)
	}
	defer pool.Close()

	switch command {
	case "server":
		aiPlanner, err := planner.New(cfg.AI)
		if err != nil {
			return fmt.Errorf("configure AI planner: %w", err)
		}
		return serve(ctx, cfg, store.NewPostgres(pool), aiPlanner, logger)
	case "migrate":
		logger.Info("applying database migrations")
		if err := migrate.Run(ctx, pool); err != nil {
			return err
		}
		logger.Info("database migrations applied")
		return nil
	default:
		return fmt.Errorf("unknown command %q; use server, migrate or healthcheck", command)
	}
}

func serve(ctx context.Context, cfg config.Config, database *store.Postgres, aiPlanner planner.Planner, logger *slog.Logger) error {
	startupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := database.Ping(startupCtx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.New(database, aiPlanner, logger, version),
		ReadHeaderTimeout: cfg.ReadTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server started", "port", cfg.Port, "version", version)
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		logger.Info("server stopped")
		return nil
	}
}

func checkHealth(url string) error {
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("health request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned %s", response.Status)
	}
	return nil
}
