package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"barclays/database/migrations"
	"barclays/internal/server"
	"barclays/internal/storage"
	_ "github.com/glebarez/go-sqlite"
	"github.com/jmoiron/sqlx"
	migrate "github.com/rubenv/sql-migrate"
)

func main() {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	mg := migrations.GetMigrationSource()
	migrate.SetTable("migrations")
	_, err = migrate.Exec(db.DB, "sqlite3", mg, migrate.Up)
	if err != nil {
		log.Fatal(fmt.Errorf("migrations failed: %w", err))
	}

	repo := storage.New(db)
	srv := server.New(server.Dependencies{
		Repository: repo,
	})

	port := 8080
	if p := os.Getenv("SERVER_PORT"); p != "" {
		parsed, err := strconv.Atoi(p)
		if err != nil {
			log.Fatalf("invalid SERVER_PORT %q: %v", p, err)
		}
		port = parsed
	}
	srv.Start(port)
}
