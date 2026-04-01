package util

import "testing"

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"a", "a"},
		{"A", "a"},
		{"already_snake", "already_snake"},
		{"Increment", "increment"},
		{"AddItem", "add_item"},
		{"UpdateUserProfile", "update_user_profile"},
		{"PasswordConfirmation", "password_confirmation"},
		{"PhoneNumber", "phone_number"},
		{"HTMLParser", "h_t_m_l_parser"},
		{"DisplayName", "display_name"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToSnakeCase(tt.input)
			if result != tt.expected {
				t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFindCommonPrefix(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected string
	}{
		{
			name:     "identical strings",
			s1:       "hello world",
			s2:       "hello world",
			expected: "hello world",
		},
		{
			name:     "common prefix",
			s1:       "hello world",
			s2:       "hello there",
			expected: "hello ",
		},
		{
			name:     "no common prefix",
			s1:       "abc",
			s2:       "xyz",
			expected: "",
		},
		{
			name:     "empty strings",
			s1:       "",
			s2:       "",
			expected: "",
		},
		{
			name:     "one empty string",
			s1:       "hello",
			s2:       "",
			expected: "",
		},
		{
			name:     "full prefix (s1 shorter)",
			s1:       "hello",
			s2:       "hello world",
			expected: "hello",
		},
		{
			name:     "full prefix (s2 shorter)",
			s1:       "hello world",
			s2:       "hello",
			expected: "hello",
		},
		{
			name:     "HTML tags",
			s1:       "<div>content1</div>",
			s2:       "<div>content2</div>",
			expected: "<div>content",
		},
		{
			name:     "UTF-8 multi-byte characters",
			s1:       "hello 世界",
			s2:       "hello 世人",
			expected: "hello 世",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindCommonPrefix(tt.s1, tt.s2)
			if result != tt.expected {
				t.Errorf("FindCommonPrefix(%q, %q) = %q, want %q",
					tt.s1, tt.s2, result, tt.expected)
			}
		})
	}
}

func TestFindCommonSuffix(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected string
	}{
		{
			name:     "identical strings",
			s1:       "hello world",
			s2:       "hello world",
			expected: "hello world",
		},
		{
			name:     "common suffix",
			s1:       "hello world",
			s2:       "goodbye world",
			expected: " world",
		},
		{
			name:     "no common suffix",
			s1:       "abc",
			s2:       "xyz",
			expected: "",
		},
		{
			name:     "empty strings",
			s1:       "",
			s2:       "",
			expected: "",
		},
		{
			name:     "one empty string",
			s1:       "hello",
			s2:       "",
			expected: "",
		},
		{
			name:     "full suffix (s1 shorter)",
			s1:       "world",
			s2:       "hello world",
			expected: "world",
		},
		{
			name:     "full suffix (s2 shorter)",
			s1:       "hello world",
			s2:       "world",
			expected: "world",
		},
		{
			name:     "HTML tags",
			s1:       "<div>content1</div>",
			s2:       "<div>content2</div>",
			expected: "</div>",
		},
		{
			name:     "UTF-8 multi-byte characters",
			s1:       "世界 hello",
			s2:       "世人 hello",
			expected: " hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindCommonSuffix(tt.s1, tt.s2)
			if result != tt.expected {
				t.Errorf("FindCommonSuffix(%q, %q) = %q, want %q",
					tt.s1, tt.s2, result, tt.expected)
			}
		})
	}
}

func TestFindCommonPrefixAndSuffix(t *testing.T) {
	// Test that prefix and suffix functions work together correctly
	s1 := "<div>content1</div>"
	s2 := "<div>content2</div>"

	prefix := FindCommonPrefix(s1, s2)
	suffix := FindCommonSuffix(s1, s2)

	if prefix != "<div>content" {
		t.Errorf("Expected prefix %q, got %q", "<div>content", prefix)
	}

	if suffix != "</div>" {
		t.Errorf("Expected suffix %q, got %q", "</div>", suffix)
	}

	// Verify they don't overlap
	changeStart := len(prefix)
	changeEnd := len(s2) - len(suffix)

	if changeStart > changeEnd {
		t.Errorf("Prefix and suffix overlap: prefix ends at %d, suffix starts at %d",
			changeStart, changeEnd)
	}

	// Extract the changed part
	changed := s2[changeStart:changeEnd]
	expected := "2"
	if changed != expected {
		t.Errorf("Changed content = %q, want %q", changed, expected)
	}
}
