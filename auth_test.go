package livetemplate

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAnonymousAuthenticator_Identify tests that anonymous authenticator always returns empty userID
func TestAnonymousAuthenticator_Identify(t *testing.T) {
	auth := &AnonymousAuthenticator{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	userID, err := auth.Identify(req)

	if err != nil {
		t.Errorf("Identify() returned unexpected error: %v", err)
	}

	if userID != "" {
		t.Errorf("Identify() returned userID = %q, want empty string", userID)
	}
}

// TestAnonymousAuthenticator_GetSessionGroup_NewSession tests session group generation for new users
func TestAnonymousAuthenticator_GetSessionGroup_NewSession(t *testing.T) {
	auth := &AnonymousAuthenticator{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	groupID, err := auth.GetSessionGroup(req, "")

	if err != nil {
		t.Errorf("GetSessionGroup() returned unexpected error: %v", err)
	}

	if groupID == "" {
		t.Error("GetSessionGroup() returned empty groupID, expected random ID")
	}

	// Verify it's a valid base64 string (from generateSessionID)
	_, err = base64.URLEncoding.DecodeString(groupID)
	if err != nil {
		t.Errorf("GetSessionGroup() returned invalid base64 groupID: %v", err)
	}
}

// TestAnonymousAuthenticator_GetSessionGroup_ExistingCookie tests that existing cookie is reused
func TestAnonymousAuthenticator_GetSessionGroup_ExistingCookie(t *testing.T) {
	auth := &AnonymousAuthenticator{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	existingGroupID := "existing-session-id-123"
	req.AddCookie(&http.Cookie{
		Name:  "livetemplate-id",
		Value: existingGroupID,
	})

	groupID, err := auth.GetSessionGroup(req, "")

	if err != nil {
		t.Errorf("GetSessionGroup() returned unexpected error: %v", err)
	}

	if groupID != existingGroupID {
		t.Errorf("GetSessionGroup() = %q, want %q (should reuse existing cookie)", groupID, existingGroupID)
	}
}

// TestAnonymousAuthenticator_GetSessionGroup_EmptyCookie tests that empty cookie value is ignored
func TestAnonymousAuthenticator_GetSessionGroup_EmptyCookie(t *testing.T) {
	auth := &AnonymousAuthenticator{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	// Add cookie with empty value
	req.AddCookie(&http.Cookie{
		Name:  "livetemplate-id",
		Value: "",
	})

	groupID, err := auth.GetSessionGroup(req, "")

	if err != nil {
		t.Errorf("GetSessionGroup() returned unexpected error: %v", err)
	}

	if groupID == "" {
		t.Error("GetSessionGroup() returned empty groupID, expected to generate new ID when cookie value is empty")
	}
}

// TestAnonymousAuthenticator_GetSessionGroup_Uniqueness tests that multiple calls generate unique IDs
func TestAnonymousAuthenticator_GetSessionGroup_Uniqueness(t *testing.T) {
	auth := &AnonymousAuthenticator{}

	seen := make(map[string]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		groupID, err := auth.GetSessionGroup(req, "")

		if err != nil {
			t.Fatalf("GetSessionGroup() returned error on iteration %d: %v", i, err)
		}

		if seen[groupID] {
			t.Errorf("GetSessionGroup() generated duplicate groupID: %q", groupID)
		}

		seen[groupID] = true
	}

	if len(seen) != iterations {
		t.Errorf("Generated %d unique IDs out of %d attempts", len(seen), iterations)
	}
}

// TestBasicAuthenticator_Identify_Success tests successful authentication
func TestBasicAuthenticator_Identify_Success(t *testing.T) {
	auth := NewBasicAuthenticator(func(username, password string) (bool, error) {
		return username == "alice" && password == "secret123", nil
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("alice", "secret123")

	userID, err := auth.Identify(req)

	if err != nil {
		t.Errorf("Identify() returned unexpected error: %v", err)
	}

	if userID != "alice" {
		t.Errorf("Identify() = %q, want %q", userID, "alice")
	}
}

// TestBasicAuthenticator_Identify_InvalidCredentials tests authentication failure
func TestBasicAuthenticator_Identify_InvalidCredentials(t *testing.T) {
	auth := NewBasicAuthenticator(func(username, password string) (bool, error) {
		return username == "alice" && password == "secret123", nil
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("alice", "wrongpassword")

	userID, err := auth.Identify(req)

	if err == nil {
		t.Error("Identify() expected error for invalid credentials, got nil")
	}

	if userID != "" {
		t.Errorf("Identify() returned userID = %q for invalid credentials, want empty string", userID)
	}

	if !strings.Contains(err.Error(), "invalid credentials") {
		t.Errorf("Identify() error = %q, want error containing 'invalid credentials'", err.Error())
	}
}

// TestBasicAuthenticator_Identify_NoAuthHeader tests missing authorization
func TestBasicAuthenticator_Identify_NoAuthHeader(t *testing.T) {
	auth := NewBasicAuthenticator(func(username, password string) (bool, error) {
		return true, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Authorization header set

	userID, err := auth.Identify(req)

	if err == nil {
		t.Error("Identify() expected error for missing auth header, got nil")
	}

	if userID != "" {
		t.Errorf("Identify() returned userID = %q for missing auth, want empty string", userID)
	}

	if !strings.Contains(err.Error(), "no basic auth credentials") {
		t.Errorf("Identify() error = %q, want error containing 'no basic auth credentials'", err.Error())
	}
}

// TestBasicAuthenticator_Identify_ValidationError tests system errors during validation
func TestBasicAuthenticator_Identify_ValidationError(t *testing.T) {
	auth := NewBasicAuthenticator(func(username, password string) (bool, error) {
		return false, errors.New("database connection failed")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("alice", "secret123")

	userID, err := auth.Identify(req)

	if err == nil {
		t.Error("Identify() expected error from validation function, got nil")
	}

	if userID != "" {
		t.Errorf("Identify() returned userID = %q for validation error, want empty string", userID)
	}

	if !strings.Contains(err.Error(), "database connection failed") {
		t.Errorf("Identify() error = %q, want error containing 'database connection failed'", err.Error())
	}
}

// TestBasicAuthenticator_GetSessionGroup_Success tests session group mapping for authenticated users
func TestBasicAuthenticator_GetSessionGroup_Success(t *testing.T) {
	auth := NewBasicAuthenticator(nil) // ValidateFunc not needed for GetSessionGroup
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	groupID, err := auth.GetSessionGroup(req, "alice")

	if err != nil {
		t.Errorf("GetSessionGroup() returned unexpected error: %v", err)
	}

	// For BasicAuthenticator, groupID should equal userID (1:1 mapping)
	if groupID != "alice" {
		t.Errorf("GetSessionGroup() = %q, want %q (1:1 mapping)", groupID, "alice")
	}
}

// TestBasicAuthenticator_GetSessionGroup_EmptyUserID tests error handling for empty userID
func TestBasicAuthenticator_GetSessionGroup_EmptyUserID(t *testing.T) {
	auth := NewBasicAuthenticator(nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	groupID, err := auth.GetSessionGroup(req, "")

	if err == nil {
		t.Error("GetSessionGroup() expected error for empty userID, got nil")
	}

	if groupID != "" {
		t.Errorf("GetSessionGroup() returned groupID = %q for empty userID, want empty string", groupID)
	}

	if !strings.Contains(err.Error(), "empty userID") {
		t.Errorf("GetSessionGroup() error = %q, want error containing 'empty userID'", err.Error())
	}
}

// TestBasicAuthenticator_GetSessionGroup_Consistency tests that same userID always returns same groupID
func TestBasicAuthenticator_GetSessionGroup_Consistency(t *testing.T) {
	auth := NewBasicAuthenticator(nil)

	users := []string{"alice", "bob", "charlie"}

	for _, user := range users {
		// Call GetSessionGroup multiple times for same user
		var groupIDs []string
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			groupID, err := auth.GetSessionGroup(req, user)
			if err != nil {
				t.Fatalf("GetSessionGroup() returned error for user %q: %v", user, err)
			}
			groupIDs = append(groupIDs, groupID)
		}

		// All should be identical
		for i := 1; i < len(groupIDs); i++ {
			if groupIDs[i] != groupIDs[0] {
				t.Errorf("GetSessionGroup() returned inconsistent groupID for user %q: %q != %q",
					user, groupIDs[i], groupIDs[0])
			}
		}

		// Should equal userID
		if groupIDs[0] != user {
			t.Errorf("GetSessionGroup() = %q, want %q (should equal userID)", groupIDs[0], user)
		}
	}
}

// TestGenerateSessionID_Length tests that generated IDs have expected length
func TestGenerateSessionID_Length(t *testing.T) {
	id := generateSessionID()

	// 32 bytes base64-encoded should produce ~43 characters
	// (32 * 8 / 6 = 42.67, rounded up with padding)
	expectedLength := 44 // With padding

	if len(id) != expectedLength {
		t.Errorf("generateSessionID() length = %d, want %d", len(id), expectedLength)
	}
}

// TestGenerateSessionID_Base64 tests that generated IDs are valid base64
func TestGenerateSessionID_Base64(t *testing.T) {
	id := generateSessionID()

	decoded, err := base64.URLEncoding.DecodeString(id)
	if err != nil {
		t.Errorf("generateSessionID() produced invalid base64: %v", err)
	}

	// Should decode to exactly 32 bytes
	if len(decoded) != 32 {
		t.Errorf("generateSessionID() decoded length = %d, want 32 bytes", len(decoded))
	}
}

// TestGenerateSessionID_Uniqueness tests that multiple calls produce unique IDs
func TestGenerateSessionID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		id := generateSessionID()

		if seen[id] {
			t.Errorf("generateSessionID() generated duplicate ID on iteration %d", i)
		}

		seen[id] = true
	}

	if len(seen) != iterations {
		t.Errorf("Generated %d unique IDs out of %d attempts", len(seen), iterations)
	}
}

// TestAuthenticator_Interface verifies that implementations satisfy Authenticator interface
func TestAuthenticator_Interface(t *testing.T) {
	var _ Authenticator = (*AnonymousAuthenticator)(nil)
	var _ Authenticator = (*BasicAuthenticator)(nil)
}
