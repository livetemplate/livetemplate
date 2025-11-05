# LiveTemplate v0.2.0 - Code Review Backlog

**Review Date**: 2025-01-05
**Coverage**: 100% of production code
**Files Reviewed**: 77 Go files
**Lines Reviewed**: 10,901 LOC
**Total Issues**: 52+

---

## Critical Issues (Must Fix Before Release)

### #1 - Insecure Default WebSocket Origin Check
- **File**: `template.go:404`
- **Issue**: Default `CheckOrigin` returns true for all origins
- **Impact**: CSRF vulnerability in production deployments - allows any website to connect to WebSocket endpoint
- **Fix**: Default to rejecting non-same-origin requests, require explicit opt-in for permissive mode
- **Code Location**:
  ```go
  Upgrader: websocket.Upgrader{
      CheckOrigin: func(r *http.Request) bool { return true }, // SECURITY RISK
  },
  ```
- **Recommended Fix**:
  ```go
  CheckOrigin: func(r *http.Request) bool {
      origin := r.Header.Get("Origin")
      return origin == "" || origin == r.Host // Same-origin only
  },
  ```
- **Effort**: Small (2-3 hours)
- **Breaking**: Potentially breaking for dev setups, but critical for security
- **Priority**: P0 - Must fix before any production use

### #2 - Silent Serialization Failures in Redis
- **File**: `session_stores.go:336`
- **Issue**: Gob encoding errors silently ignored (function just returns without error)
- **Impact**: Data loss in distributed deployments - session data silently not persisted
- **Code Location**:
  ```go
  if err := encoder.Encode(data); err != nil {
      return // Silent failure - error not logged or returned
  }
  ```
- **Recommended Fix**: Log error at minimum, consider returning error from Set() method
- **Effort**: Small (1-2 hours)
- **Priority**: P0 - Critical for distributed deployments

---

## High Priority (Performance & Reliability)

