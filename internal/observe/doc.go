// Package observe provides structured logging and metrics for LiveTemplate.
//
// # Structured Logging
//
// All logging is done via slog with structured fields:
//
//	logger := observe.NewLogger(slog.LevelInfo, nil)
//	logger.TemplateParsed("todos", 5*time.Millisecond)
//	logger.ActionReceived("add_todo", "todos")
//
// Logs include contextual information (user_id, session_id, request_id)
// when using WithContext:
//
//	ctx := context.WithValue(ctx, "user_id", "user-123")
//	contextLogger := logger.WithContext(ctx)
//	contextLogger.TreeBuilt("*TodoState", 3*time.Millisecond)
//
// # Operation Tracking
//
// Track operations with automatic duration logging:
//
//	op := logger.StartOperation("template_execute")
//	defer op.Complete()
//	// ... do work ...
//
// # Metrics
//
// Track operational metrics with automatic periodic emission:
//
//	metrics := observe.NewMetrics(slog.Default())
//	go metrics.EmitPeriodically(60 * time.Second)
//
//	metrics.ActionProcessed()
//	metrics.TemplateExecuted(5 * time.Millisecond)
//
// Metrics are emitted as structured logs every interval.
//
// # Integration
//
// Observability is integrated via Config:
//
//	cfg := livetemplate.Config{
//	    Logger: slog.New(handler),
//	    LogLevel: slog.LevelInfo,
//	    MetricsEnabled: true,
//	    MetricsInterval: 60 * time.Second,
//	}
//
// See OBSERVABILITY.md in docs/ for complete guide.
package observe
