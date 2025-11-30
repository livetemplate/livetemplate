# Authentication Reference

Authentication in LiveTemplate handles two key responsibilities: **user identification** and **session grouping**. This guide covers the Authenticator interface, built-in implementations, HTTP methods for auth flows, and patterns for custom authentication.

## Overview

LiveTemplate's authentication system determines:
1. **Who is the user?** (`userID`) - Can be empty for anonymous users
2. **Which session group should they join?** (`groupID`) - Determines state sharing

Session groups are the fundamental isolation boundary: all connections with the same `groupID` share the same `Stores` instance. Different groupIDs have completely isolated state.

```
Browser Tab 1 ──┐
                ├── groupID: "alice" ──► Shared Stores instance
Browser Tab 2 ──┘

Browser Tab 3 ──── groupID: "bob" ────► Different Stores instance (isolated)
```

## Authenticator Interface

```go
type Authenticator interface {
    // Identify returns the user ID from the request.
    // Returns "" for anonymous users.
    // Returns error if authentication fails (e.g., invalid credentials).
    Identify(r *http.Request) (userID string, err error)

    // GetSessionGroup returns the session group ID for this user.
    // Multiple requests with the same groupID share state.
    GetSessionGroup(r *http.Request, userID string) (groupID string, err error)
}
```

### Method Details

**Identify(r *http.Request) (userID, error)**
- Called for every HTTP and WebSocket request
- Returns the user's identity (username, user ID, email, etc.)
- Returns `""` for anonymous/unauthenticated users
- Returns an error to reject the request (e.g., invalid credentials)

**GetSessionGroup(r *http.Request, userID string) (groupID, error)**
- Called after `Identify()` to determine session grouping
- For most applications: `groupID = userID` (simple 1:1 mapping)
- Advanced scenarios can implement custom mappings (e.g., collaborative workspaces)
- The `groupID` determines which `Stores` instance is retrieved from `SessionStore`

## Built-in Authenticators

### AnonymousAuthenticator (Default)

Browser-based session grouping for anonymous users. This is the default when no authenticator is configured.

**How it works:**
- All tabs in the same browser share state (same cookie, same `groupID`)
- Different browsers have independent state (different cookies, different `groupID`)
- No user authentication required (`userID` is always `""`)

**Cookie details:**
- Name: `livetemplate-id`
- Duration: 1 year (persistent)
- Security: HttpOnly, SameSite=Lax

**Session ID generation:**
- Uses `crypto/rand` (not `math/rand`) for cryptographic security
- 32 bytes (256 bits) of entropy
- Base64-encoded to ~44 character string
- Collision probability: negligible (2^256 possible values)

**Example behavior:**
```
User opens Tab 1 in Chrome → groupID = "K7xR9mN2pQ8wL4vB..." (truncated, ~44 chars)
User opens Tab 2 in Chrome → groupID = "K7xR9mN2pQ8wL4vB..." (same cookie, shares state)
User opens Tab 3 in Firefox → groupID = "Yt3hF6jM1nS5xC8d..." (different browser, isolated)
```

**When to use:**
- Applications that don't require user accounts
- Guest/anonymous functionality
- Quick prototypes and demos

### BasicAuthenticator

HTTP Basic Authentication wrapper for username/password authentication.

```go
auth := livetemplate.NewBasicAuthenticator(func(username, password string) (bool, error) {
    // Integrate with your authentication system
    return db.ValidateUser(username, password)
})

tmpl := livetemplate.New("app", livetemplate.WithAuthenticator(auth))
```

**How it works:**
- Extracts credentials from `Authorization: Basic ...` header
- Calls your validation function to verify credentials
- Maps to session groups via simple 1:1 mapping (`groupID = userID`)

**Example behavior:**
```
User "alice" in Tab 1 → groupID = "alice"
User "alice" in Tab 2 → groupID = "alice" (shares state with Tab 1)
User "bob" in Tab 1   → groupID = "bob" (isolated from alice)
```

**Security Warnings:**

> **HTTPS REQUIRED**: BasicAuthenticator uses HTTP Basic Authentication, which sends credentials as base64-encoded strings. This is NOT encrypted and MUST only be used over HTTPS connections.

> **NO BUILT-IN RATE LIMITING**: This implementation has no protection against brute force attacks. For production use, implement:
> - Rate limiting middleware (e.g., `golang.org/x/time/rate`)
> - Account lockout after N failed attempts
> - External protection (fail2ban, CloudFlare, WAF)

