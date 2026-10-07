package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	store, err := initDB(ctx)
	if err != nil {
		return err
	}
	defer store.pool.Close()
	startupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := store.Health(startupCtx); err != nil {
		log.Print("DB not reachable at start; readiness will retry")
	}
	cancel()
	srv := &http.Server{
		Addr: ":8000", Handler: newHandler(store),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() {
		log.Print("notes-api listening on :8000")
		serverErr <- srv.ListenAndServe()
	}()
	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		log.Print("SIGTERM received, shutting down gracefully...")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close()
		return err
	}
	log.Print("server stopped")
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
