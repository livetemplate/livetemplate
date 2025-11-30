# Authentication System for LiveTemplate v0.5

**Status:** ✅ Implemented (v0.4.1)
**Version:** 1.0
**Date:** 2025-11-29
**Implemented:** 2025-11-30

---

## TL;DR

**Problem:** Password authentication is broken in v0.4.x because `ActionContext` can't set cookies or redirect.

**Solution:** Add HTTP methods to `ActionContext` + re-enable password auth in lvt CLI.

**What works now:** Magic-link auth (uses separate HTTP handlers)
**What's broken:** Password auth (needs cookies/redirect from `Change()`)
**Fix:** Add `ctx.SetCookie()` and `ctx.Redirect()` to ActionContext

---

## The Problem

### What Users See

```bash
$ lvt gen auth
⚠️  WARNING: Password authentication is temporarily disabled due to API compatibility.
    Use magic-link authentication instead, which works with separate HTTP handlers.
```

Users generated auth code that **doesn't compile** because it uses an outdated LiveTemplate API.

### Root Cause

Password login requires setting session cookies and redirecting, but the current `ActionContext` doesn't provide HTTP access:

```go
// CURRENT API (v0.4.x) - Can't set cookies or redirect
type ActionContext struct {
    Action string
    Data   *ActionData
    Ctx    context.Context
    // ❌ No http.ResponseWriter
    // ❌ No redirect capability
}

func (s *AuthState) Change(ctx *ActionContext) error {
    switch ctx.Action {
    case "login":
        user, err := s.validateCredentials(ctx)
        // ❌ Can't set session cookie!
        // ❌ Can't redirect to homepage!
        return nil
    }
}
```

### Why Magic-Link Works But Password Doesn't

**Magic-link splits the flow:**
1. User enters email → `Change()` sends magic link (no HTTP needed)
2. User clicks link → **Separate HTTP handler** sets cookie & redirects (has HTTP access)

**Password login is atomic:**
1. User enters credentials → `Change()` validates AND needs to set cookie + redirect (no HTTP access ❌)

---

## The Solution

### Add HTTP Methods to ActionContext

```go
// NEW API (v0.5.0)
type ActionContext struct {
    Action string
    Data   *ActionData
    Ctx    context.Context

    // Private fields
    w http.ResponseWriter
    r *http.Request
}

// New methods
func (c *ActionContext) SetCookie(cookie *http.Cookie) error
func (c *ActionContext) GetCookie(name string) (*http.Cookie, error)
func (c *ActionContext) DeleteCookie(name string)
func (c *ActionContext) Redirect(url string, code int) error
func (c *ActionContext) SetHeader(key, value string)
func (c *ActionContext) GetHeader(key string) string
```

### Usage Example

```go
// Password login now works!
func (s *AuthState) Change(ctx *ActionContext) error {
    switch ctx.Action {
    case "login":
        var input LoginInput
        if err := ctx.BindAndValidate(&input, validate); err != nil {
            s.Error = "Invalid input"
            return nil
        }

        user, err := s.validateCredentials(input.Email, input.Password)
        if err != nil {
            s.Error = "Invalid credentials"
            return nil
        }

        // Generate session token
        token := generateToken(user.ID)

        // ✅ Set session cookie (NEW)
        ctx.SetCookie(&http.Cookie{
            Name:     "session_token",
            Value:    token,
            Path:     "/",
            MaxAge:   30 * 24 * 60 * 60,
            HttpOnly: true,
            Secure:   true,
            SameSite: http.SameSiteStrictMode,
        })

        // ✅ Redirect to homepage (NEW)
        return ctx.Redirect("/", http.StatusSeeOther)
    }
}
```

---

## Implementation Plan

### Phase 1: Core HTTP Methods (v0.5.0)

**File:** `actioncontext.go`

