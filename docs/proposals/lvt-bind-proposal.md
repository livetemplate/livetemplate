# lvt-bind: Automatic Form State Binding Proposal

**Status:** Future Feature
**Date:** 2025-10-01
**Complexity:** Medium-High (~970 lines of code)

## Overview

Add automatic two-way binding between HTML form inputs and server state using the `lvt-bind` attribute. Form input changes automatically sync to server state, with server-side validation and error handling.

## Goals

1. **Eliminate boilerplate**: No manual `Change()` methods for simple field updates
2. **Automatic syncing**: Input changes automatically update server state
3. **Server validation**: Validate inputs on server-side, display errors in template
4. **Template-driven**: Use existing template expressions (`{{.FieldName}}`) for bindings
5. **Backwards compatible**: Works alongside existing `lvt-click` action system

## Evolution of the Idea

### Initial Concept
Started with complex state binding using `data-bind` attributes and JavaScript state proxies. Realized this was over-engineered.

### Key Simplifications

1. **No `data-bind` needed**: Template expressions like `{{.Counter}}` already define bindings
2. **No state proxy needed**: Just send field updates to server, let template handle rendering
3. **No `lvt-bind` attribute needed on root**: Can auto-detect all forms/inputs
4. **Keep `lvt-click`**: Maintain declarative action syntax for complex operations
5. **Template-based errors**: Render errors in template, not via JavaScript DOM manipulation

## Final Design

### 1. Binding Scope

**Option A: Explicit Root Binding** (Recommended)
```html
<div lvt-bind>
    <input type="text" name="Title" value="{{.Title}}">
    <input type="number" name="Counter" value="{{.Counter}}">
</div>
```

**Option B: Automatic (All inputs bind by default)**
```html
<!-- No lvt-bind needed, all inputs auto-bind -->
<input type="text" name="Title" value="{{.Title}}">
```

**Option C: Individual Input Binding**
```html
<input name="Counter" value="{{.Counter}}" lvt-bind>
```

### 2. Update Triggers

```html
<!-- Update on change (default) -->
<div lvt-bind>

<!-- Update on blur -->
<div lvt-bind="blur">

<!-- Update on form submit -->
<form lvt-bind="submit">

<!-- Debounced updates (e.g., for search) -->
<input name="search" lvt-bind="debounce:500">
```

### 3. Server-Side Validation (Template-Based)

**Server state includes errors:**
```go
type FormState struct {
    Name   string            `json:"name"`
    Email  string            `json:"email"`
    Age    int               `json:"age"`
    Errors map[string]string `json:"errors"` // Field -> error message
}

// Implement validation interface
func (s *FormState) Validate(fields map[string]interface{}) map[string]string {
    errors := make(map[string]string)

    if name, ok := fields["Name"].(string); ok {
        if len(name) < 2 {
            errors["Name"] = "Name must be at least 2 characters"
        }
    }

    if email, ok := fields["Email"].(string); ok {
        if !strings.Contains(email, "@") {
            errors["Email"] = "Invalid email address"
        }
    }

    // Store in state so template can render them
    s.Errors = errors
    return errors
}
```

**Template renders errors:**
```html
<div lvt-bind="blur">
    <div class="field">
        <label>Name:</label>
        <input type="text" name="Name" value="{{.Name}}"
               class="{{if .Errors.Name}}error{{end}}">
        {{if .Errors.Name}}
            <div class="error-message">{{.Errors.Name}}</div>
        {{end}}
    </div>

    <div class="field">
        <label>Email:</label>
        <input type="email" name="Email" value="{{.Email}}"
               class="{{if .Errors.Email}}error{{end}}">
        {{if .Errors.Email}}
            <div class="error-message">{{.Errors.Email}}</div>
        {{end}}
    </div>
</div>

<style>
    .error { border: 2px solid red; }
    .error-message { color: red; font-size: 12px; margin-top: 4px; }
</style>
```

## Protocol Design

