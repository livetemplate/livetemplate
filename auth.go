package livetemplate

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
)

// Authenticator identifies users and maps them to session groups.
//
// Session groups are the fundamental concept for state sharing: all connections
// with the same groupID share the same Stores instance. Different groupIDs have
// completely isolated state.
//
// The Authenticator is called for both HTTP and WebSocket requests to determine:
// 1. Who is the user? (userID) - can be "" for anonymous
// 2. Which session group should they join? (groupID)
//
// For most applications, groupID = userID (simple 1:1 mapping), but advanced
// scenarios can implement custom mappings (e.g., collaborative workspaces where
// multiple users share one groupID).
type Authenticator interface {
	// Identify returns the user ID from the request.
	// Returns "" for anonymous users.
	// Returns error if authentication fails (e.g., invalid credentials).
	Identify(r *http.Request) (userID string, err error)

	// GetSessionGroup returns the session group ID for this user.
	// Multiple requests with the same groupID share state.
	//
	// For anonymous users: typically returns a browser-based identifier.
	// For authenticated users: typically returns userID.
	//
	// The groupID determines which Stores instance is used from SessionStore.
	GetSessionGroup(r *http.Request, userID string) (groupID string, err error)
}

// AnonymousAuthenticator provides browser-based session grouping for anonymous users.
//
// This is the default authenticator and implements the most common use case:
// - All tabs in the same browser share data (same groupID)
// - Different browsers have independent data (different groupID)
// - No user authentication required (userID is always "")
//
// The groupID is stored in a persistent cookie ("livetemplate-id") that survives
// browser restarts and lasts for 1 year. This provides seamless multi-tab
// experience without requiring user login.
//
// Example behavior:
//   - User opens Tab 1 in Chrome → groupID = "anon-abc123"
//   - User opens Tab 2 in Chrome → groupID = "anon-abc123" (same cookie, shares state)
//   - User opens Tab 3 in Firefox → groupID = "anon-xyz789" (different browser, independent state)
type AnonymousAuthenticator struct{}

// Identify always returns empty string for anonymous users.
func (a *AnonymousAuthenticator) Identify(r *http.Request) (string, error) {
	return "", nil
}

// GetSessionGroup returns a browser-based session group ID.
//
// If the "livetemplate-id" cookie exists, returns its value (persistent groupID).
// If no cookie exists, generates a new random groupID.
//
// The cookie is set by the handler when a new groupID is generated, ensuring
// it persists across requests and browser restarts.
func (a *AnonymousAuthenticator) GetSessionGroup(r *http.Request, userID string) (string, error) {
	// Check for existing session ID cookie
	cookie, err := r.Cookie("livetemplate-id")
	if err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	// Generate new session group ID for this browser
	return generateSessionID(), nil
}

// BasicAuthenticator provides username/password authentication.
//
// This is a helper for integrating with existing authentication systems.
// It calls a user-provided validation function and maps authenticated users
// to session groups using a simple 1:1 mapping (groupID = userID).
//
// Example usage:
//
//	auth := livetemplate.NewBasicAuthenticator(func(username, password string) (bool, error) {
//	    // Integrate with your authentication system
//	    return db.ValidateUser(username, password)
//	})
//
//	tmpl := livetemplate.New("app", livetemplate.WithAuthenticator(auth))
//
// For production use, consider implementing a custom Authenticator with:
// - JWT tokens
// - OAuth
// - Session cookies from existing auth middleware
// - Custom session group mapping logic
type BasicAuthenticator struct {
	// ValidateFunc is called to verify username/password credentials.
	// Returns true if credentials are valid, false otherwise.
	// Returns error for system failures (e.g., database connection error).
	ValidateFunc func(username, password string) (bool, error)
}

// NewBasicAuthenticator creates a BasicAuthenticator with the given validation function.
func NewBasicAuthenticator(validateFunc func(username, password string) (bool, error)) *BasicAuthenticator {
	return &BasicAuthenticator{
		ValidateFunc: validateFunc,
	}
}

// Identify extracts and validates HTTP Basic Auth credentials.
//
// Returns the username if credentials are valid.
// Returns error if:
// - No Authorization header present
// - Invalid Basic Auth format
// - Credentials validation fails
// - System error during validation
func (a *BasicAuthenticator) Identify(r *http.Request) (string, error) {
	username, password, ok := r.BasicAuth()
	if !ok {
		return "", fmt.Errorf("no basic auth credentials provided")
	}

	valid, err := a.ValidateFunc(username, password)
	if err != nil {
		return "", fmt.Errorf("authentication error: %w", err)
	}

	if !valid {
		return "", fmt.Errorf("invalid credentials")
	}

	return username, nil
}

// GetSessionGroup returns userID as the session group ID (1:1 mapping).
//
// Each authenticated user gets their own isolated session group.
// Multiple tabs for the same user share state.
// Different users have completely isolated state.
//
// Example:
//   - User "alice" in Tab 1 → groupID = "alice"
//   - User "alice" in Tab 2 → groupID = "alice" (shares state with Tab 1)
//   - User "bob" in Tab 1 → groupID = "bob" (isolated from alice)
func (a *BasicAuthenticator) GetSessionGroup(r *http.Request, userID string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("cannot get session group for empty userID")
	}
	return userID, nil
}

// generateSessionID creates a cryptographically secure random identifier for session groups.
//
// Uses crypto/rand (not math/rand) to generate 32 bytes (256 bits) of entropy,
// which is then base64-encoded to produce a ~43 character string.
//
// This provides sufficient entropy to prevent:
// - Collision probability: negligible (2^256 possible values)
// - Brute force attacks: computationally infeasible
// - Prediction attacks: cryptographically secure random source
//
// The generated ID is suitable for:
// - Session group IDs (anonymous users)
// - Session cookies
// - Any security-sensitive identifier
func generateSessionID() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		// crypto/rand.Read only fails on systems without entropy source
		// This should never happen on modern systems
		panic(fmt.Sprintf("failed to generate session ID: %v", err))
	}
	return base64.URLEncoding.EncodeToString(b)
}
