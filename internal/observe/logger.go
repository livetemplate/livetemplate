// Package observe provides structured logging and metrics using slog.
//
// This package implements production-ready observability for LiveTemplate:
// - Structured logging with context
// - Domain-specific log methods
// - Operation tracing with duration tracking
// - Integration with slog for standard Go logging
package observe

import (
	"context"
	"log/slog"
	"os"
	"time"
)

// Logger wraps slog.Logger with LiveTemplate-specific methods.
// All logs are structured and include contextual information.
type Logger struct {
	*slog.Logger
}

// NewLogger creates a configured logger.
// If handler is nil, uses JSON handler writing to stdout.
func NewLogger(level slog.Level, handler slog.Handler) *Logger {
	if handler == nil {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}
	return &Logger{Logger: slog.New(handler)}
}

// WithContext adds context fields to logger for request tracking.
// Automatically extracts trace_id, user_id, session_id, and request_id from context.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	logger := l.Logger

	// Extract trace ID first (highest priority for correlation)
	if traceID := GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}

	// Extract common fields from context
	if userID := ctx.Value("user_id"); userID != nil {
		logger = logger.With("user_id", userID)
	}
	if sessionID := ctx.Value("session_id"); sessionID != nil {
		logger = logger.With("session_id", sessionID)
	}

	return &Logger{Logger: logger}
}

// StartOperation begins tracking an operation and returns a tracker.
// Call Complete() or Fail() on the returned Operation to log completion.
func (l *Logger) StartOperation(op string) *Operation {
	return &Operation{
		logger: l.With("operation", op),
		start:  time.Now(),
		op:     op,
	}
}

// Operation tracks the execution of an operation with timing.
type Operation struct {
	logger *slog.Logger
	start  time.Time
	op     string
}

// Complete logs successful operation completion with duration.
func (o *Operation) Complete() {
	duration := time.Since(o.start)
	o.logger.Info("operation_complete",
		"duration_ms", duration.Milliseconds(),
	)
}

// Fail logs operation failure with error and duration.
func (o *Operation) Fail(err error) {
	duration := time.Since(o.start)
	o.logger.Error("operation_failed",
		"error", err,
		"duration_ms", duration.Milliseconds(),
	)
}

// Domain-specific logging methods

// TemplateParsed logs template parsing completion.
func (l *Logger) TemplateParsed(name string, duration time.Duration) {
	l.Info("template_parsed",
		"template", name,
		"duration_ms", duration.Milliseconds(),
	)
}

// TreeBuilt logs tree building completion.
func (l *Logger) TreeBuilt(dataType string, duration time.Duration) {
	l.Debug("tree_built",
		"data_type", dataType,
		"duration_ms", duration.Milliseconds(),
	)
}

// TreeDiffed logs tree diffing completion.
func (l *Logger) TreeDiffed(changesCount int, duration time.Duration) {
	l.Debug("tree_diffed",
		"changes", changesCount,
		"duration_ms", duration.Milliseconds(),
	)
}

// Rendered logs rendering completion.
func (l *Logger) Rendered(format string, bytes int, duration time.Duration) {
	l.Debug("rendered",
		"format", format,
		"bytes", bytes,
		"duration_ms", duration.Milliseconds(),
	)
}

// ActionReceived logs incoming action.
func (l *Logger) ActionReceived(action string, store string) {
	l.Info("action_received",
		"action", action,
		"store", store,
	)
}

// WebSocketConnected logs WebSocket connection.
func (l *Logger) WebSocketConnected(userID, groupID, remoteAddr string) {
	l.Info("websocket_connected",
		"user_id", userID,
		"group_id", groupID,
		"remote_addr", remoteAddr,
	)
}

// WebSocketDisconnected logs WebSocket disconnection.
func (l *Logger) WebSocketDisconnected(userID, groupID string, duration time.Duration) {
	l.Info("websocket_disconnected",
		"user_id", userID,
		"group_id", groupID,
		"duration_seconds", duration.Seconds(),
	)
}

// BroadcastSent logs broadcast completion.
func (l *Logger) BroadcastSent(groupID string, recipientCount int) {
	l.Info("broadcast_sent",
		"group_id", groupID,
		"recipients", recipientCount,
	)
}

// ErrorEncountered logs an error with context.
func (l *Logger) ErrorEncountered(component string, err error) {
	l.Error("error_encountered",
		"component", component,
		"error", err,
	)
}