### Client → Server: Field Update
```json
{
    "type": "bind",
    "fields": {
        "Counter": 5,
        "Title": "New Title"
    }
}
```

### Server → Client: Success (Tree Diff)
```json
{
    "s": ["<p>Counter: ", "</p>"],
    "0": "5"
}
```

### Server → Client: Validation Errors
Server updates state with errors, returns tree diff showing error messages:
```json
{
    "s": ["<div class=\"error-message\">", "</div>"],
    "0": "Name must be at least 2 characters"
}
```

No special error protocol needed - errors are just part of the state rendered by template!

## Implementation Components

### 1. Server-Side (Go)

**New file: `bind.go`** (~150 lines)
```go
package livetemplate

// BindValidator interface for stores that validate field updates
type BindValidator interface {
    Validate(fields map[string]interface{}) map[string]string
}

// ApplyFields uses reflection to update struct fields
func ApplyFields(store interface{}, fields map[string]interface{}) error {
    storeValue := reflect.ValueOf(store).Elem()

    for fieldName, value := range fields {
        field := storeValue.FieldByName(fieldName)
        if !field.IsValid() || !field.CanSet() {
            return fmt.Errorf("field %s not found or not settable", fieldName)
        }

        // Convert JSON value to field type
        convertedValue := convertToFieldType(value, field.Type())
        field.Set(convertedValue)
    }

    return nil
}

// ValidateAndApply validates then applies field updates
func ValidateAndApply(store interface{}, fields map[string]interface{}) error {
    // Run validation if store implements BindValidator
    if validator, ok := store.(BindValidator); ok {
        errors := validator.Validate(fields)
        if len(errors) > 0 {
            // Validation failed, but errors are stored in state
            // Template will render them
            return nil
        }
    }

    // Apply field updates
    return ApplyFields(store, fields)
}

// Type conversion helpers
func convertToFieldType(value interface{}, targetType reflect.Type) reflect.Value {
    // Handle JSON number -> int conversion
    // Handle string -> various type conversions
    // etc.
}
```

**Modify: `action.go`** (~30 lines)
```go
// Add bind message type
type BindMessage struct {
    Type   string                 `json:"type"`   // "bind"
    Fields map[string]interface{} `json:"fields"` // Field name -> value
}

func ParseBindMessage(data []byte) (BindMessage, error) {
    var msg BindMessage
    if err := json.Unmarshal(data, &msg); err != nil {
        return BindMessage{}, err
    }
    return msg, nil
}
```

**Modify: `mount.go`** (~80 lines)
```go
func (h *liveHandler) handleMessage(data []byte, stores Stores, tmpl *Template) ([]byte, error) {
    // Detect message type
    var msgType struct {
        Type string `json:"type"`
    }
    json.Unmarshal(data, &msgType)

    if msgType.Type == "bind" {
        return h.handleBind(data, stores, tmpl)
    }

    // Fall back to action (existing behavior)
    return h.handleAction(data, stores, tmpl)
}

func (h *liveHandler) handleBind(data []byte, stores Stores, tmpl *Template) ([]byte, error) {
    bindMsg, err := ParseBindMessage(data)
    if err != nil {
        return nil, err
    }

    store := stores[""] // Single store for now

    // Validate and apply field updates
    err = ValidateAndApply(store, bindMsg.Fields)
    if err != nil {
        return nil, err
    }

    // Generate tree diff (same as existing flow)
    var buf bytes.Buffer
    err = tmpl.ExecuteUpdates(&buf, store)
    if err != nil {
        return nil, err
    }

    return buf.Bytes(), nil
}
```

### 2. Client-Side (TypeScript)

**Modify: `client/livetemplate-client.ts`** (~200 lines)

