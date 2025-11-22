# Critical Bug Investigation: Delete Handler Blocking Under Load

**Date:** 2025-01-22
**Last Updated:** 2025-11-22
**Severity:** CRITICAL
**Status:** ~~Root cause identified~~ **REVISED - New findings from timing instrumentation**
**Affected Version:** v0.4.1, v0.4.2-debug.1

## 🎯 BREAKTHROUGH FINDINGS (2025-11-22)

**THE ORIGINAL HYPOTHESIS WAS WRONG!**

Timing instrumentation in v0.4.2-debug.1 reveals:
- ✅ Delete handler completes in **1.5ms** (NOT 14 seconds!)
- ✅ store.Change() executes SQL DELETE in **778µs**
- ✅ connection.Send() returns `nil` (no error, message queued)
- ❌ **But WebSocket message NEVER reaches browser!**

### Actual Timeline (Full Test Suite):
```
20:48:41.659 - Delete action received
20:48:41.660 - store.Change() completed (778µs) - SQL DELETE executed
20:48:41.661 - Total delete processing: 1.518ms
20:48:41.661 - WebSocket Send: 4.25µs (err: <nil>) - Message queued
[... 20 second gap ...]
20:49:01.620 - Next test's action received
Test fails with 20s timeout waiting for UI update
```

**The Real Problem:** This is NOT server-side blocking - it's **WebSocket message delivery failure**. The message is queued but never transmitted to the browser.

## Executive Summary (REVISED)

~~Real-time SQLite database queries during e2e test execution prove that delete operations **block for 14+ seconds** before executing the SQL DELETE statement.~~

**CORRECTION:** Timing instrumentation proves the delete handler executes in ~1.5ms. The issue is that WebSocket messages are queued successfully but never delivered to the browser in the full test suite, while working correctly in isolated tests.