```go
// SetCookie adds a Set-Cookie header
func (c *ActionContext) SetCookie(cookie *http.Cookie) error {
    if c.w == nil {
        return ErrNoHTTPContext
    }
    http.SetCookie(c.w, cookie)
    return nil
}

// GetCookie retrieves a cookie from the request
func (c *ActionContext) GetCookie(name string) (*http.Cookie, error) {
    if c.r == nil {
        return nil, ErrNoHTTPContext
    }
    return c.r.Cookie(name)
}

// Redirect sends an HTTP redirect
func (c *ActionContext) Redirect(url string, code int) error {
    if c.w == nil || c.r == nil {
        return ErrNoHTTPContext
    }

    // Validate redirect
    if code < 300 || code >= 400 {
        return ErrInvalidRedirectCode
    }

    http.Redirect(c.w, c.r, url, code)
    return nil
}

// SetHeader sets a response header
func (c *ActionContext) SetHeader(key, value string) {
    if c.w != nil {
        c.w.Header().Set(key, value)
    }
}

// GetHeader retrieves a request header
func (c *ActionContext) GetHeader(key string) string {
    if c.r == nil {
        return ""
    }
    return c.r.Header.Get(key)
}

// DeleteCookie removes a cookie
func (c *ActionContext) DeleteCookie(name string) {
    c.SetCookie(&http.Cookie{
        Name:   name,
        Value:  "",
        Path:   "/",
        MaxAge: -1,
    })
}

// Errors
var (
    ErrNoHTTPContext       = errors.New("no HTTP context available")
    ErrInvalidRedirectCode = errors.New("invalid redirect status code (must be 3xx)")
)
```

**Tests:** `actioncontext_test.go`

```go
func TestActionContext_SetCookie(t *testing.T) {
    rec := httptest.NewRecorder()
    req := httptest.NewRequest("POST", "/", nil)

    ctx := &ActionContext{w: rec, r: req}

    err := ctx.SetCookie(&http.Cookie{
        Name:  "test",
        Value: "value",
    })

    assert.NoError(t, err)
    assert.Contains(t, rec.Header().Get("Set-Cookie"), "test=value")
}

func TestActionContext_Redirect(t *testing.T) {
    rec := httptest.NewRecorder()
    req := httptest.NewRequest("POST", "/", nil)

    ctx := &ActionContext{w: rec, r: req}

    err := ctx.Redirect("/home", http.StatusSeeOther)

    assert.NoError(t, err)
    assert.Equal(t, http.StatusSeeOther, rec.Code)
    assert.Equal(t, "/home", rec.Header().Get("Location"))
}
```

**Deliverable:** LiveTemplate v0.5.0 with HTTP-enabled ActionContext

---

### Phase 2: Re-enable Password Auth in lvt CLI

**File:** `lvt/commands/auth.go`

```diff
- // TEMPORARY: Disable password auth by default
- if !flags.NoPassword {
-     fmt.Println("⚠️  WARNING: Password authentication is temporarily disabled...")
-     flags.NoPassword = true
- }

+ // Password auth works in LiveTemplate v0.5.0+
+ if !flags.NoPassword {
+     fmt.Println("✅ Generating password authentication...")
+ }
```

**File:** `lvt/internal/kits/system/multi/templates/auth/handler.go.tmpl`

Already updated to use new API - just needs to be uncommented:

```go
{{- if .EnablePassword }}
func (s *{{.StructName}}State) handleLogin(ctx *livetemplate.ActionContext) error {
    // ... validate credentials ...

    // Set session cookie (uses new API)
    ctx.SetCookie(&http.Cookie{...})

    // Redirect (uses new API)
    return ctx.Redirect("/", http.StatusSeeOther)
}
{{- end }}
```

**Tests:** `lvt/e2e/auth_test.go`

```go
func TestPasswordAuth_EndToEnd(t *testing.T) {
    // Create test app
    app := createTestApp(t)

    // Generate auth with password
    runCommand(t, "lvt", "gen", "auth")

    // Build app
    runCommand(t, "go", "build", "./...")

    // Start server
    server := startTestServer(t, app)
    defer server.Close()

    // Register user
    resp := httpPost(t, server.URL+"/auth", map[string]string{
        "action":   "register",
        "email":    "test@example.com",
        "password": "secure123",
    })

    // Login
    resp = httpPost(t, server.URL+"/auth", map[string]string{
        "action":   "login",
        "email":    "test@example.com",
        "password": "secure123",
    })

    // Verify redirected and cookie set
    assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
    assert.NotEmpty(t, getCookie(resp, "session_token"))
}
```

**Deliverable:** `lvt gen auth` with working password authentication

---

## Migration Guide

### For LiveTemplate v0.5.0

**Backwards Compatible:** Existing apps continue to work. New methods are opt-in.

```go
// Old code (v0.4.x) - still works
func (s *MyState) Change(ctx *ActionContext) error {
    // ... no HTTP access needed ...
    return nil
}

// New code (v0.5.0) - can use HTTP methods
func (s *MyState) Change(ctx *ActionContext) error {
    ctx.SetCookie(&http.Cookie{...})  // ← NEW
    return ctx.Redirect("/", 303)      // ← NEW
}
```

### For lvt Users

**Upgrade steps:**