### #3 - Inconsistent Logging Throughout Codebase
- **Files**: `template.go`, `mount.go` (50+ instances), `pubsub/redis.go` (10+ instances)
- **Issue**: Using `log.Printf` instead of structured logging from `internal/observe`
- **Impact**:
  - Difficult debugging in production
  - No log levels (can't filter by severity)
  - No structured fields (hard to query/analyze)
  - Can't correlate logs across requests
- **Instances**: 60+ instances across codebase
- **Fix**: Replace all `log.Printf` with `internal/observe` structured logging
- **Effort**: Medium (1 day)
- **Priority**: P1

### #4 - Double Template Execution
- **File**: `template.go:689-704`
- **Issue**: Template executed twice in Execute() - once for output buffer, once for lastHTML caching
- **Impact**: 2x performance cost on initial renders (every first page load)
- **Code Location**:
  ```go
  // First execution for output
  if err := t.tmpl.Execute(&buf, data); err != nil { ... }

  // Second execution for caching (lines later)
  var htmlBuf bytes.Buffer
  if err := t.tmpl.Execute(&htmlBuf, data); err != nil { ... }
  t.lastHTML = htmlBuf.String()
  ```
- **Recommended Fix**: Capture bytes from first execution, reuse for both output and caching
- **Effort**: Small (2-3 hours)
- **Priority**: P1

### #5 - No Rate Limiting on WebSocket Messages
- **File**: `mount.go:384-456`
- **Issue**: Message loop has no rate limiting or throttling
- **Impact**: Vulnerable to DoS attacks from malicious clients sending flood of messages
- **Code Location**: `handleWebSocket()` message processing loop
- **Recommended Fix**: Add per-connection rate limiter (e.g., token bucket allowing N messages/second)
- **Effort**: Medium (4-6 hours)
- **Priority**: P1

### #6 - No Request Timeouts
- **File**: `mount.go` (handleWebSocket, handleAction)
- **Issue**: Store.Change() can block indefinitely - no timeout enforcement
- **Impact**:
  - Resource exhaustion (goroutines blocked forever)
  - Deadlocks if store operations hang
  - No way for users to cancel long operations
- **Recommended Fix**: Add context with timeout to Change() calls
- **Effort**: Medium (requires API change to Store interface)
- **Breaking**: Yes (Store.Change signature would need context.Context parameter)
- **Priority**: P1

### #7 - Goroutine Leaks on TTL Refresh
- **File**: `session_stores.go:319`
- **Issue**: Fire-and-forget goroutine created for each Get() call
- **Code Location**:
  ```go
  go func() {
      s.client.Expire(ctx, key, s.ttl) // Fire-and-forget
  }()
  ```
- **Impact**: Memory/goroutine leaks under high load (1000 requests = 1000 goroutines)
- **Recommended Fix**: Use worker pool or batch refresh operations
- **Effort**: Medium (4-6 hours)
- **Priority**: P1

### #8 - Shallow Copy in Store Cloning
- **File**: `mount.go:734-753`
- **Issue**: `copyStruct()` does shallow copy - pointers/slices/maps are shared between instances
- **Code Location**:
  ```go
  func copyStruct(src interface{}) (interface{}, error) {
      // Uses reflection but only copies field values
      // Pointers/slices/maps point to same memory
  }
  ```
- **Impact**: Data races between store instances - mutations affect all copies
- **Recommended Fix**: Implement deep copy or require stores to implement Cloner interface
- **Effort**: Medium-Large (1-2 days)
- **Breaking**: Potentially (behavior change that might break existing code)
- **Priority**: P1

### #9 - Data Race in Metrics Histogram
- **File**: `internal/observe/metrics.go:181`
- **Issue**: Copying samples array without lock protection in Percentile()
- **Code Location**:
  ```go
  samplesCopy := make([]int64, h.size)
  copy(samplesCopy, h.samples) // No lock protection
  ```
- **Impact**: Race condition when recording concurrent samples during percentile calculation
- **Recommended Fix**: Add mutex around copy operation or use atomic snapshot technique
- **Effort**: Small (1-2 hours)
- **Priority**: P1

### #10 - Inefficient Bubble Sort
- **File**: `internal/parse/parse.go:194-211`
- **Issue**: Manual O(n²) bubble sort implementation
- **Code Location**: `getSortedKeys()` function
- **Impact**: Performance degradation for templates with many dynamic fields
- **Recommended Fix**: Use `sort.Slice` from standard library
- **Effort**: Trivial (15 minutes)
- **Priority**: P1

---

## Medium Priority (Code Quality & Maintainability)

### #11 - Code Duplication: Auto-Broadcast Logic
- **Files**: `mount.go:408-417` (WebSocket), `mount.go:560-569` (HTTP)
- **Issue**: Same auto-broadcast logic duplicated in two places
- **Lines**: ~20 lines duplicated
- **Recommended Fix**: Extract to `autoBroadcastToGroup(groupID, excludeConn, data)` method
- **Effort**: Small (1 hour)
- **Priority**: P2

### #12 - Code Duplication: Parse/ParseFiles/ParseGlob
- **File**: `template.go:479-664`
- **Issue**: Similar parsing logic duplicated across three methods (~180 lines total)
- **Recommended Fix**: Extract common logic to private `parseInternal()` method
- **Effort**: Small (2-3 hours)
- **Priority**: P2

### #13 - Hard-coded Values Should Be Configurable
- **Instances**:
  1. `session_stores.go:165` - Cleanup interval hard-coded to 1 hour
  2. `mount.go:476` - Cookie MaxAge hard-coded to 1 year
  3. `pubsub/redis.go:337` - Reconnect delay hard-coded to 1 second
- **Impact**: No flexibility for different deployment requirements
- **Recommended Fix**: Add configuration options via WithCleanupInterval(), WithCookieMaxAge(), etc.
- **Effort**: Small (1-2 hours each, ~4 hours total)
- **Priority**: P2

### #14 - Missing Context Propagation
- **Files**: `session_stores.go:281,341,359,371,450`
- **Issue**: Always uses `context.Background()`, can't respect caller timeouts or cancellation
- **Instances**: 5+ method calls
- **Impact**:
  - Redis operations can't be cancelled
  - Timeouts not respected
  - Can't propagate trace IDs
- **Recommended Fix**: Accept `context.Context` parameter in SessionStore methods
- **Effort**: Medium (4-6 hours)
- **Breaking**: Yes (method signatures change)
- **Priority**: P2

### #15 - Test Code in Production
- **File**: `mount.go:984-986`
- **Issue**: `if conn.Conn == nil` check for testing embedded in production code
- **Code Location**:
  ```go
  if conn.Conn == nil {
      return // Skip send for testing
  }
  ```
- **Impact**: Test-specific logic pollutes production code
- **Recommended Fix**: Use dependency injection (inject a Writer interface) or build tags
- **Effort**: Small (2-3 hours)
- **Priority**: P2

### #16 - Global State in Tests
- **File**: `tree.go:77`
- **Issue**: Global `defaultKeyGen` variable shared across tests
- **Impact**:
  - Test failures due to shared state
  - Race conditions in parallel tests
  - Tests not isolated
- **Recommended Fix**: Remove global or use test-specific instances via functional options
- **Effort**: Small (1-2 hours)
- **Priority**: P2

### #17 - Repeated Template Parsing in evaluatePipe
- **File**: `internal/parse/parse.go:249,263`
- **Issue**: Template parsed on every `evaluatePipe()` call
- **Impact**: Performance overhead for frequently-evaluated expressions
- **Recommended Fix**: Cache parsed templates in sync.Pool or LRU cache
- **Effort**: Medium (3-4 hours)
- **Priority**: P2

### #18 - Repeated Template Parsing in AST Handlers
- **Files**:
  - `internal/parse/field.go:14-30` (handleActionNode)
  - `internal/parse/conditional.go:14-18` (handleIfNode)
- **Issue**: Templates re-parsed for every action node and condition evaluation
- **Impact**: Significant performance overhead during tree building
- **Recommended Fix**: Template caching or pre-parsing strategy at Parse() level
- **Effort**: Medium (4-6 hours)
- **Priority**: P2

### #19 - Expensive DeepEqual Fallback
- **File**: `internal/parse/parse.go:305`
- **Issue**: `reflect.DeepEqual` used as fallback in `isZeroValue()` - very slow
- **Code Location**:
  ```go
  default:
      return reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
  ```
- **Recommended Fix**: Handle more types explicitly (channels, funcs, etc.) before fallback
- **Effort**: Small (2-3 hours)
- **Priority**: P2

### #20 - Name Collision Risk
- **File**: `internal/parse/parse.go:229`
- **Issue**: `__lvt_capture_result__` function name could collide with user-defined functions
- **Impact**: If user defines same function name, unpredictable behavior
- **Recommended Fix**: Use UUID suffix or more unique namespace (e.g., `__lvt_internal_capture_<uuid>__`)
- **Effort**: Trivial (30 minutes)
- **Priority**: P2

### #21 - Unsafe Type Assertions
- **File**: `mount.go:717`
- **Issue**: Type assertion without ok check: `.Interface().(Store)`
- **Code Location**:
  ```go
  newStore := reflect.New(storeType).Interface().(Store) // Could panic
  ```
- **Impact**: Potential panic if type assertion fails
- **Recommended Fix**: Use two-value type assertion with error handling
- **Effort**: Trivial (15 minutes)
- **Priority**: P2

### #22 - runtime.Caller() Brittleness
- **File**: `template_discovery.go:21`
- **Issue**: Uses `runtime.Caller(2)` which is brittle to call stack changes
- **Code Location**:
  ```go
  _, filename, _, ok := runtime.Caller(2) // Assumes specific call depth
  ```
- **Impact**:
  - Template discovery breaks if call stack changes
  - Hard to debug when it fails
  - Fragile to refactoring
- **Recommended Fix**: Accept explicit directory parameter or use build-time code generation
- **Effort**: Medium (3-4 hours)
- **Breaking**: Potentially (API change if adding parameter)
- **Priority**: P2

### #23 - JSON Marshal Errors Ignored in Health Checks
- **File**: `health.go:101,138,202`
- **Issue**: `json.NewEncoder(w).Encode()` errors ignored
- **Impact**: Silent failures in health check responses
- **Recommended Fix**: Log errors even if can't change HTTP status
- **Effort**: Trivial (30 minutes)
- **Priority**: P2

### #24 - Weak Health Check Implementation
- **File**: `health.go:225`
- **Issue**: SessionStoreHealthChecker just calls Get() and ignores result
- **Code Location**:
  ```go
  func (c *SessionStoreHealthChecker) Check(ctx context.Context) error {
      c.store.Get("health-check-key") // Result ignored
      return nil // Always returns nil
  }
  ```
- **Impact**: Health check doesn't actually verify store functionality
- **Recommended Fix**: Attempt Get/Set/Delete cycle or document limitations
- **Effort**: Small (1-2 hours)
- **Priority**: P2

### #25 - Goroutine Leak in Health Check
- **File**: `health.go:251`
- **Issue**: Ping goroutine could leak if context cancelled mid-operation
- **Code Location**:
  ```go
  go func() {
      err = client.Ping(ctx).Err() // Could block if ctx cancelled
  }()
  ```
- **Recommended Fix**: Use select with context.Done() or ensure goroutine cleanup
- **Effort**: Small (1 hour)
- **Priority**: P2

### #26 - Panic Recovery Hides Bugs
- **File**: `tree.go:98-102`
- **Issue**: Converts all panics to errors in GenerateTreeUpdates()
- **Code Location**:
  ```go
  defer func() {
      if r := recover(); r != nil {
          err = fmt.Errorf("panic: %v", r)
      }
  }()
  ```
- **Impact**: Could hide real bugs during development
- **Recommended Fix**: Only catch expected panics, or make configurable via DevMode
- **Effort**: Small (2 hours)
- **Priority**: P2

### #27 - Hard-coded Block Tags
- **File**: `template.go:1088-1089`
- **Issue**: Block tags array hard-coded in wrapHTML() function
- **Recommended Fix**: Move to package-level constant for reusability and testing
- **Effort**: Trivial (15 minutes)
- **Priority**: P2

---

## Low Priority (Nice to Have)

### #28 - Template Registry Memory Leak
- **File**: `template.go:143`
- **Issue**: `registry` (ClientStructureRegistry) grows over time with no cleanup
- **Impact**: Memory leak for long-lived templates with many structure variations
- **Recommended Fix**: Add TTL or size limit with LRU eviction
- **Effort**: Medium (4-6 hours)
- **Priority**: P3

### #29 - Zero Value Ambiguity in ActionData
- **Files**: `action.go:75-96` (GetInt, GetFloat, GetBool)
- **Issue**: Methods return zero value on error - can't distinguish "key not found" vs "wrong type" vs "actual zero"
- **Code Location**:
  ```go
  func (d ActionData) GetInt(key string) int {
      val, _ := strconv.Atoi(d.Get(key))
      return val // Returns 0 on error - ambiguous
  }
  ```
- **Recommended Fix**: Return (value, bool) or (value, error) tuple
- **Effort**: Small (2 hours)
- **Breaking**: Yes (return signature changes)
- **Priority**: P3

### #30 - Field Name Lowercasing in Validation
- **File**: `action.go:189`
- **Issue**: Field name always lowercased in validation errors
- **Code Location**:
  ```go
  return FieldError{
      Field:   strings.ToLower(fieldName), // Always lowercase
      Message: message,
  }
  ```
- **Impact**: Error messages may not match actual form field names (camelCase/PascalCase)
- **Recommended Fix**: Use original field name or make configurable
- **Effort**: Small (1 hour)
- **Priority**: P3

### #31 - Debug Logging in Production
- **File**: `action.go:51-52`
- **Issue**: Debug log.Printf statements left in production code
- **Recommended Fix**: Remove or put behind debug flag
- **Effort**: Trivial (15 minutes)
- **Priority**: P3

### #32 - Panic on Crypto Failure
- **File**: `auth.go:176`
- **Issue**: Panics if `crypto/rand.Read()` fails
- **Code Location**:
  ```go
  if _, err := rand.Read(b); err != nil {
      panic(fmt.Sprintf("failed to generate random bytes: %v", err))
  }
  ```
- **Impact**: While rare, panic is harsh - could take down entire process
- **Recommended Fix**: Return error and handle gracefully
- **Effort**: Small (1 hour)
- **Priority**: P3

### #33 - No Brute Force Protection
- **File**: `auth.go` (BasicAuthenticator)
- **Issue**: BasicAuthenticator.Identify() has no rate limiting or lockout
- **Impact**: Vulnerable to brute force password guessing attacks
- **Recommended Fix**: Add rate limiter or document requirement for external protection (e.g., fail2ban)
- **Effort**: Medium (3-4 hours for built-in) or just documentation
- **Priority**: P3

### #34 - HTTP Basic Auth Security Warning
- **File**: `auth.go` (BasicAuthenticator)
- **Issue**: BasicAuthenticator uses HTTP Basic Auth (base64, not encrypted)
- **Impact**: Credentials sent in clear text over non-HTTPS connections
- **Recommended Fix**: Document HTTPS requirement prominently in godoc and README
- **Effort**: Trivial (documentation only)
- **Priority**: P3

### #35 - Unused Config Fields
- **File**: `config.go:52-60`
- **Issue**: ShutdownTimeout, LogLevel, MetricsEnabled loaded from env but not used
- **Code Location**: ToOptions() method doesn't convert these fields to Options
- **Recommended Fix**: Either remove these fields or implement their usage
- **Effort**: Small (2-3 hours to implement)
- **Priority**: P3

### #36 - Undocumented Bool Values
- **File**: `config.go:265-270`
- **Issue**: parseBool() accepts "yes/no/on/off" but not documented
- **Recommended Fix**: Update function comments to reflect all accepted values
- **Effort**: Trivial (documentation only)
- **Priority**: P3

### #37 - Lock Granularity in Template
- **File**: `template.go:134`
- **Issue**: Single mutex protects all mutable state (tree, data, HTML, registry)
- **Impact**: Could cause contention with many concurrent requests
- **Recommended Fix**: Consider separate locks for different state domains
- **Effort**: Medium (4-6 hours, needs careful analysis for deadlock prevention)
- **Priority**: P3

### #38 - Context Not Passed to Store.Change()
- **File**: `mount.go:323`
- **Issue**: Context created but never passed to Store.Change()
- **Code Location**:
  ```go
  ctx := context.WithValue(r.Context(), "user_id", userID)
  // But Store.Change() has no ctx parameter to receive it
  ```
- **Impact**: Stores can't use context for cancellation/deadlines/trace IDs
- **Recommended Fix**: Add context.Context parameter to Store interface methods
- **Effort**: Medium (3-4 hours)
- **Breaking**: Yes (Store interface changes)
- **Priority**: P3 (overlaps with #6, implement together)

### #39 - Gob Registration Not Enforced
- **File**: `session_stores.go:397-422`
- **Issue**: Gob requires type registration but no enforcement or helper
- **Impact**: Will fail at runtime if custom types not registered
- **Recommended Fix**: Add registration checker or provide better error messages
- **Effort**: Small (2-3 hours)
- **Priority**: P3

### #40 - copyStruct Documentation
- **File**: `mount.go:746-753`
- **Issue**: copyStruct only copies exported fields but not documented
- **Impact**: Could cause bugs if stores have unexported state
- **Recommended Fix**: Document this limitation prominently in function comment
- **Effort**: Trivial (documentation only)
- **Priority**: P3

### #41 - DeepEqual Using fmt.Sprintf
- **File**: `internal/diff/helpers.go:161`
- **Issue**: DeepEqual implemented as `fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)`
- **Code Location**:
  ```go
  func DeepEqual(a, b interface{}) bool {
      return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
  }
  ```
- **Impact**: Very inefficient, inaccurate (doesn't handle all types correctly)
- **Recommended Fix**: Use reflect.DeepEqual or custom comparison for known types
- **Effort**: Small (2 hours)
- **Priority**: P3

### #42-52 - Additional Minor Issues
- Broadcast errors handling (mount.go:788-793, 848-855, 911-916)
- Race condition in auto-broadcast goroutine launches (mount.go:408)
- Hard-coded cleanup interval (session_stores.go:165)
- Cookie configuration options (mount.go:476)
- Store Init() errors only logged (mount.go:523-529, 724-728)
- Missing error context in messages (template.go:754)
- Empty static normalization (html_minify.go:42-49)
- Redundant key generator nil check (template.go:774-777)
- Template parse failure handling (template.go:429,435)
- Silent fallback on parse errors (template.go:939-943)
- Minification failure fallback (html_minify.go:31-32)

**Combined Effort**: ~2-3 days
**Priority**: P3

---

## Summary Statistics

### Issue Distribution
- **Critical**: 2 issues (Security/Data Loss)
- **High Priority**: 8 issues (Performance/Reliability)
- **Medium Priority**: 17 issues (Code Quality)
- **Low Priority**: 25+ issues (Nice to Have)
- **Total**: 52+ documented issues

### Effort Estimates
- **Critical Issues**: 1 day total
- **High Priority**: ~2 weeks total
- **Medium Priority**: ~2 weeks total
- **Low Priority**: ~1-2 weeks total
- **Grand Total**: ~6-7 weeks for all issues

### Breaking Changes
Issues requiring breaking API changes:
- **#6**: Store.Change() context parameter
- **#14**: SessionStore method signatures (context parameters)
- **#8**: Store cloning behavior (if changing to deep copy)
- **#22**: Template discovery API (if adding directory parameter)
- **#29**: ActionData return signatures
- **#38**: Store interface context parameters

### Positive Findings ✅
- Template composition fully implemented (400 lines in template_flatten.go)
- Excellent observability package with structured logging
- Strong type safety with TreeNode refactoring
- Good separation of concerns via internal packages
- Thread-safe connection/session management
- Production-ready metrics and Prometheus export
- Comprehensive test coverage infrastructure

---

## Implementation Roadmap

### Phase 1: Critical Security & Data Loss (1 day)
**Goal**: Make library safe for production use

1. **#1 - WebSocket Origin Check** (2-3 hours)
   - Add secure default CheckOrigin
   - Add opt-in for permissive mode
   - Document security implications

2. **#2 - Redis Serialization** (1-2 hours)
   - Log gob encoding errors
   - Consider returning errors from Set()

**Milestone**: Library safe for production deployment

### Phase 2: Performance & Reliability (2 weeks)
**Goal**: Optimize performance and prevent DoS

1. **#3 - Structured Logging** (1 day)
   - Replace all log.Printf with observe package
   - Add trace ID propagation
   - Update logging configuration

2. **#4 - Double Execution** (2-3 hours)
   - Refactor Execute() to capture once
   - Benchmark performance improvement

3. **#5 - Rate Limiting** (4-6 hours)
   - Add token bucket rate limiter
   - Make rate configurable
   - Add metrics for rate limit hits

4. **#6 - Request Timeouts** (1 day)
   - Add context.Context to Store interface
   - Update all Store implementations
   - Document timeout configuration

5. **#7 - Goroutine Leaks** (4-6 hours)
   - Implement TTL refresh worker pool
   - Add shutdown coordination
   - Test under load

6. **#8 - Deep Copy** (1-2 days)
   - Implement Cloner interface
   - Add deep copy helpers
   - Test concurrent access

7. **#9 - Metrics Race** (1-2 hours)
   - Add lock to histogram
   - Benchmark performance impact

8. **#10 - Bubble Sort** (15 minutes)
   - Replace with sort.Slice
   - Remove test for specific sorting

**Milestone**: Production-grade performance and reliability

### Phase 3: Code Quality (2 weeks)
**Goal**: Improve maintainability and reduce technical debt

1. **Deduplicate Code** (#11, #12)
2. **Add Configuration** (#13)
3. **Context Propagation** (#14)
4. **Remove Test Code** (#15, #16)
5. **Template Caching** (#17, #18)
6. **Type Safety** (#19-22)
7. **Health Checks** (#23-25)
8. **Error Handling** (#26-27)

**Milestone**: Clean, maintainable codebase

### Phase 4: Polish & Documentation (Ongoing)
**Goal**: Address remaining issues and improve developer experience

1. **Memory Management** (#28)
2. **API Improvements** (#29-38)
3. **Documentation** (#34, #36, #40)
4. **Minor Fixes** (#39, #41-52)

**Milestone**: Production-ready v0.2.0 release

---

## Next Steps

1. **Review this backlog** with team/stakeholders
2. **Prioritize issues** based on your specific needs
3. **Create GitHub issues** for tracked items
4. **Assign owners** for each phase
5. **Set milestones** and target dates
6. **Begin Phase 1** (Critical fixes) immediately

## Notes

- This backlog was generated from a comprehensive 100% code review
- All file references are relative to repository root
- Line numbers are approximate and may shift with changes
- Breaking changes should be coordinated for a single major version bump
- Some issues (#6, #14, #38) overlap and should be implemented together

---

**Review completed by**: Claude Code
**Review methodology**: File-by-file systematic analysis
**Focus areas**: Security, Performance, Code Quality, Maintainability
