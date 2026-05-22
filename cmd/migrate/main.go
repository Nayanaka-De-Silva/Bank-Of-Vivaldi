package main

import (
	"context"
	"log"

	"bank-of-vivaldi/internal/config"
	"bank-of-vivaldi/internal/infrastructure/postgres"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	if err := postgres.Migrate(ctx, db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	log.Println("migrations complete")
}
