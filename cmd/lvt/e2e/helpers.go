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

	// quickPollDelay is used for rapid polling checks (e.g., waiting for server readiness)
	quickPollDelay = 200 * time.Millisecond
)

// waitForCondition polls a JavaScript condition until it returns true or times out
// This is more reliable than manual retry loops with fixed delays
func waitForCondition(ctx context.Context, jsCondition string, timeout time.Duration, pollInterval time.Duration) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		// Check if parent context already has a deadline
		deadline, hasDeadline := ctx.Deadline()

		// Calculate effective timeout (minimum of requested and remaining parent timeout)
		effectiveTimeout := timeout
		if hasDeadline {
			remaining := time.Until(deadline)
			if remaining < timeout {
				effectiveTimeout = remaining
			}
		}

		// Only create new timeout context if we have time remaining
		if effectiveTimeout <= 0 {
			return fmt.Errorf("parent context already expired while waiting for condition: %s", jsCondition)
		}

		ctx, cancel := context.WithTimeout(ctx, effectiveTimeout)
		defer cancel()

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// Provide better error message with actual timeout used
				return fmt.Errorf("timeout (%.1fs) waiting for condition: %s (error: %v)", effectiveTimeout.Seconds(), jsCondition, ctx.Err())
			case <-ticker.C:
				var result bool
				if err := chromedp.Evaluate(jsCondition, &result).Do(ctx); err != nil {
					// Continue polling even on evaluation errors (DOM might not be ready)
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