**Production recommendation:** Consider implementing a custom `Authenticator` with JWT tokens, OAuth, or session cookies from existing auth middleware.

## Configuration Options

### WithAuthenticator

Set a custom authenticator for user identification and session grouping:

```go
tmpl := livetemplate.New("app", livetemplate.WithAuthenticator(myAuth))
```

### WithCookieMaxAge

Set the maximum age for session cookies (used by `AnonymousAuthenticator`):

```go
// 30-day sessions instead of default 1 year
tmpl := livetemplate.New("app", livetemplate.WithCookieMaxAge(30*24*time.Hour))
```

Default: 365 days (1 year)

## ActionContext HTTP Methods (v0.5)

Version 0.5 added HTTP-aware methods to `ActionContext` for authentication flows that need to set cookies or redirect users. These methods are available for HTTP POST actions but return `ErrNoHTTPContext` for WebSocket actions.

### Context Checking

```go
func (s *AuthStore) Change(ctx *livetemplate.ActionContext) error {
    if ctx.IsHTTP() {
        // Can use SetCookie, Redirect, etc.
    } else {
        // WebSocket action - HTTP methods not available
    }
    return nil
}
```

### Cookie Operations

**SetCookie** - Add a Set-Cookie header to the response:

```go
ctx.SetCookie(&http.Cookie{
    Name:     "session_token",
    Value:    token,
    Path:     "/",
    HttpOnly: true,                    // Prevent XSS access
    Secure:   true,                    // HTTPS only
    SameSite: http.SameSiteStrictMode, // CSRF protection
    MaxAge:   86400 * 30,              // 30 days
})
```

**GetCookie** - Read a cookie from the request:

```go
cookie, err := ctx.GetCookie("session_token")
if err == http.ErrNoCookie {
    // Cookie doesn't exist
}
```

**DeleteCookie** - Remove a cookie:

```go
ctx.DeleteCookie("session_token") // Sets MaxAge = -1
```

### Redirect Operations

```go
// Redirect to dashboard after login
ctx.Redirect("/dashboard", http.StatusSeeOther) // 303
```

**Security:** Only relative paths starting with `/` are allowed. This prevents open redirect vulnerabilities:

```go
// Valid redirects
ctx.Redirect("/dashboard", http.StatusSeeOther)     // OK
ctx.Redirect("/users/profile", http.StatusFound)    // OK

// Invalid redirects (rejected with ErrInvalidRedirectURL)
ctx.Redirect("https://evil.com", http.StatusFound)  // Rejected
ctx.Redirect("//evil.com", http.StatusFound)        // Rejected (protocol-relative)
```

### Header Operations

```go
// Set response header
ctx.SetHeader("X-Custom-Header", "value")

// Get request header
authHeader := ctx.GetHeader("Authorization")
```

### Error Types

```go
var (
    // Returned when HTTP methods are called from WebSocket actions
    ErrNoHTTPContext = errors.New("HTTP methods require HTTP context")

    // Returned when Redirect is called with non-3xx status
    ErrInvalidRedirectCode = errors.New("invalid redirect status code (must be 3xx)")

    // Returned when Redirect URL could cause open redirect vulnerability
    ErrInvalidRedirectURL = errors.New("invalid redirect URL (must be relative path starting with /)")
)
```

## Custom Authenticator Patterns

### JWT Token Authenticator

```go
type JWTAuthenticator struct {
    SecretKey []byte
}

func (a *JWTAuthenticator) Identify(r *http.Request) (string, error) {
    // Extract token from Authorization header
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        return "", nil // Anonymous user
    }

    // Parse "Bearer <token>"
    parts := strings.SplitN(authHeader, " ", 2)
    if len(parts) != 2 || parts[0] != "Bearer" {
        return "", fmt.Errorf("invalid authorization header format")
    }

    // Validate and parse JWT
    token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
        return a.SecretKey, nil
    })
    if err != nil {
        return "", fmt.Errorf("invalid token: %w", err)
    }

    claims := token.Claims.(jwt.MapClaims)
    return claims["sub"].(string), nil
}

func (a *JWTAuthenticator) GetSessionGroup(r *http.Request, userID string) (string, error) {
    if userID == "" {
        // Anonymous users get browser-based grouping
        return generateBrowserSessionID(r)
    }
    return userID, nil // 1:1 mapping for authenticated users
}
```

### Multi-Tenant Authenticator

For applications where multiple users share state (e.g., collaborative workspaces):

