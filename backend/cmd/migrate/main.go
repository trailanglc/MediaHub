package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	direction := flag.String("direction", "", "migration direction: up or down (positional arg also accepted)")
	flag.Parse()

	dir := *direction
	args := flag.Args()
	if dir == "" && len(args) > 0 {
		dir = args[0]
	}
	if dir == "" {
		dir = "up"
	}

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN is required")
	}

	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		log.Fatalf("migrate init: %v", err)
	}
	defer m.Close()

	switch dir {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate up: %v", err)
		}
		fmt.Println("migrate: up OK")
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate down: %v", err)
		}
		fmt.Println("migrate: down OK")
	default:
		log.Fatalf("unknown direction: %s (use up or down)", dir)
	}
}
