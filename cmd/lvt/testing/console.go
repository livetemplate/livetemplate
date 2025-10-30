package testing

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type ConsoleLogger struct {
	mu   sync.RWMutex
	logs []ConsoleLog
}

type ConsoleLog struct {
	Type    string
	Message string
	Args    []string
}

func NewConsoleLogger() *ConsoleLogger {
	return &ConsoleLogger{
		logs: make([]ConsoleLog, 0),
	}
}

func (cl *ConsoleLogger) Start(ctx context.Context) {
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			cl.mu.Lock()
			defer cl.mu.Unlock()

			log := ConsoleLog{
				Type: ev.Type.String(),
				Args: make([]string, 0, len(ev.Args)),
			}

			for _, arg := range ev.Args {
				if arg.Value != nil {
					log.Args = append(log.Args, string(arg.Value))
				}
			}

			if len(log.Args) > 0 {
				log.Message = strings.Trim(log.Args[0], "\"")
			}

			cl.logs = append(cl.logs, log)

		case *runtime.EventExceptionThrown:
			cl.mu.Lock()
			defer cl.mu.Unlock()

			msg := "unknown error"
			if ev.ExceptionDetails.Exception != nil && ev.ExceptionDetails.Exception.Description != "" {
				msg = ev.ExceptionDetails.Exception.Description
			} else if ev.ExceptionDetails.Text != "" {
				msg = ev.ExceptionDetails.Text
			}

			cl.logs = append(cl.logs, ConsoleLog{
				Type:    "error",
				Message: msg,
				Args:    []string{msg},
			})
		}
	})
}

func (cl *ConsoleLogger) GetLogs() []ConsoleLog {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	result := make([]ConsoleLog, len(cl.logs))
	copy(result, cl.logs)
	return result
}

func (cl *ConsoleLogger) GetErrors() []ConsoleLog {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	errors := make([]ConsoleLog, 0)
	for _, log := range cl.logs {
		if log.Type == "error" {
			errors = append(errors, log)
		}
	}
	return errors
}

func (cl *ConsoleLogger) GetWarnings() []ConsoleLog {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	warnings := make([]ConsoleLog, 0)
	for _, log := range cl.logs {
		if log.Type == "warning" {
			warnings = append(warnings, log)
		}
	}
	return warnings
}

func (cl *ConsoleLogger) HasErrors() bool {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	for _, log := range cl.logs {
		if log.Type == "error" {
			return true
		}
	}
	return false
}

func (cl *ConsoleLogger) HasWarnings() bool {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	for _, log := range cl.logs {
		if log.Type == "warning" {
			return true
		}
	}
	return false
}

func (cl *ConsoleLogger) Clear() {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	cl.logs = make([]ConsoleLog, 0)
}

func (cl *ConsoleLogger) FindLog(pattern string) (ConsoleLog, bool) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	for _, log := range cl.logs {
		if strings.Contains(log.Message, pattern) {
			return log, true
		}
	}
	return ConsoleLog{}, false
}

func (cl *ConsoleLogger) FilterByType(logType string) []ConsoleLog {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	filtered := make([]ConsoleLog, 0)
	for _, log := range cl.logs {
		if log.Type == logType {
			filtered = append(filtered, log)
		}
	}
	return filtered
}

func (cl *ConsoleLogger) Count() int {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	return len(cl.logs)
}

func (cl *ConsoleLogger) CountByType(logType string) int {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	count := 0
	for _, log := range cl.logs {
		if log.Type == logType {
			count++
		}
	}
	return count
}

func (cl *ConsoleLogger) Print() {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	fmt.Println("\n=== Console Logs ===")
	for i, log := range cl.logs {
		fmt.Printf("[%d] [%s] %s\n", i, log.Type, log.Message)
		if len(log.Args) > 1 {
			for j, arg := range log.Args[1:] {
				fmt.Printf("    arg[%d]: %s\n", j+1, arg)
			}
		}
	}
	fmt.Println("===================")
}

func (cl *ConsoleLogger) PrintErrors() {
	errors := cl.GetErrors()
	if len(errors) == 0 {
		return
	}

	fmt.Println("\n=== Console Errors ===")
	for i, log := range errors {
		fmt.Printf("[%d] %s\n", i, log.Message)
	}
	fmt.Println("======================")
}
