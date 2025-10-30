package testing

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"
)

type ServerLogger struct {
	mu       sync.RWMutex
	logs     []string
	reader   *io.PipeReader
	writer   *io.PipeWriter
	stopChan chan struct{}
}

func NewServerLogger() *ServerLogger {
	pr, pw := io.Pipe()
	return &ServerLogger{
		logs:     make([]string, 0),
		reader:   pr,
		writer:   pw,
		stopChan: make(chan struct{}),
	}
}

func (sl *ServerLogger) Start() {
	go func() {
		scanner := bufio.NewScanner(sl.reader)
		for scanner.Scan() {
			line := scanner.Text()
			sl.mu.Lock()
			sl.logs = append(sl.logs, line)
			sl.mu.Unlock()
		}
	}()
}

func (sl *ServerLogger) Stop() {
	close(sl.stopChan)
	sl.writer.Close()
	sl.reader.Close()
}

func (sl *ServerLogger) Writer() io.Writer {
	return sl.writer
}

func (sl *ServerLogger) GetLogs() []string {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	result := make([]string, len(sl.logs))
	copy(result, sl.logs)
	return result
}

func (sl *ServerLogger) Clear() {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	sl.logs = make([]string, 0)
}

func (sl *ServerLogger) FindLog(pattern string) (string, bool) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	for _, log := range sl.logs {
		if strings.Contains(log, pattern) {
			return log, true
		}
	}
	return "", false
}

func (sl *ServerLogger) FindLogs(pattern string) []string {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	matches := make([]string, 0)
	for _, log := range sl.logs {
		if strings.Contains(log, pattern) {
			matches = append(matches, log)
		}
	}
	return matches
}

func (sl *ServerLogger) HasLog(pattern string) bool {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	for _, log := range sl.logs {
		if strings.Contains(log, pattern) {
			return true
		}
	}
	return false
}

func (sl *ServerLogger) Count() int {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	return len(sl.logs)
}

func (sl *ServerLogger) CountMatching(pattern string) int {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	count := 0
	for _, log := range sl.logs {
		if strings.Contains(log, pattern) {
			count++
		}
	}
	return count
}

func (sl *ServerLogger) GetLastN(n int) []string {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	if n <= 0 || len(sl.logs) == 0 {
		return []string{}
	}

	if n >= len(sl.logs) {
		result := make([]string, len(sl.logs))
		copy(result, sl.logs)
		return result
	}

	result := make([]string, n)
	copy(result, sl.logs[len(sl.logs)-n:])
	return result
}

func (sl *ServerLogger) Print() {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	fmt.Println("\n=== Server Logs ===")
	for i, log := range sl.logs {
		fmt.Printf("[%d] %s\n", i, log)
	}
	fmt.Println("===================")
}

func (sl *ServerLogger) PrintLast(n int) {
	logs := sl.GetLastN(n)
	if len(logs) == 0 {
		return
	}

	fmt.Printf("\n=== Last %d Server Logs ===\n", n)
	for i, log := range logs {
		fmt.Printf("[%d] %s\n", i, log)
	}
	fmt.Println("============================")
}

func (sl *ServerLogger) PrintMatching(pattern string) {
	matches := sl.FindLogs(pattern)
	if len(matches) == 0 {
		return
	}

	fmt.Printf("\n=== Server Logs Matching '%s' ===\n", pattern)
	for i, log := range matches {
		fmt.Printf("[%d] %s\n", i, log)
	}
	fmt.Println("=====================================")
}