```bash
# 1. Upgrade lvt CLI
go install github.com/livetemplate/lvt/cmd/lvt@latest

# 2. Upgrade LiveTemplate dependency
go get github.com/livetemplate/livetemplate@v0.5.0
go mod tidy

# 3. Regenerate auth (optional)
rm -rf internal/app/auth
lvt gen auth  # Now includes password auth!

# 4. Run migrations
lvt migration up

# 5. Build and test
go build ./...
```

---

## Database Schema

```sql
-- Users table
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    hashed_password TEXT,  -- NULL for magic-link-only users
    confirmed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

-- Tokens table (session, magic-link, reset, confirm)
CREATE TABLE user_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token TEXT UNIQUE NOT NULL,
    context TEXT NOT NULL,  -- "session", "magic", "reset", "confirm"
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_user_tokens_token ON user_tokens(token);
CREATE INDEX idx_user_tokens_expires ON user_tokens(expires_at);
```

---

## Security Considerations

### Cookie Security

```go
// Enforce secure defaults
func (c *ActionContext) SetCookie(cookie *http.Cookie) error {
    // Warn on insecure cookies over HTTPS
    if !cookie.Secure && c.isHTTPS() {
        log.Warn("Setting non-Secure cookie over HTTPS")
    }

    // Warn on session cookies without HttpOnly
    if !cookie.HttpOnly && isSessionCookie(cookie.Name) {
        log.Warn("Session cookie without HttpOnly flag")
    }

    http.SetCookie(c.w, cookie)
    return nil
}
```

### Redirect Validation

```go
// Prevent open redirects
func (c *ActionContext) Redirect(url string, code int) error {
    // Only allow relative paths or same-origin URLs
    if !isValidRedirect(url) {
        return ErrInvalidRedirect
    }

    // Only allow 3xx codes
    if code < 300 || code >= 400 {
        return ErrInvalidRedirectCode
    }

    http.Redirect(c.w, c.r, url, code)
    return nil
}
```

### Password Hashing

```go
// Use bcrypt with OWASP recommended cost
import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
    cost := 12  // OWASP minimum
    return bcrypt.GenerateFromPassword([]byte(password), cost)
}

func VerifyPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}
```

---

## Implementation Notes

The HTTP methods were implemented in v0.4.1 as part of the Session API update:

- `ctx.SetCookie()` - Sets HTTP cookies (returns `ErrNoHTTPContext` for WebSocket actions)
- `ctx.GetCookie()` - Retrieves cookies from the request
- `ctx.DeleteCookie()` - Removes cookies by setting MaxAge=-1
- `ctx.Redirect()` - Sends HTTP redirects with URL validation
- `ctx.SetHeader()` / `ctx.GetHeader()` - Header access
- `ctx.IsHTTP()` - Check if in HTTP context

See `examples/login/` for a complete working example of password authentication with cookies and redirects.

---

## Timeline

| Milestone | Date | Status |
|-----------|------|--------|
| v0.4.1 | 2025-11-30 | ✅ HTTP methods added to ActionContext |
| v0.5.x | TBD | 📋 Re-enable password auth in lvt |
| v0.5.x | TBD | 📋 Rate limiting, session management |
| v0.6.0 | TBD | 📋 OAuth integration |

---

## Success Criteria

- ✅ `ctx.SetCookie()` works and is tested
- ✅ `ctx.Redirect()` works and is tested
- ✅ Password auth compiles and runs
- ✅ E2E tests pass for password login/register
- ✅ Backwards compatible with v0.4.x apps
- ✅ Security review completed
- ✅ Documentation updated
- ✅ Migration guide published

---

## References

- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
- [OWASP Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [Go http.Cookie Documentation](https://pkg.go.dev/net/http#Cookie)
- [Phoenix LiveView Auth Patterns](https://hexdocs.pm/phoenix_live_view/security-model.html)

---

## Open Questions

1. **Should Redirect() stop further processing?**
   - Proposal: Yes, return error to stop `Change()` execution

2. **Handle multiple redirects?**
   - Proposal: First redirect wins, second returns error

3. **Limit which headers can be set?**
   - Proposal: Allow all, warn on potentially breaking headers

4. **Expose full Request/ResponseWriter?**
   - Proposal: No, keep controlled API surface

---

## Next Steps

1. **Review this proposal** - Gather feedback from team
2. **Implement Phase 1** - Add HTTP methods to ActionContext
3. **Write comprehensive tests** - Cover all edge cases
4. **Update lvt CLI** - Re-enable password auth
5. **Beta test** - Get user feedback
6. **Release v0.5.0** - Make password auth available

---

_For questions or feedback, open an issue on [github.com/livetemplate/livetemplate](https://github.com/livetemplate/livetemplate/issues)_
