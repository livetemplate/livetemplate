# Building a Real-Time Chat App with LiveTemplate

A complete tutorial for building a real-time chat application using LiveTemplate's simple kit. This demonstrates **automatic multi-tab syncing**, session management, and reactive UI updates with just **2 files**.

## What You'll Build

- Real-time messaging with automatic tab syncing
- User login and presence tracking
- Instant UI updates across all tabs in the same browser
- Browser session isolation (each browser has its own chat room)
- Message history and timestamps

**All in just 2 files: `main.go` and `chat.tmpl`**

## Quick Start

```bash
cd examples/chat
GOWORK=off go run main.go
```

Then open <http://localhost:8090> in **multiple browser tabs** to see automatic syncing in action:
- Messages sent in one tab appear instantly in all other tabs
- Each browser gets its own isolated chat session

## Tutorial: Building from Scratch

### Step 1: Create a New App

Start by creating a new LiveTemplate application with the `simple` kit:

```bash
lvt new chat --kit simple
cd chat
```

The `simple` kit generates a minimal structure:

- `main.go` - Application logic (single file)
- `chat.tmpl` - HTML template (single file)
- `go.mod` - Go module configuration
- `README.md` - Documentation

No cmd/, internal/, or database directories. Perfect for focused applications!

### Step 2: Define the Chat State

Open `main.go` and replace the counter example with chat state:

```go
package main

import (
    "log"
    "net/http"
    "os"
    "sync"
    "time"

    "github.com/livefir/livetemplate"
)

type ChatState struct {
    Messages      []Message
    Users         map[string]*User
    CurrentUser   string
    OnlineCount   int
    TotalMessages int
    mu            sync.RWMutex  // Thread-safe access
}

type Message struct {
    ID        int
    Username  string
    Text      string
    Timestamp string
}

type User struct {
    Username string
    JoinedAt time.Time
    IsOnline bool
}
```

**Key concepts:**

- Single `ChatState` struct holds all app state
- `sync.RWMutex` for thread-safe concurrent access
- Simple Go structs - no database, no ORM, no complexity

### Step 3: Implement Actions

Add the `Change` method to handle user actions:

```go
func (s *ChatState) Change(ctx *livetemplate.ActionContext) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    switch ctx.Action {
    case "send":
        var data struct {
            Message string `json:"message"`
        }

        if err := ctx.Bind(&data); err != nil {
            return nil
        }

        if data.Message == "" {
            return nil
        }

        s.TotalMessages++
        msg := Message{
            ID:        s.TotalMessages,
            Username:  s.CurrentUser,
            Text:      data.Message,
            Timestamp: time.Now().Format("15:04:05"),
        }

        s.Messages = append(s.Messages, msg)
        return nil  // Auto-syncs to all tabs in same browser!

    case "join":
        var data struct {
            Username string `json:"username"`
        }

        if err := ctx.Bind(&data); err != nil {
            return nil
        }

        s.CurrentUser = data.Username

        if _, exists := s.Users[data.Username]; !exists {
            s.Users[data.Username] = &User{
                Username: data.Username,
                JoinedAt: time.Now(),
                IsOnline: true,
            }
            s.updateOnlineCount()
        }

        return nil
    }

    return nil
}

func (s *ChatState) updateOnlineCount() {
    count := 0
    for _, user := range s.Users {
        if user.IsOnline {
            count++
        }
    }
    s.OnlineCount = count
}
```

**Key concepts:**

- `ctx.Action` comes from HTML `lvt-submit="action"` attribute
- `ctx.Bind(&data)` extracts form data
- Just modify state - broadcasting happens automatically!
- No manual WebSocket code needed

### Step 4: Initialize and Run

Add initialization and main function:

