package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/livefir/livetemplate/examples/todos/db"
	_ "modernc.org/sqlite"
)

var (
	database *sql.DB
	queries  *db.Queries
)

// InitDB initializes the SQLite database and runs migrations
func InitDB(dbPath string) (*db.Queries, error) {
	var err error

	// Open database connection
	database, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations (create tables)
	if err := runMigrations(database); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create queries instance
	queries = db.New(database)

	log.Printf("Database initialized at: %s", dbPath)
	return queries, nil
}

// runMigrations creates the database schema
func runMigrations(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS todos (
  id TEXT PRIMARY KEY,
  text TEXT NOT NULL,
  completed BOOLEAN NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_todos_created_at ON todos(created_at);
CREATE INDEX IF NOT EXISTS idx_todos_completed ON todos(completed);
`
	_, err := db.Exec(schema)
	return err
}

// CloseDB closes the database connection
func CloseDB() {
	if database != nil {
		if err := database.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		} else {
			log.Println("Database connection closed")
		}
	}
}

// GetDBPath returns the database file path, using `:memory:` for tests
func GetDBPath() string {
	// Check if we're running in test mode
	if os.Getenv("TEST_MODE") == "1" {
		return ":memory:"
	}
	return "todos.db"
}
