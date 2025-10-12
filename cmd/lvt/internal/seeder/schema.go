package seeder

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type TableSchema struct {
	Name       string
	Columns    []Column
	PrimaryKey string
	Indexes    []Index
}

type Column struct {
	Name      string
	Type      string
	Nullable  bool
	IsPrimary bool
}

type Index struct {
	Name    string
	Columns []string
}

// ParseSchema parses the schema.sql file and returns all table schemas
func ParseSchema(schemaPath string) ([]TableSchema, error) {
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	return parseSchemaContent(string(content))
}

// FindSchemaFile locates the schema.sql file
func FindSchemaFile() (string, error) {
	schemaPath := "internal/database/schema.sql"

	// Try current directory first
	if _, err := os.Stat(schemaPath); err == nil {
		return schemaPath, nil
	}

	// Try walking up the directory tree
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		checkPath := filepath.Join(currentDir, schemaPath)
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

	return "", fmt.Errorf("schema.sql not found (looking for %s)", schemaPath)
}

// parseSchemaContent parses the SQL content and extracts table schemas
func parseSchemaContent(content string) ([]TableSchema, error) {
	var tables []TableSchema

	// Remove comments
	content = removeComments(content)

	// Find all CREATE TABLE statements
	tableRegex := regexp.MustCompile(`CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s*\(((?:[^()]+|\([^)]*\))*)\)`)
	matches := tableRegex.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		tableName := match[1]
		columnsSQL := match[2]

		table := TableSchema{
			Name:    tableName,
			Columns: []Column{},
			Indexes: []Index{},
		}

		// Parse columns
		columns := parseColumns(columnsSQL)
		table.Columns = columns

		// Find primary key
		for _, col := range columns {
			if col.IsPrimary {
				table.PrimaryKey = col.Name
				break
			}
		}

		tables = append(tables, table)
	}

	// Parse indexes
	indexRegex := regexp.MustCompile(`CREATE\s+INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s+ON\s+(\w+)\s*\(([^)]+)\)`)
	indexMatches := indexRegex.FindAllStringSubmatch(content, -1)

	for _, match := range indexMatches {
		if len(match) < 4 {
			continue
		}

		indexName := match[1]
		tableName := match[2]
		columnsStr := match[3]

		// Find the table and add the index
		for i := range tables {
			if tables[i].Name == tableName {
				columns := strings.Split(columnsStr, ",")
				for j := range columns {
					columns[j] = strings.TrimSpace(columns[j])
				}

				tables[i].Indexes = append(tables[i].Indexes, Index{
					Name:    indexName,
					Columns: columns,
				})
				break
			}
		}
	}

	return tables, nil
}

// parseColumns parses column definitions from CREATE TABLE statement
func parseColumns(columnsSQL string) []Column {
	var columns []Column

	// Split by comma, but be careful of commas inside parentheses
	columnDefs := splitColumns(columnsSQL)

	for _, colDef := range columnDefs {
		colDef = strings.TrimSpace(colDef)
		if colDef == "" {
			continue
		}

		// Skip constraints like FOREIGN KEY, CHECK, etc.
		if strings.HasPrefix(strings.ToUpper(colDef), "CONSTRAINT") ||
			strings.HasPrefix(strings.ToUpper(colDef), "FOREIGN KEY") ||
			strings.HasPrefix(strings.ToUpper(colDef), "CHECK") ||
			strings.HasPrefix(strings.ToUpper(colDef), "UNIQUE") {
			continue
		}

		col := parseColumn(colDef)
		if col.Name != "" {
			columns = append(columns, col)
		}
	}

	return columns
}

// parseColumn parses a single column definition
func parseColumn(colDef string) Column {
	parts := strings.Fields(colDef)
	if len(parts) < 2 {
		return Column{}
	}

	col := Column{
		Name:     parts[0],
		Type:     strings.ToUpper(parts[1]),
		Nullable: true,
	}

	// Check for constraints
	defUpper := strings.ToUpper(colDef)
	if strings.Contains(defUpper, "PRIMARY KEY") {
		col.IsPrimary = true
		col.Nullable = false
	}
	if strings.Contains(defUpper, "NOT NULL") {
		col.Nullable = false
	}

	return col
}

// splitColumns splits column definitions by comma, respecting parentheses
func splitColumns(s string) []string {
	var result []string
	var current strings.Builder
	depth := 0

	for _, ch := range s {
		switch ch {
		case '(':
			depth++
			current.WriteRune(ch)
		case ')':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				result = append(result, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

// removeComments removes SQL comments from content
func removeComments(content string) string {
	// Remove single-line comments (-- ...)
	lineCommentRegex := regexp.MustCompile(`--[^\n]*`)
	content = lineCommentRegex.ReplaceAllString(content, "")

	// Remove multi-line comments (/* ... */)
	blockCommentRegex := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	content = blockCommentRegex.ReplaceAllString(content, "")

	return content
}

// FindTable finds a table by name (case-insensitive)
func FindTable(tables []TableSchema, name string) *TableSchema {
	nameLower := strings.ToLower(name)
	for i := range tables {
		if strings.ToLower(tables[i].Name) == nameLower {
			return &tables[i]
		}
	}
	return nil
}
