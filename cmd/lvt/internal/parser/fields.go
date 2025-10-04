package parser

import (
	"fmt"
	"strings"
)

type Field struct {
	Name    string
	Type    string
	GoType  string
	SQLType string
}

// ParseFields parses field definitions in the format "name:type name2:type2"
func ParseFields(args []string) ([]Field, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no fields provided")
	}

	var fields []Field
	for _, arg := range args {
		parts := strings.Split(arg, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid field format '%s', expected 'name:type'", arg)
		}

		name := strings.TrimSpace(parts[0])
		typ := strings.TrimSpace(parts[1])

		if name == "" {
			return nil, fmt.Errorf("field name cannot be empty")
		}
		if typ == "" {
			return nil, fmt.Errorf("field type cannot be empty for field '%s'", name)
		}

		// Validate type
		goType, sqlType, err := MapType(typ)
		if err != nil {
			return nil, fmt.Errorf("field '%s': %w", name, err)
		}

		fields = append(fields, Field{
			Name:    name,
			Type:    typ,
			GoType:  goType,
			SQLType: sqlType,
		})
	}

	return fields, nil
}

// MapType maps a user-provided type to Go and SQL types
func MapType(typ string) (goType, sqlType string, err error) {
	switch strings.ToLower(typ) {
	case "string", "str", "text":
		return "string", "TEXT", nil
	case "int", "integer":
		return "int64", "INTEGER", nil
	case "bool", "boolean":
		return "bool", "BOOLEAN", nil
	case "float", "float64", "decimal":
		return "float64", "REAL", nil
	case "time", "datetime", "timestamp":
		return "time.Time", "DATETIME", nil
	default:
		return "", "", fmt.Errorf("unsupported type '%s' (supported: string, int, bool, float, time)", typ)
	}
}

// FieldsToGoStruct generates Go struct field declarations
func FieldsToGoStruct(fields []Field) string {
	var sb strings.Builder
	for _, f := range fields {
		// Capitalize first letter for export
		fieldName := strings.Title(f.Name)
		sb.WriteString(fmt.Sprintf("\t%s %s `json:\"%s\"`\n", fieldName, f.GoType, f.Name))
	}
	return sb.String()
}

// FieldsToSQLColumns generates SQL column definitions
func FieldsToSQLColumns(fields []Field) string {
	var sb strings.Builder
	for i, f := range fields {
		sb.WriteString(fmt.Sprintf("  %s %s NOT NULL", f.Name, f.SQLType))
		if i < len(fields)-1 {
			sb.WriteString(",\n")
		}
	}
	return sb.String()
}
