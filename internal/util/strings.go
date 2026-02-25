// Package util provides generic utility functions that can be used across the livetemplate codebase.
package util

// FindCommonPrefix finds the longest common prefix between two strings.
// Returns the prefix string, or empty string if no common prefix exists.
//
// Note: This function operates at the byte level, not rune level. This is
// appropriate for HTML comparison where ASCII delimiters are being matched,
// but may split multi-byte UTF-8 sequences. For general text processing,
// consider using a rune-based approach.
func FindCommonPrefix(s1, s2 string) string {
	minLen := min(len(s2), len(s1))

	for i := range minLen {
		if s1[i] != s2[i] {
			return s1[:i]
		}
	}
	return s1[:minLen]
}

// FindCommonSuffix finds the longest common suffix between two strings.
// Returns the suffix string, or empty string if no common suffix exists.
//
// Note: This function operates at the byte level, not rune level. This is
// appropriate for HTML comparison where ASCII delimiters are being matched,
// but may split multi-byte UTF-8 sequences. For general text processing,
// consider using a rune-based approach.
func FindCommonSuffix(s1, s2 string) string {
	len1, len2 := len(s1), len(s2)
	minLen := min(len2, len1)

	for i := range minLen {
		if s1[len1-1-i] != s2[len2-1-i] {
			return s1[len1-i:]
		}
	}
	return s1[len1-minLen:]
}