The v0.4.1 async WebSocket fix (PR #56) successfully made `connection.Send()` non-blocking. The current issue is that messages are queued but the writePump goroutine or WebSocket connection itself fails to deliver them under accumulated test load.

## Evidence

### Database Timeline (Smoking Gun)

Real-time SQLite queries during `TestTutorialE2E` full suite run:

```
Time        Post Count  Status
--------    ----------  ------
20:02:24    1          Delete button clicked in browser
20:02:26    1          Post still in database
20:02:28    1          Post still in database
20:02:30    1          Post still in database
20:02:32    1          Post still in database
20:02:34    1          Post still in database
20:02:36    1          Post still in database (14 seconds elapsed)
20:02:38    -          Database closed by test cleanup
20:02:44    -          Test times out (20 second timeout)
```

**Key Finding:** The post remained in the database for at least 14 seconds after the delete button was clicked, proving the delete handler is blocked before it can execute `DELETE FROM posts WHERE id = ?`.

### Test Results Comparison

**Single Test (PASSES):**
```bash
$ go test -run "TestTutorialE2E/Delete_Post_with_Accepted_Confirmation"
--- PASS: TestTutorialE2E/Delete_Post_with_Accepted_Confirmation (2.41s)
```

**Full Suite (FAILS):**
```bash
$ go test -run "TestTutorialE2E"
--- FAIL: TestTutorialE2E/Delete_Post_with_Accepted_Confirmation (20.06s)
    tutorial_test.go:676: ⚠️ Delete wait timed out
```

The delete test is the 5th of 8 subtests. After 4 previous subtests execute successfully, the delete operation blocks.

### Screenshot Evidence

**Before Delete (Modal Open):**
- File: `/tmp/delete_before_1763837930.png`
- Shows: Edit modal with "My First Blog Post" ready for deletion

**After Delete Click (500ms later):**
- File: `/tmp/delete_after_click_1763837930.png`
- Single test: Shows "No posts yet. Add one above!" ✅
- Full suite: Shows post STILL IN TABLE ❌

## Technical Analysis

### Delete Handler Flow

Expected execution path:
```
1. Browser sends delete action via WebSocket
2. Server receives action in handleAction()
3. state.Change() called with action="delete"
4. Parse and validate DeleteInput
5. ⚠️ Execute SQL: s.Queries.DeletePost(dbCtx, input.ID)  ← BLOCKS HERE
6. Call state.loadUsers() to reload data
7. Render template to generate tree update
8. Call connection.Send() with rendered bytes
9. Browser receives update and removes post from DOM
```

**The blocking occurs at step 5 or between steps 5-7.**

### Why connection.Send() Is NOT The Problem

From `internal/session/registry.go:82-100`:

```go
// Try to queue message for async delivery
select {
case c.sendChan <- &wsMessage{messageType, data}:
    return nil // Message queued successfully
case <-c.done:
    return ErrConnectionClosed
default:
    // Buffer full - client is too slow, close connection
    go c.Close()
    return ErrClientTooSlow  // ← Returns immediately, non-blocking!
}
```

The `Send()` method has a `default` case that returns `ErrClientTooSlow` immediately when the buffer is full. It **does not block**.

### Possible Blocking Locations

Since the database query proves the delete handler is blocked before executing the SQL statement, the blocking must occur in one of these locations:

#### 1. **WebSocket Message Reading (Most Likely)**

The `handleAction()` function in `mount.go` might be blocked waiting to read from the WebSocket, if:
- The read operation is blocking
- Multiple requests are being serialized through a single goroutine
- There's a mutex protecting the WebSocket read operation

**Code to investigate:** `mount.go` around line 450-520

#### 2. **Session Lock/Mutex**

There might be a mutex protecting session state that's held too long:
- Lock acquired during initial page load
- Not released until WebSocket send completes
- Subsequent actions block waiting for lock

**Code to investigate:**
- `internal/session/registry.go`
- Any `sync.Mutex` or `sync.RWMutex` usage

#### 3. **Template Rendering Lock**

If template rendering is protected by a mutex and rendering blocks on WebSocket send (old code path):
- First action acquires render lock
- Tries to send to full WebSocket buffer (blocks in old code)
- Second action waits for render lock
- Deadlock or long delay

**Code to investigate:**
- Template execution code in `mount.go`
- Any locks around `template.Execute()`

#### 4. **Goroutine Pool Exhaustion**

If handlers execute in a limited goroutine pool:
- All goroutines blocked on slow WebSocket sends
- New requests wait for available goroutine
- Unlikely given Go's goroutine model, but possible with custom pooling

## Test Design Context

The `TestTutorialE2E` test creates:
- **ONE server** running on port 8800
- **ONE browser context** shared across all 8 subtests
- **ONE WebSocket connection** from browser to server

By the time the delete test runs (5th subtest), the WebSocket has processed:
1. Initial connection and page load
2. Add post operation (subtest 3)
3. Modal delete verification with fixture creation (subtest 4)
4. Now attempting actual delete (subtest 5) ← BLOCKS

## Next Steps for Investigation

### ✅ Step 1: Add Timing Instrumentation (COMPLETED)

**Status:** Implemented in v0.4.2-debug.1, results captured in full test suite.

**Findings:**
- Delete handler: 1.518ms total
- store.Change() (SQL DELETE): 778µs
- Template render: 649µs
- WebSocket Send(): 4.25µs (err: <nil>)
- **Message queued successfully but never delivered to browser**

Original plan was:

```go
// In mount.go, around handleAction()
func (h *liveHandler) handleAction(...) {
    start := time.Now()
    defer func() {
        log.Printf("[TIMING] handleAction completed in %v", time.Since(start))
    }()

    log.Printf("[TIMING] Received action: %s at %v", action, time.Now())

    // Before state.Change()
    changeStart := time.Now()
    err := h.state.Change(ctx)
    log.Printf("[TIMING] state.Change() completed in %v", time.Since(changeStart))

    // Before rendering
    renderStart := time.Now()
    // ... render code ...
    log.Printf("[TIMING] Template render completed in %v", time.Since(renderStart))

    // Before Send()
    sendStart := time.Now()
    err = connection.Send(...)
    log.Printf("[TIMING] connection.Send() completed in %v (err: %v)", time.Since(sendStart), err)
}
```

---

## 🔍 REVISED INVESTIGATION (Based on Step 1 Results)

Since timing shows the handler completes in 1.5ms but messages don't reach the browser, the focus shifts to **WebSocket message delivery**:

### Step 1a: Instrument writePump Goroutine (NEXT)

The writePump goroutine dequeues messages from `sendChan` and writes to WebSocket. We need to know:
- Is writePump receiving the message from sendChan?
- How long does `conn.WriteMessage()` take?
- Is WriteMessage blocking?
- Are there errors being swallowed?

Add to `/Users/adnaan/code/livetemplate/livetemplate/internal/session/registry.go`:

```go
func (c *Connection) writePump() {
    log.Printf("[PUMP] writePump started for connection %p", c)
    defer log.Printf("[PUMP] writePump exiting for connection %p", c)

    messageCount := 0

    for {
        select {
        case msg := <-c.sendChan:
            messageCount++
            queueSize := len(c.sendChan)
            log.Printf("[PUMP] Dequeued message #%d (queue: %d/%d)", messageCount, queueSize, cap(c.sendChan))

            writeStart := time.Now()
            err := c.Conn.WriteMessage(msg.messageType, msg.data)
            writeDuration := time.Since(writeStart)

            log.Printf("[PUMP] WriteMessage #%d completed in %v (len: %d bytes, err: %v)",
                messageCount, writeDuration, len(msg.data), err)

            if writeDuration > 100*time.Millisecond {
                log.Printf("[PUMP] ⚠️ SLOW WriteMessage: %v for message #%d", writeDuration, messageCount)
            }

            if err != nil {
                log.Printf("[PUMP] ERROR: WriteMessage failed: %v", err)
                return
            }

        case <-c.done:
            log.Printf("[PUMP] Received done signal, exiting")
            return
        }
    }
}
```

Expected outcomes:
- **If logs show message dequeued but WriteMessage hangs:** TCP send buffer is full
- **If logs show message never dequeued:** sendChan is full (50 messages) or writePump died
- **If logs show WriteMessage returns error:** Connection is broken

### Step 1b: Check WebSocket Connection Health

Add connection health logging when Send() is called:

```go
func (c *Connection) Send(messageType int, data []byte) error {
    // Log connection state
    log.Printf("[SEND] Attempting send (queue: %d/%d, done: %v)",
        len(c.sendChan), cap(c.sendChan),
        select { case <-c.done: true; default: false })

    select {
    case c.sendChan <- &wsMessage{messageType, data}:
        log.Printf("[SEND] ✅ Message queued successfully")
        return nil
    case <-c.done:
        log.Printf("[SEND] ❌ Connection closed")
        return ErrConnectionClosed
    default:
        log.Printf("[SEND] ❌ Buffer full, closing connection")
        go c.Close()
        return ErrClientTooSlow
    }
}
```

### Step 1c: Add Browser-Side WebSocket Logging

Modify test to log WebSocket events:

```javascript
// In browser console during test
window.wsMessageCount = 0;
window.wsMessages = [];
const originalOnMessage = client.ws.onmessage;
client.ws.onmessage = function(event) {
    window.wsMessageCount++;
    const msg = JSON.parse(event.data);
    window.wsMessages.push({
        time: new Date().toISOString(),
        count: window.wsMessageCount,
        action: msg.meta?.action,
        success: msg.meta?.success
    });
    console.log(`[WS-RECV #${window.wsMessageCount}]`, msg.meta?.action, msg.meta?.success);
    originalOnMessage.call(this, event);
};
```

This will show if messages arrive but aren't processed correctly.

---

### Step 2: Check for Mutex/Lock Usage (Lower Priority)

Search for synchronization primitives:

```bash
cd /Users/adnaan/code/livetemplate/livetemplate
grep -r "sync.Mutex\|sync.RWMutex\|sync.Lock" --include="*.go" | grep -v test | grep -v vendor
```

For each mutex found:
- Identify what it protects
- Check if it's held during WebSocket send operations
- Verify unlock happens in all code paths

### Step 3: Analyze WebSocket Read Path

Examine how WebSocket messages are read in `mount.go`:

```bash
cd /Users/adnaan/code/livetemplate/livetemplate
grep -A30 "conn.ReadMessage\|ReadJSON" mount.go
```

Questions to answer:
- Is reading serialized (one message at a time)?
- Is there a goroutine per connection or shared handler?
- Are reads blocking indefinitely?

### Step 4: Check writePump Goroutine Status

Verify the background writePump goroutine isn't stuck:

```go
// In internal/session/registry.go, add to writePump():
func (c *Connection) writePump() {
    log.Printf("[PUMP] writePump started for connection %p", c)
    defer log.Printf("[PUMP] writePump exiting for connection %p", c)

    for {
        select {
        case msg := <-c.sendChan:
            start := time.Now()
            err := c.Conn.WriteMessage(msg.messageType, msg.data)
            duration := time.Since(start)

            if duration > 100*time.Millisecond {
                log.Printf("[PUMP] SLOW WRITE: %v (len: %d bytes)", duration, len(msg.data))
            }

            if err != nil {
                log.Printf("[PUMP] Write error: %v", err)
                return
            }
        case <-c.done:
            return
        }
    }
}
```

### Step 5: Add Goroutine Stack Dump on Timeout

When test times out, capture goroutine stacks to see what's blocked:

```go
// In tutorial_test.go, on timeout:
if err != nil {
    // Dump all goroutine stacks
    buf := make([]byte, 1<<20)
    n := runtime.Stack(buf, true)
    os.WriteFile("/tmp/goroutine_stacks.txt", buf[:n], 0644)
    t.Logf("Goroutine stacks written to /tmp/goroutine_stacks.txt")
    t.Fatalf("Failed to delete post: %v", err)
}
```

### Step 6: Test with Increased WebSocket Buffer

As a diagnostic test (not a fix), increase the buffer size to see if it changes behavior:

```go
// In template.go, change:
wsBufferSize := 50  // Current
// To:
wsBufferSize := 500 // Test value
```

If this fixes the issue, it confirms buffer exhaustion is involved, but doesn't explain why the handler blocks instead of returning `ErrClientTooSlow`.

### Step 7: Review handleAction() Serialization

Check if actions are processed serially or concurrently:

```go
// Search for handleAction and see if there's any channel or mutex
// controlling concurrent access
grep -B10 -A50 "func.*handleAction" mount.go
```

## Hypothesis

Based on the evidence, the most likely cause is:

**WebSocket message reading is serialized, and the handler goroutine blocks waiting to send a response before reading the next message.**

Sequence:
1. Test sends multiple actions rapidly (add, edit, delete)
2. Handler processes first action, generates response
3. Handler tries to send response via WebSocket
4. Old code: `WriteMessage()` blocks waiting for slow client
5. New code (v0.4.1): `Send()` returns `ErrClientTooSlow`
6. **Handler doesn't handle the error properly** and retries or blocks
7. Next action waits for current handler to complete
8. Delete action is queued but handler is stuck

**This would explain why:**
- Individual tests pass (no backlog)
- Full suite fails (accumulated backlog)
- Database doesn't get updated (handler never runs)

## Reproduction Steps

```bash
cd /Users/adnaan/code/livetemplate/lvt

# Single test (passes)
go test -v -run "TestTutorialE2E/Delete_Post_with_Accepted_Confirmation" ./e2e/

# Full suite (fails on delete)
go test -v -run "TestTutorialE2E" ./e2e/
```

## Related Files

- `/tmp/CRITICAL_FINDINGS.md` - Initial analysis
- `/tmp/delete_before_1763837930.png` - Screenshot before delete
- `/tmp/delete_after_click_1763837930.png` - Screenshot after delete (full suite)
- `/tmp/delete_after_click_1763834641.png` - Screenshot after delete (single test)
- `/tmp/query_db_at_intervals.sh` - Script to query database during test

## Impact Assessment

**Severity:** CRITICAL

**User Impact:**
- Delete operations hang for 14+ seconds under moderate load
- Application becomes unresponsive during high activity
- Users may click delete multiple times, causing duplicate operations
- Poor user experience, appears broken

**Production Risk:**
- Any application using livetemplate v0.4.1 with concurrent users
- Especially CRUD operations that generate many WebSocket updates
- Risk increases with: number of concurrent users, update frequency, slow client connections

## Recommendation

1. **Immediate:** Revert to v0.3.x until root cause is fixed (if blocking is worse in v0.4.1)
2. **Short-term:** Implement Step 1-3 investigation to identify exact blocking location
3. **Long-term:** Ensure all action handlers are truly async and don't block on WebSocket I/O

## Open Questions

1. Why doesn't `connection.Send()` returning `ErrClientTooSlow` cause the handler to complete?
2. Is there error handling that retries the send operation?
3. Are actions processed in parallel or serial per connection?
4. What happens to actions sent while a handler is blocked?

## References

- Original issue: async-websocket-sends.md proposal
- Fix PR: #56 (livetemplate/livetemplate)
- v0.4.1 release notes
- Test failure logs: `/tmp/test_realtime.log`, `/tmp/test_db_query.log`
