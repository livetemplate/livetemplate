package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
	_ "github.com/mattn/go-sqlite3"
)

// E2E test timing constants
// These constants define wait times for various browser operations to make tests
// more maintainable and easier to tune for different environments
const (
	// shortDelay is used for brief pauses between operations (e.g., after clicking buttons)
	shortDelay = 500 * time.Millisecond

	// standardDelay is used for typical operations (e.g., waiting for navigation)
	standardDelay = 1 * time.Second

	// formSubmitDelay is used after form submissions to wait for processing and WebSocket updates
	formSubmitDelay = 2 * time.Second

	// modalAnimationDelay is used to wait for modal open/close animations to complete
	modalAnimationDelay = 3 * time.Second

	// quickPollDelay is used for rapid polling checks (e.g., waiting for server readiness)
	quickPollDelay = 200 * time.Millisecond
)

// waitForCondition polls a JavaScript condition until it returns true or times out
// This is more reliable than manual retry loops with fixed delays
func waitForCondition(ctx context.Context, jsCondition string, timeout time.Duration, pollInterval time.Duration) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("timeout waiting for condition: %s", jsCondition)
			case <-ticker.C:
				var result bool
				if err := chromedp.Evaluate(jsCondition, &result).Do(ctx); err != nil {
					continue
				}
				if result {
					return nil
				}
			}
		}
	}
}

// seedTestData seeds test data into SQLite database using parameterized queries
// This is safer than string concatenation and prevents SQL injection
func seedTestData(dbPath string, queries []struct {
	SQL  string
	Args []interface{}
}) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Begin transaction for atomicity
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	for _, q := range queries {
		if _, err := tx.Exec(q.SQL, q.Args...); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