```typescript
class BindManager {
    private client: LiveTemplateClient;

    constructor(client: LiveTemplateClient) {
        this.client = client;
    }

    // Initialize all bindings
    initialize(): void {
        // Find all lvt-bind elements
        document.querySelectorAll('[lvt-bind]').forEach(element => {
            const trigger = element.getAttribute('lvt-bind') || 'change';
            this.setupBinding(element, trigger);
        });
    }

    private setupBinding(element: Element, trigger: string): void {
        const inputs = Array.from(
            element.querySelectorAll('input, select, textarea')
        ) as HTMLInputElement[];

        inputs.forEach(input => {
            if (trigger === 'blur') {
                input.addEventListener('blur', () => this.handleFieldChange(input));
            } else if (trigger === 'change') {
                input.addEventListener('change', () => this.handleFieldChange(input));
            } else if (trigger.startsWith('debounce:')) {
                const delay = parseInt(trigger.split(':')[1]);
                this.setupDebounce(input, delay);
            }
        });

        // Handle form submit if element is a form
        if (element.tagName === 'FORM') {
            element.addEventListener('submit', (e) => {
                e.preventDefault();
                this.handleFormSubmit(element as HTMLFormElement);
            });
        }
    }

    private handleFieldChange(input: HTMLInputElement): void {
        const fieldName = input.name;
        const value = this.extractValue(input);

        this.sendFieldUpdate({ [fieldName]: value });
    }

    private handleFormSubmit(form: HTMLFormElement): void {
        const formData = new FormData(form);
        const fields: Record<string, any> = {};

        formData.forEach((value, key) => {
            fields[key] = value;
        });

        this.sendFieldUpdate(fields);
    }

    private extractValue(input: HTMLInputElement): any {
        switch (input.type) {
            case 'checkbox':
                return input.checked;
            case 'number':
                return parseFloat(input.value);
            case 'radio':
                return input.checked ? input.value : null;
            default:
                return input.value;
        }
    }

    private sendFieldUpdate(fields: Record<string, any>): void {
        const message = {
            type: 'bind',
            fields: fields
        };

        // Use existing send mechanism
        this.client.send(JSON.stringify(message));
    }
}

// Integrate into LiveTemplateClient
class LiveTemplateClient {
    private bindManager: BindManager | null = null;

    static autoInit(): void {
        // ... existing code ...

        // Initialize bind manager
        client.bindManager = new BindManager(client);
        client.bindManager.initialize();
    }
}
```

### 3. Example Implementation

**New: `examples/form-bind/main.go`**
```go
package main

import (
    "log"
    "net/http"
    "strings"
    "time"
    "github.com/livefir/livetemplate"
)

type UserForm struct {
    Name    string            `json:"name"`
    Email   string            `json:"email"`
    Age     int               `json:"age"`
    Bio     string            `json:"bio"`
    Updated string            `json:"updated"`
    Errors  map[string]string `json:"errors"`
}

// Implement BindValidator
func (f *UserForm) Validate(fields map[string]interface{}) map[string]string {
    errors := make(map[string]string)

    // Validate name
    if name, ok := fields["Name"].(string); ok {
        f.Name = name
        if len(name) < 2 {
            errors["Name"] = "Name must be at least 2 characters"
        }
    }

    // Validate email
    if email, ok := fields["Email"].(string); ok {
        f.Email = email
        if !strings.Contains(email, "@") {
            errors["Email"] = "Invalid email address"
        }
    }

    // Validate age
    if age, ok := fields["Age"].(float64); ok {
        f.Age = int(age)
        if age < 18 || age > 120 {
            errors["Age"] = "Age must be between 18 and 120"
        }
    }

    // Update timestamp if validation passed
    if len(errors) == 0 {
        f.Updated = time.Now().Format(time.RFC3339)
        f.Errors = nil
    } else {
        f.Errors = errors
    }

    return errors
}

func main() {
    state := &UserForm{
        Name:    "",
        Email:   "",
        Age:     18,
        Bio:     "",
        Updated: time.Now().Format(time.RFC3339),
        Errors:  make(map[string]string),
    }

    tmpl := livetemplate.New("form")
    tmpl.ParseFiles("form.html")

    http.Handle("/", tmpl.Handle(state))
    // In production: serve from CDN
    // For development: use internal/testing.ServeClientLibrary
    http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

    log.Println("Server starting on :8080")
    http.ListenAndServe(":8080", nil)
}
```

