package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	_ = godotenv.Load()

	dsn := buildDSN()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}

	if err := runMigrations(dsn); err != nil {
		log.Fatalf("migration failed: %v", err)
	}
	log.Println("migrations completed successfully")

	addr := ":" + getEnv("PORT", "8080")
	log.Printf("server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, newRouter(db)))
}
