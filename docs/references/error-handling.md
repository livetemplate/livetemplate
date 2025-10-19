# Error Handling Reference

Complete guide to error handling in LiveTemplate applications.

## Table of Contents

- [Overview](#overview)
- [Server-Side Errors](#server-side-errors)
- [Validation Errors](#validation-errors)
- [Template Error Display](#template-error-display)
- [Client-Side Error Handling](#client-side-error-handling)
- [Error Types](#error-types)
- [Best Practices](#best-practices)
- [Examples](#examples)

---

## Overview

LiveTemplate provides a comprehensive error handling system that automatically propagates validation errors from the server to the client and displays them in templates.

### Error Flow

```
User submits form
    ↓
Server: Change() method processes action
    ↓
Validation error occurs
    ↓
Error returned from Change()
    ↓
LiveTemplate wraps error with metadata
    ↓
Error sent to client in response
    ↓
Template re-renders with error data
    ↓
User sees error messages
```

---

## Server-Side Errors

Errors in LiveTemplate are returned from the `Change()` method of your store.

### Basic Error Return

```go
func (s *TodoState) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "delete":
        id := ctx.GetString("id")
        if id == "" {
            return fmt.Errorf("ID is required")
        }

        if err := s.deleteTodo(id); err != nil {
            return fmt.Errorf("failed to delete todo: %w", err)
        }
    }
    return nil
}
```

**When `Change()` returns an error:**
- The error is automatically sent to the client
- Template re-renders with error data available
- Form lifecycle events fire (`lvt:error`)
- No state changes are persisted

### Error Types

LiveTemplate recognizes different error types:

1. **Simple errors** - `fmt.Errorf()`, `errors.New()`
2. **Field errors** - `livetemplate.FieldError`
3. **Multiple field errors** - `livetemplate.MultiError`
4. **Validation errors** - From `go-playground/validator`

---

## Validation Errors

LiveTemplate integrates with `go-playground/validator` for field-level validation.

### Using go-playground/validator

```go
import "github.com/go-playground/validator/v10"

var validate = validator.New()

type TodoInput struct {
    Title       string `json:"title" validate:"required,min=3,max=100"`
    Description string `json:"description" validate:"max=500"`
    Priority    int    `json:"priority" validate:"min=1,max=5"`
}

func (s *TodoState) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "add":
        var input TodoInput

        // BindAndValidate automatically handles validation errors
        if err := ctx.BindAndValidate(&input, validate); err != nil {
            return err // Errors sent to client with field names
        }

        // Input is valid, proceed
        s.Todos = append(s.Todos, Todo{
            Title:       input.Title,
            Description: input.Description,
            Priority:    input.Priority,
        })
    }
    return nil
}
```

### Validation Tags

Common validation tags:

| Tag | Description | Example |
|-----|-------------|---------|
| `required` | Field must not be empty | `validate:"required"` |
| `min` | Minimum value/length | `validate:"min=3"` |
| `max` | Maximum value/length | `validate:"max=100"` |
| `email` | Valid email format | `validate:"email"` |
| `url` | Valid URL format | `validate:"url"` |
| `alpha` | Alphabetic characters only | `validate:"alpha"` |
| `numeric` | Numeric characters only | `validate:"numeric"` |
| `alphanum` | Alphanumeric characters | `validate:"alphanum"` |
| `oneof` | Value must be one of | `validate:"oneof=red green blue"` |

See [validator documentation](https://pkg.go.dev/github.com/go-playground/validator/v10) for complete list.

### Manual Field Errors

Create field-specific errors manually:

```go
func (s *State) Change(ctx *livetemplate.ActionContext) error {
    username := ctx.GetString("username")

    // Check if username already exists
    if s.usernameExists(username) {
        return livetemplate.NewFieldError("username",
            errors.New("username already taken"))
    }

    return nil
}
```

### Multiple Field Errors

Return multiple field errors at once:

```go
func (s *State) Change(ctx *livetemplate.ActionContext) error {
    var errors livetemplate.MultiError

    email := ctx.GetString("email")
    if !isValidEmail(email) {
        errors = append(errors,
            livetemplate.NewFieldError("email",
                errors.New("invalid email format")))
    }

    password := ctx.GetString("password")
    if len(password) < 8 {
        errors = append(errors,
            livetemplate.NewFieldError("password",
                errors.New("password must be at least 8 characters")))
    }

    if len(errors) > 0 {
        return errors
    }

    return nil
}
```

---

## Template Error Display

LiveTemplate provides template helpers for displaying errors.

### Error Helpers

| Helper | Description | Returns |
|--------|-------------|---------|
| `.lvt.HasError "field"` | Check if field has error | `bool` |
| `.lvt.Error "field"` | Get error message for field | `string` |
| `.lvt.Errors` | Get all errors | `map[string]string` |

### Basic Error Display

```html
<form lvt-submit="save">
    <div>
        <label for="email">Email</label>
        <input
            type="email"
            id="email"
            name="email"
            {{if .lvt.HasError "email"}}aria-invalid="true"{{end}}>

        {{if .lvt.HasError "email"}}
            <small class="error">{{.lvt.Error "email"}}</small>
        {{end}}
    </div>

    <button type="submit">Save</button>
</form>
```

### Styling Invalid Fields

```html
<input
    type="text"
    name="username"
    class="{{if .lvt.HasError "username"}}input-error{{end}}">
```

With CSS:
```css
.input-error {
    border-color: #ef4444;
    background-color: #fef2f2;
}
```

### Displaying All Errors

```html
{{if .lvt.Errors}}
    <div class="error-summary">
        <h4>Please fix the following errors:</h4>
        <ul>
            {{range $field, $message := .lvt.Errors}}
                <li><strong>{{$field}}:</strong> {{$message}}</li>
            {{end}}
        </ul>
    </div>
{{end}}
```

### Error Summary at Top

```html
<form lvt-submit="create">
    {{if .lvt.Errors}}
        <div class="alert alert-error">
            {{range .lvt.Errors}}
                <p>{{.}}</p>
            {{end}}
        </div>
    {{end}}

    <!-- Form fields -->
</form>
```

---

## Client-Side Error Handling

Handle errors in JavaScript using form lifecycle events.

### Form Error Event

```javascript
const form = document.querySelector('form');

form.addEventListener('lvt:error', (e) => {
    console.log('Validation failed');
    console.log('Errors:', e.detail.errors);

    // e.detail contains:
    // {
    //   action: "save",
    //   errors: {
    //     "email": "invalid email format",
    //     "password": "password too short"
    //   },
    //   meta: {
    //     success: false
    //   }
    // }
});
```

### Show Custom Error Notification

```javascript
form.addEventListener('lvt:error', (e) => {
    const errorCount = Object.keys(e.detail.errors).length;
    showNotification(`Please fix ${errorCount} error(s)`, 'error');
});
```

### Focus First Invalid Field

```javascript
form.addEventListener('lvt:error', (e) => {
    const firstErrorField = Object.keys(e.detail.errors)[0];
    const input = form.querySelector(`[name="${firstErrorField}"]`);
    if (input) {
        input.focus();
    }
});
```

### Clear Errors on Input

```javascript
document.querySelectorAll('input').forEach(input => {
    input.addEventListener('input', () => {
        // Clear error styling when user starts typing
        input.classList.remove('input-error');
        const errorMsg = input.parentElement.querySelector('.error');
        if (errorMsg) {
            errorMsg.style.display = 'none';
        }
    });
});
```

---

## Error Types

LiveTemplate provides specific error types for different scenarios.

### FieldError

Represents an error for a specific form field.

```go
type FieldError struct {
    Field   string
    Message string
}

func (e FieldError) Error() string {
    return fmt.Sprintf("%s: %s", e.Field, e.Message)
}
```

**Usage:**
```go
return livetemplate.NewFieldError("email", errors.New("email already exists"))
```

### MultiError

Collection of field errors.

```go
type MultiError []FieldError

func (m MultiError) Error() string {
    // Returns concatenated error messages
}
```

**Usage:**
```go
var errs livetemplate.MultiError
errs = append(errs, livetemplate.NewFieldError("email", errors.New("invalid")))
errs = append(errs, livetemplate.NewFieldError("password", errors.New("too short")))
return errs
```

### ValidationError

Automatically created by `BindAndValidate()` when using `go-playground/validator`.

```go
// Automatically converts validator errors to MultiError
if err := ctx.BindAndValidate(&input, validate); err != nil {
    return err // Returns MultiError with field names
}
```

---

## Best Practices

### 1. Use Specific Error Messages

❌ **Bad:**
```go
return errors.New("invalid input")
```

✅ **Good:**
```go
return livetemplate.NewFieldError("email",
    errors.New("email must be a valid email address"))
```

### 2. Validate Early

```go
func (s *State) Change(ctx *livetemplate.ActionContext) error {
    // Validate input first
    var input TodoInput
    if err := ctx.BindAndValidate(&input, validate); err != nil {
        return err
    }

    // Then perform business logic
    if err := s.saveTodo(input); err != nil {
        return fmt.Errorf("failed to save: %w", err)
    }

    return nil
}
```

### 3. Show Errors Near Fields

✅ **Good UX:**
```html
<input name="email">
{{if .lvt.HasError "email"}}
    <small class="error">{{.lvt.Error "email"}}</small>
{{end}}
```

### 4. Use Accessible Error Attributes

```html
<input
    name="email"
    {{if .lvt.HasError "email"}}
        aria-invalid="true"
        aria-describedby="email-error"
    {{end}}>

{{if .lvt.HasError "email"}}
    <span id="email-error" role="alert">
        {{.lvt.Error "email"}}
    </span>
{{end}}
```

### 5. Preserve User Input on Error

LiveTemplate automatically preserves form data on error. No special handling needed.

### 6. Handle Non-Field Errors

For errors that don't belong to a specific field:

```go
// Return general error
return errors.New("database connection failed")
```

Display in template:
```html
{{if .lvt.Errors}}
    {{if .lvt.Error ""}}
        <div class="alert alert-error">
            {{.lvt.Error ""}}
        </div>
    {{end}}
{{end}}
```

---

## Examples

### Complete Form with Error Handling

**Server:**
```go
type SignupInput struct {
    Username string `json:"username" validate:"required,min=3,max=20,alphanum"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}

func (s *AuthState) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "signup":
        var input SignupInput

        // Validate input
        if err := ctx.BindAndValidate(&input, validate); err != nil {
            return err
        }

        // Check if username exists
        if s.usernameExists(input.Username) {
            return livetemplate.NewFieldError("username",
                errors.New("username already taken"))
        }

        // Check if email exists
        if s.emailExists(input.Email) {
            return livetemplate.NewFieldError("email",
                errors.New("email already registered"))
        }

        // Create user
        if err := s.createUser(input); err != nil {
            return fmt.Errorf("failed to create account: %w", err)
        }
    }
    return nil
}
```

**Template:**
```html
<form lvt-submit="signup">
    <h2>Sign Up</h2>

    {{if .lvt.Errors}}
        <div class="alert alert-error">
            <p>Please fix the errors below</p>
        </div>
    {{end}}

    <div class="form-group">
        <label for="username">Username</label>
        <input
            type="text"
            id="username"
            name="username"
            class="{{if .lvt.HasError "username"}}input-error{{end}}"
            {{if .lvt.HasError "username"}}aria-invalid="true"{{end}}>
        {{if .lvt.HasError "username"}}
            <small class="error">{{.lvt.Error "username"}}</small>
        {{end}}
    </div>

    <div class="form-group">
        <label for="email">Email</label>
        <input
            type="email"
            id="email"
            name="email"
            class="{{if .lvt.HasError "email"}}input-error{{end}}"
            {{if .lvt.HasError "email"}}aria-invalid="true"{{end}}>
        {{if .lvt.HasError "email"}}
            <small class="error">{{.lvt.Error "email"}}</small>
        {{end}}
    </div>

    <div class="form-group">
        <label for="password">Password</label>
        <input
            type="password"
            id="password"
            name="password"
            class="{{if .lvt.HasError "password"}}input-error{{end}}"
            {{if .lvt.HasError "password"}}aria-invalid="true"{{end}}>
        {{if .lvt.HasError "password"}}
            <small class="error">{{.lvt.Error "password"}}</small>
        {{end}}
        <small class="help">Must be at least 8 characters</small>
    </div>

    <button type="submit" class="btn-primary">Sign Up</button>
</form>
```

**JavaScript:**
```javascript
const form = document.querySelector('form');

form.addEventListener('lvt:error', (e) => {
    // Focus first invalid field
    const firstField = Object.keys(e.detail.errors)[0];
    const input = form.querySelector(`[name="${firstField}"]`);
    if (input) {
        input.focus();
    }

    // Show notification
    showNotification('Please fix the errors in the form', 'error');
});

form.addEventListener('lvt:success', (e) => {
    showNotification('Account created successfully!', 'success');
    // Redirect or clear form
});
```

---

## Related Documentation

- **[Client Attributes Reference](client-attributes.md)** - Form lifecycle events
- **[Go API Reference](https://pkg.go.dev/github.com/livefir/livetemplate)** - Error types API
- **[go-playground/validator](https://pkg.go.dev/github.com/go-playground/validator/v10)** - Validation tags
- **[Template Support Matrix](template-support-matrix.md)** - Template syntax