**New: `examples/form-bind/form.html`**
```html
<!DOCTYPE html>
<html>
<head>
    <title>Form Binding Example</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 600px; margin: 40px auto; }
        .field { margin-bottom: 20px; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
        input, textarea, select {
            width: 100%;
            padding: 8px;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 14px;
        }
        input.error, textarea.error {
            border-color: #e74c3c;
            background-color: #fff5f5;
        }
        .error-message {
            color: #e74c3c;
            font-size: 12px;
            margin-top: 5px;
        }
        .preview {
            background: #f8f9fa;
            padding: 20px;
            border-radius: 4px;
            margin-top: 30px;
        }
        .preview h3 { margin-top: 0; }
    </style>
</head>
<body>
    <h1>User Profile Form</h1>

    <!-- All inputs within this div auto-bind on blur -->
    <div lvt-bind="blur">
        <div class="field">
            <label>Name:</label>
            <input type="text"
                   name="Name"
                   value="{{.Name}}"
                   placeholder="Enter your name"
                   class="{{if .Errors.Name}}error{{end}}">
            {{if .Errors.Name}}
                <div class="error-message">{{.Errors.Name}}</div>
            {{end}}
        </div>

        <div class="field">
            <label>Email:</label>
            <input type="email"
                   name="Email"
                   value="{{.Email}}"
                   placeholder="your@email.com"
                   class="{{if .Errors.Email}}error{{end}}">
            {{if .Errors.Email}}
                <div class="error-message">{{.Errors.Email}}</div>
            {{end}}
        </div>

        <div class="field">
            <label>Age:</label>
            <input type="number"
                   name="Age"
                   value="{{.Age}}"
                   min="18"
                   max="120"
                   class="{{if .Errors.Age}}error{{end}}">
            {{if .Errors.Age}}
                <div class="error-message">{{.Errors.Age}}</div>
            {{end}}
        </div>

        <div class="field">
            <label>Bio:</label>
            <textarea name="Bio" rows="4" placeholder="Tell us about yourself">{{.Bio}}</textarea>
        </div>
    </div>

    <!-- Preview section shows current state -->
    <div class="preview">
        <h3>Current State</h3>
        <p><strong>Name:</strong> {{.Name}}</p>
        <p><strong>Email:</strong> {{.Email}}</p>
        <p><strong>Age:</strong> {{.Age}}</p>
        <p><strong>Bio:</strong> {{.Bio}}</p>
        <p><strong>Last updated:</strong> {{.Updated}}</p>
    </div>

    <script src="/livetemplate-client.js"></script>
</body>
</html>
```

## Key Design Decisions

### 1. Template-Based Error Rendering
**Decision:** Render errors in template, not via JavaScript DOM manipulation

**Rationale:**
- Consistent with LiveTemplate's template-driven approach
- Uses existing tree diff mechanism
- Errors automatically clear when state updates
- No manual DOM manipulation needed

### 2. Keep `lvt-click` Actions
**Decision:** Maintain existing action system alongside bindings

**Rationale:**
- Actions are better for complex business logic
- Bindings are better for simple field updates
- Both approaches complement each other
- No breaking changes

### 3. Explicit `lvt-bind` Attribute
**Decision:** Require `lvt-bind` attribute on container element

**Rationale:**
- Opt-in behavior (not all forms need auto-binding)
- Clear intent in template
- Allows different triggers per section
- Easy to disable binding for specific forms

### 4. Flexible Update Triggers
**Decision:** Support multiple trigger modes (change, blur, submit, debounce)

**Rationale:**
- Different use cases need different update strategies
- Search inputs need debouncing
- Complex forms benefit from blur or submit
- Simple counters can use immediate change

## Implementation Estimates

