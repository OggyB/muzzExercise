package main

import (
	"github.com/oggyb/muzz-exercise/internal/config"
	"log"

	"github.com/oggyb/muzz-exercise/internal/db"
)

func main() {
	// Load configuration
	cfg := config.New()

	database, err := db.NewDB(cfg)
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}

	if err := db.SeedTestData(database); err != nil {
		log.Fatalf("failed to seed: %v", err)
	}

	log.Println("Seeding completed.")
}
