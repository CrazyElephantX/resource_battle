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

	"resource_battle/internal/app"
	"resource_battle/internal/config"
	"resource_battle/internal/db"
	"resource_battle/internal/migrate"
)

func main() {
	cfg := config.FromEnv()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer pool.Close()

	if err := migrate.Up(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// Загрузка начальных задач, если таблица пуста
	imported, err := app.SeedTasksIfEmpty(ctx, pool)
	if err != nil {
		log.Printf("WARNING: seed tasks: %v", err)
	} else if imported > 0 {
		log.Printf("seed tasks: imported %d tasks", imported)
	}

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           app.New(pool),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
