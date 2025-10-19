package seeder

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	defaultDBPath = "app.db"
	testIDPrefix  = "test-seed-%"
)

// Seeder handles database seeding operations
type Seeder struct {
	db     *sql.DB
	dbPath string
}

// New creates a new Seeder instance
func New() (*Seeder, error) {
	dbPath, err := findDatabasePath()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Seeder{
		db:     db,
		dbPath: dbPath,
	}, nil
}

// Close closes the database connection
func (s *Seeder) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Seed generates and inserts N rows of test data for the given table
func (s *Seeder) Seed(table TableSchema, count int) error {
	fmt.Printf("Seeding %s with %d rows...\n", table.Name, count)

	// Prepare column names and placeholders for INSERT
	var columns []string
	var placeholders []string

	for _, col := range table.Columns {
		columns = append(columns, col.Name)
		placeholders = append(placeholders, "?")
	}

	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table.Name,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	// Begin transaction for better performance
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert rows
	for i := 0; i < count; i++ {
		values := s.generateRow(table, i)

		if _, err := stmt.Exec(values...); err != nil {
			return fmt.Errorf("failed to insert row %d: %w", i+1, err)
		}

		// Show progress
		if (i+1)%10 == 0 || i+1 == count {
			fmt.Printf("  Progress: %d/%d\n", i+1, count)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("✅ Successfully seeded %d rows into %s\n", count, table.Name)
	return nil
}

// generateRow generates a single row of data
func (s *Seeder) generateRow(table TableSchema, index int) []interface{} {
	var values []interface{}

	for _, col := range table.Columns {
		var value interface{}

		// Handle special columns
		switch strings.ToLower(col.Name) {
		case "id":
			value = GenerateID(index)
		case "created_at", "updated_at":
			value = GenerateCreatedAt()
		default:
			value = GenerateValue(col)
		}

		values = append(values, value)
	}

	return values
}

// Cleanup removes all test-seeded data from the given table
func (s *Seeder) Cleanup(tableName string) error {
	fmt.Printf("Cleaning up test data from %s...\n", tableName)

	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE id LIKE ?", tableName)

	result, err := s.db.Exec(deleteSQL, testIDPrefix)
	if err != nil {
		return fmt.Errorf("failed to delete test data: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		fmt.Printf("ℹ️  No test data found in %s\n", tableName)
	} else {
		fmt.Printf("✅ Removed %d test record(s) from %s\n", rowsAffected, tableName)
	}

	return nil
}

// CountTestRecords counts the number of test-seeded records in a table
func (s *Seeder) CountTestRecords(tableName string) (int, error) {
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id LIKE ?", tableName)

	var count int
	err := s.db.QueryRow(countSQL, testIDPrefix).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count test records: %w", err)
	}

	return count, nil
}

// findDatabasePath locates the SQLite database file
func findDatabasePath() (string, error) {
	// Try current directory first
	if _, err := os.Stat(defaultDBPath); err == nil {
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
		}

		// Move up one directory
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			// Reached root
			break
		}
		currentDir = parent
	}

	return "", fmt.Errorf("database not found (looking for %s). Run this command from your project root.", defaultDBPath)
}
