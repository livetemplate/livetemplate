# WebSocket Patterns with LiveTemplate

## Pattern 1: New Page Per Connection (Current Counter Example)
Good for: Independent per-client state, real-time apps where each user has their own data

```go
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, _ := s.upgrader.Upgrade(w, r, nil)
    defer conn.Close()
    
    // Create new page for this WebSocket connection
    page, err := s.app.NewPage("counter", initialData)
    if err != nil {
        log.Printf("Error creating page: %v", err)
        return
    }
    defer page.Close()
    
    // Send token to client
    tokenMessage := map[string]any{
        "type":  "page_token",
        "token": page.GetToken(),
    }
    conn.WriteJSON(tokenMessage)
    
    // Handle updates...
}
```

## Pattern 2: Shared Page via Token (Better for Most Apps)
Good for: Shared state, resumable sessions, multiple device sync

### Step 1: HTTP Handler Creates Page
```go
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
    // Create page (don't close it!)
    page, err := s.app.NewPage("dashboard", data)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Embed token in HTML for WebSocket to use
    html, _ := page.Render()
    html = strings.ReplaceAll(html, "{{PAGE_TOKEN}}", page.GetToken())
    
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(html))
    
    // Don't close page - WebSocket will use it
}
```

### Step 2: WebSocket Gets Existing Page
```go  
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, _ := s.upgrader.Upgrade(w, r, nil)
    defer conn.Close()
    
    // Get token from query param, header, or WebSocket message
    token := r.URL.Query().Get("token")
    
    // Get existing page by token
    page, err := s.app.GetPage(token)
    if err != nil {
        log.Printf("Error getting page: %v", err)
        return
    }
    
    // Handle updates to existing page...
    // No need to send token - client already has it
}
```

### Step 3: HTML Template
```html
<script>
    // Token embedded from server
    const pageToken = "{{PAGE_TOKEN}}"; 
    
    const client = new LiveTemplateClient({
        // Connect with existing token
        presetToken: pageToken
    });
    
    client.connect(`ws://localhost:8080/ws?token=${pageToken}`);
</script>
```

## API Method Summary

### Old (Verbose)
- `app.NewApplicationPage(tmpl, data)` → `app.NewPage("template-name", data)`
- `app.GetApplicationPage(token)` → `app.GetPage(token)`

### New (Clean)
- `app.NewPage(templateName, data)` - Create new page from registered template
- `app.GetPage(token)` - Get existing page by token
- `app.RegisterTemplate(name, template)` - Register template for reuse
- `app.RegisterTemplateFromFile(name, file)` - Register from file