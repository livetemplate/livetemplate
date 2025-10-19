package seeder

import (
	"fmt"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

// GenerateValue generates a realistic value for a column based on its name and type
func GenerateValue(column Column) interface{} {
	// Skip generated fields
	if column.Name == "id" || column.Name == "created_at" || column.Name == "updated_at" {
		return nil
	}

	// Context-aware generation based on field name
	fieldLower := strings.ToLower(column.Name)

	// Check for specific field patterns
	switch {
	case contains(fieldLower, "email"):
		return gofakeit.Email()
	case contains(fieldLower, "name") && !contains(fieldLower, "username", "filename"):
		return gofakeit.Name()
	case contains(fieldLower, "username", "user_name"):
		return gofakeit.Username()
	case contains(fieldLower, "first_name", "firstname"):
		return gofakeit.FirstName()
	case contains(fieldLower, "last_name", "lastname"):
		return gofakeit.LastName()
	case contains(fieldLower, "phone", "mobile", "telephone"):
		return gofakeit.Phone()
	case contains(fieldLower, "address"):
		return gofakeit.Address().Address
	case contains(fieldLower, "city"):
		return gofakeit.City()
	case contains(fieldLower, "state", "province"):
		return gofakeit.State()
	case contains(fieldLower, "country"):
		return gofakeit.Country()
	case contains(fieldLower, "zip", "zipcode", "postal"):
		return gofakeit.Zip()
	case contains(fieldLower, "url", "website", "link"):
		return gofakeit.URL()
	case contains(fieldLower, "title"):
		if column.Type == "TEXT" {
			return gofakeit.JobTitle()
		}
		return gofakeit.BuzzWord() + " " + gofakeit.Noun()
	case contains(fieldLower, "job", "position", "occupation"):
		return gofakeit.JobTitle()
	case contains(fieldLower, "company", "organization"):
		return gofakeit.Company()
	case contains(fieldLower, "content", "description", "body", "bio"):
		return gofakeit.Paragraph(2, 3, 8, " ")
	case contains(fieldLower, "text", "comment", "note", "message"):
		return gofakeit.Sentence(12)
	case contains(fieldLower, "price", "amount", "cost", "fee", "salary"):
		return gofakeit.Float64Range(10, 10000)
	case contains(fieldLower, "quantity", "count", "stock"):
		return gofakeit.Number(1, 1000)
	case contains(fieldLower, "age"):
		return gofakeit.Number(18, 99)
	case contains(fieldLower, "rating", "score"):
		return gofakeit.Float64Range(1, 5)
	case contains(fieldLower, "status"):
		return gofakeit.RandomString([]string{"active", "inactive", "pending", "completed"})
	case contains(fieldLower, "category", "type"):
		return gofakeit.Word()
	case contains(fieldLower, "image", "avatar", "photo", "picture"):
		return gofakeit.URL()
	case contains(fieldLower, "color", "colour"):
		return gofakeit.Color()
	case contains(fieldLower, "uuid"):
		return gofakeit.UUID()
	case contains(fieldLower, "date", "birthday", "dob"):
		return gofakeit.Date().Format("2006-01-02")
	}

	// Fall back to type-based generation
	return generateByType(column.Type)
}

// generateByType generates a value based on SQL type
func generateByType(sqlType string) interface{} {
	typeUpper := strings.ToUpper(sqlType)

	switch {
	case strings.Contains(typeUpper, "INT"):
		return gofakeit.Number(1, 1000)
	case strings.Contains(typeUpper, "BOOL"):
		return gofakeit.Bool()
	case strings.Contains(typeUpper, "REAL"), strings.Contains(typeUpper, "FLOAT"), strings.Contains(typeUpper, "DOUBLE"):
		return gofakeit.Float64Range(0, 1000)
	case strings.Contains(typeUpper, "TEXT"), strings.Contains(typeUpper, "VARCHAR"), strings.Contains(typeUpper, "CHAR"):
		return gofakeit.Sentence(10)
	case strings.Contains(typeUpper, "DATE"), strings.Contains(typeUpper, "TIME"):
		return gofakeit.Date().Format("2006-01-02 15:04:05")
	default:
		return gofakeit.Word()
	}
}

// GenerateID generates a test seed ID
func GenerateID(index int) string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("test-seed-%d-%d", timestamp, index)
}

// GenerateCreatedAt generates a random created_at timestamp within the last 90 days
func GenerateCreatedAt() string {
	daysAgo := gofakeit.Number(0, 90)
	hoursAgo := gofakeit.Number(0, 23)
	minutesAgo := gofakeit.Number(0, 59)

	date := time.Now().
		AddDate(0, 0, -daysAgo).
		Add(-time.Hour * time.Duration(hoursAgo)).
		Add(-time.Minute * time.Duration(minutesAgo))

	return date.Format("2006-01-02 15:04:05")
}

// contains checks if a string contains any of the given substrings
func contains(s string, substrings ...string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// GenerateExampleValue generates an example value for documentation/display purposes
func GenerateExampleValue(column Column) string {
	value := GenerateValue(column)
	if value == nil {
		return "(auto-generated)"
	}

	switch v := value.(type) {
	case string:
		if len(v) > 50 {
			return fmt.Sprintf("%q...", v[:47])
		}
		return fmt.Sprintf("%q", v)
	case int:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%.2f", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