```go
func (s *ChatState) Init() error {
    if s.Users == nil {
        s.Users = make(map[string]*User)
    }
    if s.Messages == nil {
        s.Messages = []Message{}
    }
    return nil
}

func main() {
    log.Println("chat starting...")

    state := &ChatState{
        Users:    make(map[string]*User),
        Messages: []Message{},
    }

    tmpl := livetemplate.New("chat", livetemplate.WithDevMode(true))
    http.Handle("/", tmpl.Handle(state))

    // Serve client library for development
    http.HandleFunc("/livetemplate-client.js", serveClientLibrary)

    port := os.Getenv("PORT")
    if port == "" {
        port = "8090"
    }

    log.Printf("🚀 Chat server starting on http://localhost:%s", port)
    log.Println("📝 Open multiple browser tabs to test multi-user chat")
    log.Println("💬 Messages are broadcast to all connected users")

    http.ListenAndServe(":"+port, nil)
}
```

### Step 5: Create the UI

Replace `chat.tmpl` with the chat interface. Key template concepts:

**Conditional Rendering:**

```html
{{if not .CurrentUser}}
    <!-- Show login form -->
{{else}}
    <!-- Show chat interface -->
{{end}}
```

**Message Loop:**

```html
{{range .Messages}}
<div class="message {{if eq .Username $.CurrentUser}}mine{{end}}">
    <div class="message-header">
        <span class="message-username">{{.Username}}</span>
        <span class="message-time">{{.Timestamp}}</span>
    </div>
    <div class="message-text">{{.Text}}</div>
</div>
{{end}}
```

**Form Actions:**

```html
<form lvt-submit="join">
    <input type="text" name="username" required autofocus>
    <button type="submit">Join Chat</button>
</form>

<form lvt-submit="send">
    <input type="text" name="message" autocomplete="off">
    <button type="submit">Send</button>
</form>
```

**Auto-scroll Script:**

```html
<script>
    {{if .CurrentUser}}
    function scrollToBottom() {
        const messages = document.getElementById('messages');
        if (messages) {
            messages.scrollTop = messages.scrollHeight;
        }
    }

    scrollToBottom();

    if (window.LiveTemplate) {
        const originalUpdate = window.LiveTemplate.prototype.updateDOM;
        window.LiveTemplate.prototype.updateDOM = function(...args) {
            originalUpdate.apply(this, args);
            setTimeout(scrollToBottom, 50);
        };
    }
    {{end}}
</script>
```

### Step 6: Run and Test

```bash
go run main.go
```

Open <http://localhost:8090> in multiple browser tabs:

**Test 1 - Same browser, multiple tabs:**

- Open 2+ tabs in Chrome
- Login with any username in tab 1
- Send a message in tab 1
- **It appears instantly in tab 2!** ✨
- Try sending from tab 2 - appears in tab 1

**Test 2 - Different browsers (isolated sessions):**

- Open Chrome and Firefox
- Each browser gets its own chat room
- Messages in Chrome don't appear in Firefox
- Each browser maintains separate state

## How It Works

### Automatic Session Syncing

```text
Chrome Tab 1       Server (Go)        Chrome Tab 2
    |                   |                     |
    |---- join -------->|                     |
    |          [groupID: session-abc]         |
    |                   |<------ join --------|
    |          [Same groupID: session-abc]    |
    |                   |                     |
    |--- send msg ----->|                     |
    |         [Auto-broadcast to group]       |
    |<---- update ------|------- update ----->|
    |                   |                     |
```

**The magic:**

1. Each browser gets a unique session ID (stored in cookie)
2. All tabs in the same browser share the session ID
3. State changes automatically sync to all tabs in the same session
4. Only changed HTML is sent (tree-diffing)
5. Zero manual broadcasting code required!

### Why So Simple?