### Code Changes
- **New:** `bind.go` (~150 lines)
- **New:** `examples/form-bind/main.go` (~80 lines)
- **New:** `examples/form-bind/form.html` (~100 lines)
- **New:** `examples/form-bind/form_test.go` (~200 lines)
- **Modify:** `action.go` (~30 lines added)
- **Modify:** `mount.go` (~80 lines added)
- **Modify:** `client/livetemplate-client.ts` (~200 lines added)

**Total:** ~840 new lines, ~310 modified lines = **~1150 lines of code**

### Development Time Estimate
- Server-side binding handler: 4 hours
- Client-side bind manager: 6 hours
- Type conversion and validation: 3 hours
- Form example: 2 hours
- E2E tests: 4 hours
- Documentation: 2 hours

**Total:** ~21 hours (3 days)

### Testing Requirements
1. Unit tests for field type conversion
2. Unit tests for validation logic
3. E2E tests for form submission
4. E2E tests for validation errors
5. E2E tests for different trigger modes
6. WebSocket vs HTTP fallback tests
7. Multi-field update tests

## Benefits

1. **Less boilerplate**: No `Change()` methods for simple field updates
2. **Automatic sync**: Form inputs auto-sync to server without manual actions
3. **Server validation**: Business logic validation stays on server
4. **Type safety**: Go struct fields with proper types
5. **Template-driven**: Uses existing template system for rendering
6. **Backwards compatible**: Works with existing `lvt-click` actions
7. **Flexible**: Different trigger modes for different use cases

## Limitations & Future Enhancements

### Current Limitations
1. Single store only (no multi-store support yet)
2. No nested field binding (e.g., `User.Address.City`)
3. No array/slice field updates
4. No file upload handling
5. No custom validators (besides `Validate()` method)

### Future Enhancements
1. **Nested field binding**: Support `Address.City` paths
2. **Array operations**: Add/remove items in slices
3. **File uploads**: Handle file input binding
4. **Custom validators**: Pluggable validation system
5. **Optimistic updates**: Update UI immediately, rollback on error
6. **Batch updates**: Group multiple field changes into single request
7. **Computed fields**: Auto-update derived fields

## Alternatives Considered

### Alternative 1: Client-Side State Proxy
Create JavaScript Proxy for reactive state mutations.

**Rejected because:**
- Requires custom JavaScript in templates
- More complex implementation
- Doesn't leverage existing template system

### Alternative 2: Auto-Bind All Inputs
Automatically bind all inputs without `lvt-bind` attribute.

**Rejected because:**
- Less explicit (magic behavior)
- Can't selectively disable binding
- Harder to have different trigger modes

### Alternative 3: JavaScript DOM Manipulation for Errors
Insert error divs via JavaScript instead of template rendering.

**Rejected because:**
- Inconsistent with template-driven approach
- Requires manual DOM manipulation
- More complex client code

## References

### Similar Approaches in Other Frameworks
- **Phoenix LiveView**: Form bindings with `phx-change` and `phx-submit`
- **Laravel Livewire**: Wire:model for two-way binding
- **Hotwire Turbo**: Form submissions with Turbo Frames
- **HTMX**: hx-post, hx-trigger for form handling

### Internal References
- Current action system: `action.go`, `mount.go`
- Tree diff system: `template.go`, `tree.go`
- Client library: `client/livetemplate-client.ts`

## Next Steps

When ready to implement:

1. Start with server-side foundation (`bind.go`)
2. Add client-side bind detection and field extraction
3. Implement validation and error handling
4. Create form example with all features
5. Write comprehensive E2E tests
6. Update documentation with usage examples

## Questions to Resolve Before Implementation

1. **Trigger defaults**: Should `lvt-bind` without value default to "change" or "blur"?
2. **Error clearing**: Should errors auto-clear on next input, or only after validation passes?
3. **Multi-store**: Should we support `lvt-bind="storeName"` for multi-store apps?
4. **Nested fields**: How to handle nested struct updates (e.g., `User.Address.City`)?
5. **Type conversion errors**: How to handle invalid type conversions (e.g., "abc" for number field)?
