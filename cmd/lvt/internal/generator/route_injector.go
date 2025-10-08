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
	HandlerCall string // e.g., "users.Handler(queries)"
	ImportPath  string // e.g., "myapp/internal/app/users"
}

// InjectRoute adds an http.Handle route and import to main.go
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

	// Check if route already exists (check for the pattern, but ignore comments)
	routePattern := fmt.Sprintf(`http.Handle("%s", %s)`, route.Path, route.HandlerCall)
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		// Skip comments
		if strings.HasPrefix(trimmedLine, "//") {
			continue
		}
		if trimmedLine == routePattern || strings.Contains(line, routePattern) {
			// Route already exists, don't add again
			return nil
		}
	}

	// Add import if not present
	importLine := fmt.Sprintf(`	"%s"`, route.ImportPath)
	importExists := false
	importInsertIndex := -1
	inImportBlock := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track if we're in an import block
		if strings.HasPrefix(trimmed, "import (") {
			inImportBlock = true
		}
		if inImportBlock && trimmed == ")" {
			inImportBlock = false
		}

		// Check if import exists (only within import block)
		if inImportBlock && strings.Contains(line, `"`+route.ImportPath+`"`) {
			importExists = true
		}

		// Find where to insert import (after internal/database import, within import block)
		if inImportBlock && strings.Contains(line, "/internal/database\"") {
			importInsertIndex = i + 1
		}
	}

	if !importExists && importInsertIndex != -1 {
		// Insert import
		lines = insertLine(lines, importInsertIndex, importLine)
	} else if !importExists {
		// Fallback: if no /internal/database import found, add at end of import block
		for i, line := range lines {
			if strings.TrimSpace(line) == ")" && i > 0 {
				// Check if previous lines are imports
				prevLine := strings.TrimSpace(lines[i-1])
				if strings.Contains(prevLine, `"`) || strings.Contains(prevLine, "import") {
					// Insert before the )
					lines = insertLine(lines, i, importLine)
					break
				}
			}
		}
	}

	// Find where to insert route (after TODO comment)
	routeInsertIndex := -1
	for i, line := range lines {
		// Look for TODO comment about adding routes
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