```go
type TenantAuthenticator struct {
    SessionStore sessions.Store // Your session middleware
}

func (a *TenantAuthenticator) Identify(r *http.Request) (string, error) {
    // Extract user from session cookie (adapt to your auth system)
    session, err := a.SessionStore.Get(r, "session-name")
    if err != nil {
        return "", nil // Anonymous
    }
    userID, _ := session.Values["user_id"].(string)
    return userID, nil
}

func (a *TenantAuthenticator) GetSessionGroup(r *http.Request, userID string) (string, error) {
    // Extract tenant from subdomain: "acme.example.com" → "acme"
    host := r.Host
    if idx := strings.Index(host, "."); idx > 0 {
        return host[:idx], nil
    }
    // Or from header: X-Tenant-ID
    if tenantID := r.Header.Get("X-Tenant-ID"); tenantID != "" {
        return tenantID, nil
    }
    // Fallback to user-based grouping
    return userID, nil
}
```

**Example behavior:**
```
User "alice" in workspace "acme" → groupID = "acme"
User "bob" in workspace "acme"   → groupID = "acme" (shares state with alice!)
User "carol" in workspace "beta" → groupID = "beta" (isolated from acme)
```

### Session Cookie Authenticator

Integrate with existing session middleware:

```go
type SessionAuthenticator struct {
    SessionStore sessions.Store
}

func (a *SessionAuthenticator) Identify(r *http.Request) (string, error) {
    session, err := a.SessionStore.Get(r, "session-name")
    if err != nil {
        return "", nil // Anonymous
    }

    userID, ok := session.Values["user_id"].(string)
    if !ok {
        return "", nil // Anonymous
    }

    return userID, nil
}

func (a *SessionAuthenticator) GetSessionGroup(r *http.Request, userID string) (string, error) {
    if userID == "" {
        return generateBrowserSessionID(r)
    }
    return userID, nil
}
```

## Authentication Flow

### HTTP Request Flow

```
1. HTTP Request arrives
2. Authenticator.Identify(r) → userID (or "" for anonymous)
3. Authenticator.GetSessionGroup(r, userID) → groupID
4. SessionStore.Get(groupID) → Stores (or create new)
5. Handler processes request with shared Stores
6. Response sent (with cookies if set)
```

### WebSocket Upgrade Flow

```
1. WebSocket upgrade request arrives
2. Authenticator.Identify(r) → userID
3. Authenticator.GetSessionGroup(r, userID) → groupID
4. Check connection limits
5. Set session cookie if new groupID
6. Upgrade to WebSocket
7. Get/create Stores for groupID
8. Register connection in ConnectionRegistry
9. Call OnConnect() on SessionAware stores
10. Send initial template tree
11. Enter message loop
```

## Security Best Practices

### Secure Cookie Settings

Always use secure cookie settings for authentication:

```go
ctx.SetCookie(&http.Cookie{
    Name:     "session_token",
    Value:    token,
    Path:     "/",
    HttpOnly: true,                    // Prevents JavaScript access (XSS protection)
    Secure:   true,                    // HTTPS only (prevents sniffing)
    SameSite: http.SameSiteStrictMode, // CSRF protection
    MaxAge:   86400 * 30,              // Explicit expiration
})
```

### HTTPS Requirement

All authentication mechanisms should use HTTPS in production:
- Basic Auth credentials are base64-encoded (not encrypted)
- Session cookies can be stolen over plain HTTP
- JWT tokens are visible in plain text

### Rate Limiting

Implement rate limiting for login endpoints:

```go
import "golang.org/x/time/rate"

var loginLimiter = rate.NewLimiter(rate.Every(time.Second), 5) // 5 requests/second

func (s *AuthStore) Change(ctx *livetemplate.ActionContext) error {
    if ctx.Action == "login" {
        if !loginLimiter.Allow() {
            return errors.New("too many login attempts, please try again later")
        }
        // ... handle login
    }
    return nil
}
```

### Open Redirect Prevention

LiveTemplate's `Redirect()` method automatically prevents open redirects by:
- Only allowing relative paths starting with `/`
- Rejecting protocol-relative URLs (`//evil.com`)
- Rejecting absolute URLs (`https://evil.com`)

## See Also

- [Server Actions Reference](server-actions.md) - Push updates from server-side code
- [Session Reference](session.md) - Session stores and connection management
- [Error Handling](error-handling.md) - Validation and error display
- [Scaling Guide](../SCALING.md) - Redis-backed session stores for distributed deployments
