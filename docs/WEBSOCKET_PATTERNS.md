# WebSocket Patterns with LiveTemplate

> **Prerequisites**: All patterns assume templates are registered at startup with `app.ParseFiles("templates/*.html")`

## Pattern 1: New Page Per Connection (Current Counter Example)
Good for: Independent per-client state, real-time apps where each user has their own data

```go
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, _ := s.upgrader.Upgrade(w, r, nil)
    defer conn.Close()
    
    // Create new page for this WebSocket connection (template already registered)
    page, err := s.app.NewPage("counter", initialData)
    if err != nil {
        log.Printf("Error creating page: %v", err)
        return
    }
    
    // Send token to client
    tokenMessage := map[string]any{
        "type":  "page_token",
        "token": page.GetToken(),
    }
    conn.WriteJSON(tokenMessage)
    
    // Handle action messages
    for {
        var actionMsg livetemplate.ActionMessage
        if err := conn.ReadJSON(&actionMsg); err != nil {
            break
        }
        
        if err := s.app.HandleAction(r); err != nil {
            log.Printf("Action failed: %v", err)
        }
    }
}
```

## Pattern 2: Shared Page via Token (Better for Most Apps)
Good for: Shared state, resumable sessions, multiple device sync

### Step 1: HTTP Handler Creates Page
```go
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
    // Create page (template already registered with ParseFiles)
    page, err := s.app.NewPage("dashboard", data)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Serve HTML directly with token embedded for client
    if err := page.ServeHTTP(w, data); err != nil {
        log.Printf("Serve failed: %v", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }
    
    // Page persists for WebSocket to use via app.GetPage(r)
}
```

### Step 2: WebSocket Gets Existing Page
```go  
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, _ := s.upgrader.Upgrade(w, r, nil)
    defer conn.Close()
    
    // Get existing page from request (handles session authentication)
    page, err := s.app.GetPage(r)
    if err != nil {
        log.Printf("Error getting page: %v", err)
        return
    }
    
    // Handle action messages for existing page
    for {
        var actionMsg livetemplate.ActionMessage
        if err := conn.ReadJSON(&actionMsg); err != nil {
            break
        }
        
        if err := s.app.HandleAction(r); err != nil {
            log.Printf("Action failed: %v", err)
        }
    }
}
```

### Step 3: Client-Side Setup

**Template Integration:**
LiveTemplate automatically embeds session information in the served HTML. The client library handles authentication transparently:

```html
<script src="livetemplate-client.js"></script>
<script>
    const client = new LiveTemplateClient();
    client.connect(); // Automatically uses embedded session data
</script>
```

**Manual Token Passing (if needed):**
```javascript
// Only needed if you want to manually handle the token
const token = page.GetToken(); // Server-side
const client = new LiveTemplateClient();
client.connect(`ws://localhost:8080/ws?cache_token=${token}`);
```

## API Method Summary

### Application Setup (Once at Startup)
- `app := livetemplate.NewApplication()` - Create application
- `app.ParseFiles("templates/page.html")` - Parse and register template (name = filename without extension)

### Page Management
- `app.NewPage(templateName, data)` - Create new page from registered template
- `app.GetPage(r)` - Get existing page from HTTP request (handles authentication)
- `page.ServeHTTP(w, data)` - Serve HTML to client
- `page.GetToken()` - Get session token for WebSocket authentication
- `page.RegisterDataModel(model)` - Register data model with action methods

### WebSocket Actions
- `app.HandleAction(r)` - Process action messages (calls data model methods)