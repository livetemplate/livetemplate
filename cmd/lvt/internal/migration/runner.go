package migration

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

const (
	defaultDBPath        = "app.db"
	defaultMigrationsDir = "internal/database/migrations"
	migrationsTableName  = "goose_db_version"
)

// Runner wraps goose for migration operations
type Runner struct {
	db            *sql.DB
	migrationsDir string
}

// New creates a new migration runner
// It auto-detects the database path and migrations directory
func New() (*Runner, error) {
	// Find migrations directory
	migrationsDir, err := findMigrationsDir()
	if err != nil {
		return nil, fmt.Errorf("migrations directory not found: %w", err)
	}

	// Find database file
	dbPath, err := findDatabasePath()
	if err != nil {
		return nil, fmt.Errorf("database not found: %w", err)
	}

	// Open database connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set goose dialect for SQLite
	if err := goose.SetDialect("sqlite3"); err != nil {
		return nil, fmt.Errorf("failed to set dialect: %w", err)
	}

	return &Runner{
		db:            db,
		migrationsDir: migrationsDir,
	}, nil
}

// Close closes the database connection
func (r *Runner) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// Up runs all pending migrations and regenerates sqlc code
func (r *Runner) Up() error {
	if err := goose.Up(r.db, r.migrationsDir); err != nil {
		return fmt.Errorf("migration up failed: %w", err)
	}

	// Run sqlc generate after successful migration
	if err := r.runSqlcGenerate(); err != nil {
		fmt.Printf("⚠️  Warning: sqlc generate failed: %v\n", err)
		fmt.Println("   You can run it manually: cd internal/database && sqlc generate")
	}

	return nil
}

// Down rolls back the most recent migration and regenerates sqlc code
func (r *Runner) Down() error {
	if err := goose.Down(r.db, r.migrationsDir); err != nil {
		return fmt.Errorf("migration down failed: %w", err)
	}

	// Run sqlc generate after successful rollback
	if err := r.runSqlcGenerate(); err != nil {
		fmt.Printf("⚠️  Warning: sqlc generate failed: %v\n", err)
		fmt.Println("   You can run it manually: cd internal/database && sqlc generate")
	}

	return nil
}

// Status shows the status of all migrations
func (r *Runner) Status() error {
	if err := goose.Status(r.db, r.migrationsDir); err != nil {
		return fmt.Errorf("migration status failed: %w", err)
	}
	return nil
}

// Create generates a new migration file with the given name
func (r *Runner) Create(name string) error {
	// Generate timestamp-based filename
	timestamp := time.Now().Format("20060102150405")
	filename := fmt.Sprintf("%s_%s.sql", timestamp, name)
	filepath := filepath.Join(r.migrationsDir, filename)

	// Create migration file with goose format
	content := `-- +goose Up
-- +goose StatementBegin
-- Add your SQL here
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Add your SQL here
-- +goose StatementEnd
`

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create migration file: %w", err)
	}

	fmt.Printf("Created migration: %s\n", filename)
	return nil
}

// findMigrationsDir locates the migrations directory
func findMigrationsDir() (string, error) {
	// Try current directory first
	if _, err := os.Stat(defaultMigrationsDir); err == nil {
		return defaultMigrationsDir, nil
	}

	// Try walking up the directory tree
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		checkPath := filepath.Join(currentDir, defaultMigrationsDir)
		if _, err := os.Stat(checkPath); err == nil {
			return checkPath, nil
		}

		// Move up one directory
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			// Reached root
			break
		}
		currentDir = parent
	}

	return "", fmt.Errorf("migrations directory not found (looking for %s)", defaultMigrationsDir)
}

// findDatabasePath locates the SQLite database file and creates it if it doesn't exist
func findDatabasePath() (string, error) {
	// Try current directory first
	if _, err := os.Stat(defaultDBPath); err == nil {
		return defaultDBPath, nil
	} else if os.IsNotExist(err) {
		// Create the database file if it doesn't exist
		if err := createEmptyDB(defaultDBPath); err != nil {
			return "", fmt.Errorf("failed to create database: %w", err)
		}
		return defaultDBPath, nil
	}

	// Try walking up the directory tree
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		checkPath := filepath.Join(currentDir, defaultDBPath)
		if _, err := os.Stat(checkPath); err == nil {
			return checkPath, nil
		} else if os.IsNotExist(err) && currentDir == "." {
			// Create in current directory
			if err := createEmptyDB(checkPath); err != nil {
				return "", fmt.Errorf("failed to create database: %w", err)
			}
			return checkPath, nil
		}

		// Move up one directory
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			// Reached root - create in original location
			if err := createEmptyDB(defaultDBPath); err != nil {
				return "", fmt.Errorf("failed to create database: %w", err)
			}
			return defaultDBPath, nil
		}
		currentDir = parent
	}
}

// createEmptyDB creates an empty SQLite database file
func createEmptyDB(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()

	// Ping to ensure the file is created
	if err := db.Ping(); err != nil {
		return err
	}

	fmt.Printf("Created database: %s\n", path)
	return nil
}

// runSqlcGenerate runs sqlc generate in the database directory
func (r *Runner) runSqlcGenerate() error {
	// Find the internal/database directory
	dbDir := filepath.Dir(r.migrationsDir) // migrations is inside database

	fmt.Println("Generating database code with sqlc...")

	// Run sqlc generate
	cmd := exec.Command("go", "run", "github.com/sqlc-dev/sqlc/cmd/sqlc", "generate")
	cmd.Dir = dbDir
	cmd.Env = append(os.Environ(), "GOWORK=off") // Disable workspace mode for nested modules
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sqlc generate failed: %w", err)
	}

	fmt.Println("✅ Database code generated successfully!")
	return nil
}
