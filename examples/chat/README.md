# LiveTemplate Chat Example

A real-time multi-user chat application demonstrating LiveTemplate's broadcasting capabilities and multi-session isolation.

## Features

- **Real-time messaging** - Messages broadcast instantly to all connected users
- **Multi-user support** - Multiple users can chat simultaneously
- **Session isolation** - Each browser tab shares the same user session
- **Auto-scroll** - Chat automatically scrolls to show new messages
- **User presence** - See how many users are online
- **Message history** - All messages persisted during the session

## What This Demonstrates

### 1. **Broadcasting** (NEW)
```go
// Messages are automatically broadcast to all users
// The handler manages this via ConnectionRegistry
handler := tmpl.Handle(initialState)
```

When a user sends a message, the state change triggers an update that's automatically sent to all connected WebSocket clients.

### 2. **Session Groups**
- Each browser (identified by cookie) shares state across its tabs
- Multiple browsers get independent chat sessions
- This demonstrates anonymous user session grouping

### 3. **Real-time Updates**
- WebSocket connection keeps UI in sync
- Tree-based updates minimize bandwidth
- No page reloads needed

## Running the Example

```bash
# From the repository root
go run examples/chat/main.go
```

Then open multiple browser windows at `http://localhost:8090`

## Testing Multi-User Chat

1. **Same browser, multiple tabs:**
   - Open 2+ tabs in Chrome
   - Login with the same username
   - Send a message - it appears in all tabs
   - This demonstrates multi-tab state sharing

2. **Different browsers:**
   - Open Chrome and Firefox
   - Login with different usernames
   - Send messages from each
   - Both users see all messages in real-time
   - This demonstrates multi-user broadcasting

3. **Incognito/Private mode:**
   - Use incognito to simulate a different user
   - Each incognito session is isolated

## Architecture

```
User A (Chrome)          Server                User B (Firefox)
     |                      |                        |
     |------- join -------->|                        |
     |                  [Register]                   |
     |                      |<-------- join ---------|
     |                  [Register]                   |
     |                      |                        |
     |---- send message --->|                        |
     |                  [Broadcast]                  |
     |<----- update --------|-------- update ------->|
     |                      |                        |
```

### State Flow

1. **User joins:** `join` action creates/updates user in `ChatState.Users`
2. **Message sent:** `send` action appends to `ChatState.Messages`
3. **Auto-broadcast:** Handler automatically sends updates to all connections
4. **UI updates:** Each client receives tree-based update and re-renders

## Code Highlights

### Broadcasting (Automatic)

```go
// In Change() method - just update state
s.Messages = append(s.Messages, msg)
return nil

// The handler automatically broadcasts to all connections!
// No manual broadcasting code needed
```

### Session Management

```go
// Each browser gets its own session group (via cookie)
// Multiple tabs in same browser share the same ChatState
handler := tmpl.Handle(initialState)
```

### User Tracking

```go
type ChatState struct {
    Messages      []Message
    Users         map[string]*User  // Track all users
    CurrentUser   string            // Current logged-in user
    OnlineCount   int              // Live user count
    mu            sync.RWMutex     // Thread-safe access
}
```

## Extending This Example

### Add User Authentication

Replace anonymous auth with BasicAuthenticator:

```go
auth := livetemplate.NewBasicAuthenticator(func(username, password string) (bool, error) {
    return validateUser(username, password) // Your validation logic
})

tmpl := livetemplate.New("chat",
    livetemplate.WithAuthenticator(auth),
)
```

### Add Chat Rooms

Use BroadcastToGroup for room-specific messages:

```go
// In your message handler
handler.BroadcastToGroup(roomID, state)
```

### Add Direct Messages

Use BroadcastToUsers for private messages:

```go
// Send to specific user
handler.BroadcastToUsers([]string{recipientID}, privateMessage)
```

### Persist Messages

Add database storage:

```go
func (s *ChatState) Init() error {
    // Load messages from database
    s.Messages = loadMessagesFromDB()
    return nil
}

func (s *ChatState) Change(ctx *livetemplate.ActionContext) error {
    // ... create message
    saveMessageToDB(msg)
    // ... continue
}
```

## Performance Notes

- **Efficient updates:** Only changed HTML is sent (tree diffing)
- **Concurrent safe:** RWMutex protects shared state
- **Scalable:** Each connection has independent template for tree diffing

## Related Examples

- `examples/counter` - Basic reactive state
- `examples/todos` - CRUD operations
- `examples/admin` - Admin dashboard with system broadcasts (see next example)
