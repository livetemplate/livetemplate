package ui

import "strings"

// InferType suggests a type based on the field name
// Uses common naming patterns to intelligently guess the most appropriate type
func InferType(fieldName string) string {
	lower := strings.ToLower(fieldName)

	// Exact matches for common field names
	exactMatches := map[string]string{
		// String fields
		"name":        "string",
		"title":       "string",
		"description": "string",
		"content":     "string",
		"email":       "string",
		"username":    "string",
		"password":    "string",
		"url":         "string",
		"slug":        "string",
		"code":        "string",
		"token":       "string",
		"key":         "string",
		"address":     "string",
		"city":        "string",
		"country":     "string",
		"phone":       "string",
		"status":      "string",
		"type":        "string",

		// Integer fields
		"age":      "int",
		"count":    "int",
		"quantity": "int",
		"views":    "int",
		"likes":    "int",
		"score":    "int",
		"rank":     "int",
		"level":    "int",
		"year":     "int",
		"month":    "int",
		"day":      "int",

		// Float fields
		"price":     "float",
		"amount":    "float",
		"rating":    "float",
		"lat":       "float",
		"lng":       "float",
		"latitude":  "float",
		"longitude": "float",

		// Boolean fields
		"enabled":   "bool",
		"active":    "bool",
		"published": "bool",
		"verified":  "bool",
		"approved":  "bool",
		"deleted":   "bool",
		"hidden":    "bool",
		"visible":   "bool",
		"public":    "bool",
		"private":   "bool",

		// Time fields
		"created_at":   "time",
		"updated_at":   "time",
		"deleted_at":   "time",
		"published_at": "time",
		"started_at":   "time",
		"ended_at":     "time",
		"expires_at":   "time",
	}

	// Check for exact match
	if typ, ok := exactMatches[lower]; ok {
		return typ
	}

	// Pattern matching for suffixes
	if strings.HasSuffix(lower, "_at") || strings.HasSuffix(lower, "_date") || strings.HasSuffix(lower, "_time") {
		return "time"
	}

	if strings.HasSuffix(lower, "_count") || strings.HasSuffix(lower, "_number") || strings.HasSuffix(lower, "_index") {
		return "int"
	}

	if strings.HasSuffix(lower, "_price") || strings.HasSuffix(lower, "_amount") || strings.HasSuffix(lower, "_rate") {
		return "float"
	}

	if strings.HasPrefix(lower, "is_") || strings.HasPrefix(lower, "has_") || strings.HasPrefix(lower, "can_") {
		return "bool"
	}

	// Pattern matching for contains
	if strings.Contains(lower, "email") {
		return "string"
	}

	if strings.Contains(lower, "url") {
		return "string"
	}

	if strings.Contains(lower, "price") || strings.Contains(lower, "amount") {
		return "float"
	}

	// Default to string for unknown fields
	return "string"
}

// ParseFieldInput parses user input which can be either:
// "fieldname" -> uses inferred type
// "fieldname:type" -> uses specified type
func ParseFieldInput(input string) (name, typ string) {
	parts := strings.SplitN(input, ":", 2)
	name = strings.TrimSpace(parts[0])

	if len(parts) == 2 {
		// User specified a type
		typ = strings.TrimSpace(parts[1])
	} else {
		// Infer type from name
		typ = InferType(name)
	}

	return name, typ
}
