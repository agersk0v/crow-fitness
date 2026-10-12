package main

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

	"github.com/agersk0v/crow-fitness/backend/internal/config"
	"github.com/agersk0v/crow-fitness/backend/internal/health"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		slog.Error("API stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return errors.New("invalid database configuration")
	}
	poolConfig.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return errors.New("could not create database connection pool")
	}
	defer pool.Close()

	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           health.NewHandler(pool),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	slog.Info("API listening", "port", cfg.HTTPPort)
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			return err
		}
	}
	return nil
}

func healthcheck() error {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	client := http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://127.0.0.1:" + port + "/api/health")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health status %d", response.StatusCode)
	}
	return nil
}
