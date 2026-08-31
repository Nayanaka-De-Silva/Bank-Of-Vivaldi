package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"bank-of-vivaldi/internal/application"
	"bank-of-vivaldi/internal/config"
	"bank-of-vivaldi/internal/infrastructure/httpapi"
	"bank-of-vivaldi/internal/infrastructure/httpui"
	"bank-of-vivaldi/internal/infrastructure/postgres"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.PingTimeout)
	defer cancel()

	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	if cfg.AutoMigrate {
		if err := postgres.Migrate(context.Background(), db); err != nil {
			log.Fatalf("migrate: %v", err)
		}
	}

	store := postgres.NewStore(db)
	service := application.NewService(store)

	uiServer, err := httpui.NewServer(service)
	if err != nil {
		log.Fatalf("build server: %v", err)
	}
	apiServer := httpapi.NewServer(service)

	// The JSON API and the HTML UI share one service and one database; only
	// the transport layer differs. /api/v1 has no authentication of its own —
	// see internal/infrastructure/httpapi's package doc and README.md's API
	// section for why that's a deliberate v1 limitation.
	root := http.NewServeMux()
	root.Handle("/api/v1/", http.StripPrefix("/api/v1", apiServer.Routes()))
	root.Handle("/", uiServer.Routes())

	httpServer := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: root,
	}

	go func() {
		log.Printf("bank-of-vivaldi listening on %s", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}
