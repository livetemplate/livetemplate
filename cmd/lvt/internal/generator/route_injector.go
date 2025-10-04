package generator

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// RouteInfo contains information about a route to be injected
type RouteInfo struct {
	Path        string // e.g., "/users"
	PackageName string // e.g., "users"
	HandlerCall string // e.g., "users.Handler(queries)" or "counter.Handler()"
	ImportPath  string // e.g., "myapp/internal/app/users"
}

// InjectRoute adds a route and import to main.go
func InjectRoute(mainGoPath string, route RouteInfo) error {
	// Read the file
	file, err := os.Open(mainGoPath)
	if err != nil {
		return fmt.Errorf("failed to open main.go: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read main.go: %w", err)
	}

	// Check if route already exists (check for the route pattern)
	routePattern := fmt.Sprintf(`http.Handle("%s", %s)`, route.Path, route.HandlerCall)
	for _, line := range lines {
		if strings.Contains(line, routePattern) {
			// Route already exists, don't add again
			return nil
		}
	}

	// Add import if not present
	importLine := fmt.Sprintf(`	"%s"`, route.ImportPath)
	importExists := false
	importInsertIndex := -1

	for i, line := range lines {
		// Check if import exists (check if line contains the import path)
		if strings.Contains(line, `"`+route.ImportPath+`"`) {
			importExists = true
		}

		// Find where to insert import (after internal/database import)
		if strings.Contains(line, "/internal/database") {
			importInsertIndex = i + 1
		}
	}

	if !importExists && importInsertIndex != -1 {
		// Insert import
		lines = insertLine(lines, importInsertIndex, importLine)
	}

	// Find where to insert route (after TODO comment or before port := line)
	routeInsertIndex := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Look for TODO comment about routes
		if strings.Contains(line, "TODO: Add routes here") {
			// Insert after the example comment
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "" || !strings.HasPrefix(strings.TrimSpace(lines[j]), "//") {
					routeInsertIndex = j
					break
				}
			}
			break
		}

		// Fallback: insert before port := line
		if strings.Contains(trimmed, `port := os.Getenv("PORT")`) {
			routeInsertIndex = i
			// Add blank line before if needed
			if i > 0 && strings.TrimSpace(lines[i-1]) != "" {
				lines = insertLine(lines, i, "")
				routeInsertIndex = i + 1
			}
			break
		}
	}

	if routeInsertIndex == -1 {
		return fmt.Errorf("could not find appropriate location to inject route")
	}

	// Insert route (with proper indentation)
	routeLine := fmt.Sprintf(`	http.Handle("%s", %s)`, route.Path, route.HandlerCall)
	lines = insertLine(lines, routeInsertIndex, routeLine)

	// Write back
	output := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(mainGoPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write main.go: %w", err)
	}

	return nil
}

// insertLine inserts a line at the given index
func insertLine(lines []string, index int, line string) []string {
	result := make([]string, 0, len(lines)+1)
	result = append(result, lines[:index]...)
	result = append(result, line)
	result = append(result, lines[index:]...)
	return result
}