**Traditional approach (what you DON'T need):**

- ❌ Manual WebSocket management
- ❌ Database setup
- ❌ ORM configuration
- ❌ Complex directory structure
- ❌ Separate frontend/backend
- ❌ API endpoints
- ❌ State sync logic

**LiveTemplate simple kit:**

- ✅ Just modify Go structs
- ✅ 2 files total
- ✅ Auto-broadcasting
- ✅ Auto-updates
- ✅ Standard `html/template`
- ✅ Standard `net/http`

## Customization Ideas

### Add Persistence

Store messages in a slice that survives restarts:

```go
var persistedMessages []Message

func (s *ChatState) Init() error {
    s.Messages = persistedMessages  // Load from memory
    // Or load from file: loadFromJSON("messages.json")
    return nil
}

func (s *ChatState) Change(ctx *livetemplate.ActionContext) error {
    // ... after adding message
    persistedMessages = s.Messages  // Save to memory
    // Or save to file: saveToJSON("messages.json", s.Messages)
}
```

### Add Typing Indicators

```go
type ChatState struct {
    // ... existing fields
    TypingUsers map[string]bool
}

// In Change()
case "typing":
    var data struct {
        Username string `json:"username"`
    }
    ctx.Bind(&data)
    s.TypingUsers[data.Username] = true
    // Auto-broadcast!
```

### Add Message Reactions

```go
type Message struct {
    // ... existing fields
    Reactions map[string]int  // emoji -> count
}

case "react":
    var data struct {
        MessageID int    `json:"messageId"`
        Emoji     string `json:"emoji"`
    }
    ctx.Bind(&data)
    s.Messages[data.MessageID].Reactions[data.Emoji]++
```

### Add Chat Rooms

```go
type ChatState struct {
    Rooms       map[string]*Room
    CurrentRoom string
}

type Room struct {
    Name     string
    Messages []Message
}
```

## Production Considerations

### 1. Use CDN for Client Library

In `chat.tmpl`:

```html
<script src="https://unpkg.com/@livefir/livetemplate-client@latest/dist/livetemplate-client.browser.js"></script>
```

### 2. Add Rate Limiting

```go
case "send":
    if time.Since(s.LastMessageTime) < time.Second {
        return nil  // Too fast, ignore
    }
    // ... process message
```

### 3. Add Message Limits

```go
if len(s.Messages) > 100 {
    s.Messages = s.Messages[len(s.Messages)-100:]  // Keep last 100
}
```

### 4. Add Authentication

For production, use real auth instead of just username:

```go
auth := livetemplate.NewBasicAuthenticator(func(username, password string) (bool, error) {
    return validateUser(username, password)
})

tmpl := livetemplate.New("chat",
    livetemplate.WithDevMode(false),
    livetemplate.WithAuthenticator(auth),
)
```

### 5. Create a Global Chat Room (Cross-Browser)

By default, each browser has its own isolated chat. To make all users share the same chat room:

```go
// Custom authenticator that puts everyone in same session group
type GlobalChatAuthenticator struct{}

func (a *GlobalChatAuthenticator) Identify(r *http.Request) (string, error) {
    return "", nil // Anonymous
}

func (a *GlobalChatAuthenticator) GetSessionGroup(r *http.Request, userID string) (string, error) {
    return "global-chat-room", nil // Everyone shares same group!
}

// Use it:
tmpl := livetemplate.New("chat",
    livetemplate.WithDevMode(true),
    livetemplate.WithAuthenticator(&GlobalChatAuthenticator{}),
)
```

Now Chrome, Firefox, Safari all see the same messages!

## Key Takeaways

1. **Two files** - That's it! `main.go` + `chat.tmpl`
2. **Zero boilerplate** - No cmd/, internal/, database/
3. **Auto-syncing** - Tabs stay in sync automatically
4. **Standard Go** - Uses `net/http` and `html/template`
5. **Type-safe** - Go structs, no JSON marshaling needed
6. **Efficient** - Tree-diffing sends only changes

## Comparison with Counter Example

The simple kit starts with a counter. Here's how we evolved it:

| Counter Example | Chat Example |
|-----------------|--------------|
| `AppState{Counter int}` | `ChatState{Messages []Message}` |
| `increment/decrement` actions | `join/send` actions |
| Single user | Multi-user with broadcasting |
| Simple int update | List of messages |

Same pattern, different data!

## Next Steps

- Try the `counter` example for a simpler starting point
- Try the `todos` example for CRUD operations
- Use `lvt new myapp --kit multi` for apps needing databases

## Related Documentation

- [LiveTemplate Core Docs](../../README.md)
- [Broadcasting Guide](../../docs/design/IMPLEMENTATION_STATUS.md)
- [Template Syntax](https://pkg.go.dev/html/template)
- [LiveTemplate API](https://pkg.go.dev/github.com/livefir/livetemplate)
