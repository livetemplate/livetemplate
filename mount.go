package livetemplate

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	lvtcontext "github.com/livetemplate/livetemplate/internal/context"
	"github.com/livetemplate/livetemplate/internal/observe"
	"github.com/livetemplate/livetemplate/internal/send"
	"github.com/livetemplate/livetemplate/internal/session"
	"github.com/livetemplate/livetemplate/internal/upload"
	"github.com/livetemplate/livetemplate/internal/uploadtypes"
	"github.com/livetemplate/livetemplate/pubsub"
	"golang.org/x/time/rate"
)

// Session allows controllers to trigger server-initiated actions for
// connected clients. Actions triggered via Session affect every connection
// in the same session group (all tabs sharing one browser session, plus
// any additional devices that the configured Authenticator places in the
// same group).
//
// This is the recommended way to implement:
//   - Timers and ticks
//   - Background job completion notifications
//   - Webhook-triggered updates
//   - Cross-tab synchronization
//
// Scope: Session is scoped to a session group (groupID), not to a user
// identity (userID). For the typical anonymous flow where each browser
// session maps to one group via cookie, this is equivalent to "all tabs
// of this browser". For authenticated flows the mapping depends on how
// the Authenticator assigns groupIDs — a user with multiple devices may
// share a group across devices (by returning a stable groupID keyed on
// userID) or may have per-device groups (by returning a per-session
// groupID). Session.TriggerAction always targets the group of the
// Context it was obtained from, never other groups.
type Session interface {
	// TriggerAction dispatches the action to the matching controller
	// method on every connection in the session group, then sends the
	// updated template to each of those connections.
	//
	// This behaves identically to client-initiated actions: the action
	// runs through the controller's action method, errors are captured,
	// and diffs are sent over WebSocket to each connection.
	//
	// Example:
	//   session.TriggerAction("tick", nil)
	//   session.TriggerAction("new_notification", map[string]interface{}{"id": 123})
	TriggerAction(action string, data map[string]interface{}) error
}

// LiveHandler is the interface returned by Template.Handle()
// It provides HTTP handling and lifecycle management for live template connections.
//
// For server-initiated actions, implement an OnConnect(state, ctx) lifecycle
// method on your controller and call ctx.Session() to obtain a Session handle
// that can be used to trigger actions from background goroutines. See the
// Session interface above and docs/references/server-actions.md for details.
type LiveHandler interface {
	http.Handler

	// Shutdown gracefully shuts down the handler, draining connections.
	//
	// It performs the following steps:
	//  1. Stops accepting new WebSocket connections
	//  2. Sends close frames to all active WebSocket connections
	//  3. Waits for in-flight requests to complete (respecting context timeout)
	//
	// The context timeout controls how long to wait for connections to close.
	// After the timeout, remaining connections are forcefully closed.
	//
	// Example usage with http.Server:
	//   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	//   defer cancel()
	//   handler.Shutdown(ctx)
	//   server.Shutdown(ctx)
	Shutdown(ctx context.Context) error

	// MetricsHandler returns an http.Handler that exports Prometheus metrics.
	//
	// The handler responds to GET requests with metrics in Prometheus text format.
	// Typically mounted at /metrics for scraping by Prometheus.
	//
	// Example with standard library http mux:
	//   mux := http.NewServeMux()
	//   handler := template.Handle(store)
	//   mux.Handle("/live", handler)
	//   mux.Handle("/metrics", handler.MetricsHandler())
	//   http.ListenAndServe(":8080", mux)
	//
	// Example with gorilla/mux:
	//   r := mux.NewRouter()
	//   handler := template.Handle(store)
	//   r.Handle("/live", handler)
	//   r.Handle("/metrics", handler.MetricsHandler())
	MetricsHandler() http.Handler

	// Publish fans a topic action out to every subscribed connection (local +
	// cross-instance), the same primitive as ctx.Publish but with no Context —
	// for out-of-band callers (webhook, cron, admin path). Use case F/G/H.
	//
	//   handler.Publish(livetemplate.UserTopic("alice"), "DM", data)
	//   handler.Publish("announcements", "Maintenance", map[string]any{"at": t})
	//
	// Each receiver runs action against its OWN state via the shared dispatch
	// path (the reconciler model). There is no sender to exclude (no Context),
	// so every subscriber — including all of the addressed identity's
	// connections — receives it.
	//
	// Send is ungated (proposal §3): Publish runs NO ACL — the Subscribe-time
	// ACL gates who reads. Reserved lvt: topics are permitted on the send side
	// without a SelfTopic()-equality check (anti-spoof is a Subscribe-side
	// rule). Developer (non-lvt:) topics must satisfy the segment grammar.
	// Scope out-of-band Publish call sites the way you would scope a
	// "send to these users" capability — keep them in trusted code.
	//
	// Returns an error only for an invalid topic/action argument (empty, or a
	// malformed developer topic); a transport failure to a remote instance is
	// logged, not returned (local fan-out still succeeds).
	Publish(topic, action string, data map[string]interface{}) error
}

// ephemeralSweepTTL is how long idle HTTP template cache entries survive in ephemeral
// mode before being evicted by the sweep loop. 30 minutes balances memory reclamation
// for abandoned sessions vs keeping diff baselines alive for active users between
// interactions (e.g., reading a page before submitting a form).
const ephemeralSweepTTL = 30 * time.Minute

// mountConfig configures the mount handler (internal only)
type mountConfig struct {
	Template               *Template
	Controller             interface{} // Singleton controller with dependencies
	State                  State       // Initial state template (cloned per session)
	Upgrader               WSUpgrader
	SessionStore           SessionStore
	Authenticator          Authenticator
	PubSubBroadcaster      pubsub.Broadcaster // Optional: for distributed broadcasting across instances
	AllowedOrigins         []string
	WebSocketDisabled      bool
	MaxConnections         int64                               // Maximum total connections (0 = unlimited)
	MaxConnectionsPerGroup int64                               // Maximum connections per group (0 = unlimited)
	CookieMaxAge           time.Duration                       // Session cookie max age (default: 1 year)
	UploadConfigs          map[string]uploadtypes.UploadConfig // Upload field configurations
	wsBufferSize           int                                 // WebSocket send buffer size per connection (default: 50)
	ProgressiveEnhancement bool                                // Enable non-JS form submission support with PRG pattern (default: true)
	Capabilities           []string                            // Controller capabilities detected at setup (e.g., ["change"])
	TopicACL               TopicACLFunc                        // Topic-subscription ACL hook (nil unless WithTopicACL); deny-all default when nil and !OpenTopics
	OpenTopics             bool                                // WithOpenTopics(): permit every topic Subscribe (mutually exclusive with TopicACL)
}

// liveHandler handles both WebSocket and HTTP requests
type liveHandler struct {
	config          mountConfig
	persistable     persistableState // non-nil if state has lvt:"persist" fields
	registry        *session.ConnectionRegistry
	limits          *session.ConnectionLimits
	metricsExporter *observe.PrometheusExporter
	tempFileManager uploadTempFileManager
	httpTemplates   sync.Map // groupID → *httpTemplateCacheEntry (cached for HTTP POST diff optimization)
	httpLastPaths   sync.Map // groupID → string (last served request path, for detecting URL changes)

	// Graceful shutdown state
	shutdownOnce sync.Once
	shutdownChan chan struct{}
	shutdownWg   sync.WaitGroup
	isShutdown   atomic.Bool
}

// httpTemplateCacheEntry wraps a cached Template with a mutex.
// Concurrent HTTP requests for the same groupID (e.g., multiple tabs)
// must serialize template operations to avoid data races on lastTree/lastData.
type httpTemplateCacheEntry struct {
	mu           sync.Mutex
	tmpl         *Template
	lastAccessed atomic.Int64 // unix timestamp, for time-based eviction in ephemeral mode
}

// wsReadMessage carries data from the readPump goroutine to the event loop.
type wsReadMessage struct {
	data []byte
	err  error
}

// flashKey is a bare flash key (no "_flash:" prefix) — distinct from prefixed keys in connState.messages.
type flashKey string

type connState struct {
	state       interface{}            // Typed state (cloned per session)
	messages    map[string]string      // keys: field errors (plain) + flash (prefixed with "_flash:"); note: flashExpiry below uses bare keys (typed as flashKey)
	flashExpiry map[flashKey]time.Time // keys: bare flash key WITHOUT "_flash:" prefix (typed flashKey, see comment above)
	messagesMu  sync.RWMutex           // Mutex for thread-safe message access
	groupID     string                 // Session/group ID for this connection
}

func (c *connState) setError(field, message string) {
	c.messagesMu.Lock()
	defer c.messagesMu.Unlock()
	c.messages[field] = message
}

func (c *connState) clearErrors() {
	c.messagesMu.Lock()
	defer c.messagesMu.Unlock()
	// Clear field errors but preserve flash messages for the upcoming render.
	// Flash messages are cleared separately after the response is sent.
	newMessages := make(map[string]string)
	for k, v := range c.messages {
		if strings.HasPrefix(k, lvtcontext.FlashPrefix) {
			newMessages[k] = v
		}
	}
	c.messages = newMessages
}

func (c *connState) getMessages() map[string]string {
	// Prune before snapshot so renderers never observe expired entries.
	c.pruneExpiredFlash()

	c.messagesMu.RLock()
	defer c.messagesMu.RUnlock()

	// Return copy to avoid race conditions
	result := make(map[string]string, len(c.messages))
	for k, v := range c.messages {
		result[k] = v
	}
	return result
}

func (c *connState) setFlash(key, message string, expiry time.Duration) {
	// Validate key: reject keys with ":" or starting with "_"
	if strings.Contains(key, ":") || strings.HasPrefix(key, "_") {
		slog.Warn("Invalid flash key ignored",
			slog.String("component", "live_handler"),
			slog.String("key", key),
			slog.String("reason", "keys must not contain ':' or start with '_'"))
		return
	}

	c.messagesMu.Lock()
	defer c.messagesMu.Unlock()
	c.messages[lvtcontext.FlashPrefix+key] = message
	if expiry > 0 {
		if c.flashExpiry == nil {
			c.flashExpiry = make(map[flashKey]time.Time)
		}
		c.flashExpiry[flashKey(key)] = time.Now().Add(expiry)
	} else {
		// No expiry — persist until ClearFlash. Remove any prior expiry.
		// delete on a nil map is a safe no-op in Go, so no nil guard needed.
		delete(c.flashExpiry, flashKey(key))
	}
}

// clearFlashKey removes a single flash message by key. Called by
// Context.ClearFlash — the explicit clearing path for flash messages
// that persist until acknowledged.
func (c *connState) clearFlashKey(key string) {
	c.messagesMu.Lock()
	defer c.messagesMu.Unlock()
	delete(c.messages, lvtcontext.FlashPrefix+key)
	// delete on a nil map is a safe no-op in Go, so no nil guard needed here.
	delete(c.flashExpiry, flashKey(key))
}

// clearAllFlash removes every flash message on this connection while
// preserving field-validation errors. Called by Context.ClearAllFlash.
func (c *connState) clearAllFlash() {
	c.messagesMu.Lock()
	defer c.messagesMu.Unlock()
	for k := range c.messages {
		if strings.HasPrefix(k, lvtcontext.FlashPrefix) {
			delete(c.messages, k)
		}
	}
	// Drop the expiry map outright — matches the lazy-init pattern in setFlash.
	c.flashExpiry = nil
}

// pruneExpiredFlash removes only flash messages whose expiry has
// passed. Flash messages without an expiry (expiry == zero time) are
// NOT removed — they persist until explicitly cleared via ClearFlash.
//
// This replaces the old clearFlash() which removed ALL flash messages
// after each render. The new model matches Phoenix LiveView: flash
// is a separate namespace that survives renders and background updates
// until the developer (or an expiry timer) explicitly clears it.
func (c *connState) pruneExpiredFlash() {
	// Fast path: skip write lock when no expiry entries exist (common case
	// for controllers that don't use FlashExpiry). The read lock avoids
	// unnecessary write-lock contention on every render site.
	c.messagesMu.RLock()
	empty := len(c.flashExpiry) == 0
	c.messagesMu.RUnlock()
	if empty {
		return
	}
	c.messagesMu.Lock()
	defer c.messagesMu.Unlock()
	// Re-check under the write lock: another goroutine may have emptied
	// flashExpiry between the read-unlock and the write-lock above. A goroutine
	// that added entries is also fine to miss — newly-set flash can't be expired
	// yet, so skipping one prune cycle for new entries is safe.
	if len(c.flashExpiry) == 0 {
		return
	}
	now := time.Now()
	for key, exp := range c.flashExpiry {
		if now.After(exp) {
			delete(c.messages, lvtcontext.FlashPrefix+string(key))
			delete(c.flashExpiry, key)
		}
	}
}

// getFlashValues returns all flash messages as url.Values for cookie encoding.
func (c *connState) getFlashValues() url.Values {
	c.messagesMu.RLock()
	defer c.messagesMu.RUnlock()
	vals := url.Values{}
	for k, v := range c.messages {
		if strings.HasPrefix(k, lvtcontext.FlashPrefix) {
			vals.Set(strings.TrimPrefix(k, lvtcontext.FlashPrefix), v)
		}
	}
	return vals
}

// hasErrors returns true if there are any field errors (non-flash messages)
func (c *connState) hasErrors() bool {
	c.messagesMu.RLock()
	defer c.messagesMu.RUnlock()
	for k := range c.messages {
		if !strings.HasPrefix(k, lvtcontext.FlashPrefix) {
			return true
		}
	}
	return false
}

// getErrorsOnly returns only field errors (excludes flash messages)
func (c *connState) getErrorsOnly() map[string]string {
	c.messagesMu.RLock()
	defer c.messagesMu.RUnlock()

	result := make(map[string]string)
	for k, v := range c.messages {
		if !strings.HasPrefix(k, lvtcontext.FlashPrefix) {
			result[k] = v
		}
	}
	return result
}

func (h *liveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add header to indicate WebSocket availability
	if h.config.WebSocketDisabled {
		w.Header().Set("X-LiveTemplate-WebSocket", "disabled")
	} else {
		w.Header().Set("X-LiveTemplate-WebSocket", "enabled")
	}

	if WSIsUpgrade(r) {
		if h.config.WebSocketDisabled {
			http.Error(w, "WebSocket is disabled on this endpoint", http.StatusBadRequest)
			return
		}
		h.handleWebSocket(w, r)
	} else {
		h.handleHTTP(w, r)
	}
}

func (h *liveHandler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check if shutting down - reject new connections
	if h.isShutdown.Load() {
		http.Error(w, "Service is shutting down", http.StatusServiceUnavailable)
		return
	}

	// Track this connection goroutine for graceful shutdown
	h.shutdownWg.Add(1)
	defer h.shutdownWg.Done()

	// Authenticate user and get session group
	userID, err := h.config.Authenticator.Identify(r)
	if err != nil {
		slog.Error("Authentication failed",
			slog.String("component", "live_handler"),
			slog.Any("error", err))
		h.writeUnauthorized(w)
		return
	}

	groupID, err := h.config.Authenticator.GetSessionGroup(r, userID)
	if err != nil {
		slog.Error("Failed to get session group",
			slog.String("component", "live_handler"),
			slog.Any("error", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check connection limits before upgrading
	if !h.limits.CanAccept(groupID) {
		stats := h.limits.Stats()
		slog.Warn("Connection rejected (at capacity)",
			slog.String("component", "live_handler"),
			slog.Int64("active", stats.ActiveConnections),
			slog.Int64("max", stats.MaxConnections),
			slog.String("group_id", groupID),
			slog.Int64("group_count", h.limits.GroupConnectionCount(groupID)),
			slog.Int64("max_per_group", stats.MaxPerGroup))
		http.Error(w, "Service at capacity, please try again later", http.StatusServiceUnavailable)
		return
	}

	// Set session cookie if this is a new session (cookie doesn't exist)
	setCookieIfNew(w, r, groupID, h.config.CookieMaxAge)

	// Upgrade to WebSocket after authentication and limit check succeeds
	conn, err := h.config.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed",
			slog.String("component", "live_handler"),
			slog.Any("error", err))
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			slog.Warn("WebSocket close error",
				slog.String("component", "live_handler"),
				slog.Any("error", err))
		}
	}()

	// Acquire connection slot (increment counters)
	if err := h.limits.Acquire(groupID); err != nil {
		slog.Error("Failed to acquire connection slot",
			slog.String("component", "live_handler"),
			slog.Any("error", err))
		if writeErr := conn.WriteMessage(WSCloseMessage, WSFormatCloseMessage(WSCloseServiceRestart, "Service at capacity")); writeErr != nil {
			slog.Warn("Failed to send close message",
				slog.String("component", "live_handler"),
				slog.Any("error", writeErr))
		}
		return
	}
	defer h.limits.Release(groupID)

	slog.Info("Client connected",
		slog.String("component", "live_handler"),
		slog.String("user_id", userID),
		slog.String("group_id", groupID),
		slog.String("remote_addr", r.RemoteAddr),
		slog.Int64("active_connections", h.limits.ActiveConnections()))

	// Clone template for this connection to avoid state conflicts
	// Each WebSocket connection needs its own template instance because
	// ExecuteUpdates() tracks state (lastTree, lastData, etc.)
	connTmpl, err := h.config.Template.Clone()
	if err != nil {
		slog.Error("Failed to clone template",
			slog.String("component", "live_handler"),
			slog.Any("error", err))
		return
	}

	// Get or create state for this session group.
	// If persist fields exist, try to restore them from SessionStore.
	// Otherwise, always start with a fresh clone (ephemeral).
	ctx := r.Context()
	var typedState interface{}
	var isReconnect bool
	if restored, ok := h.restorePersistedState(ctx, groupID); ok {
		typedState = restored
		isReconnect = true
	} else {
		typedState, err = h.cloneStateTyped()
		if err != nil {
			slog.Error("Failed to clone state",
				slog.String("component", "live_handler"),
				slog.Any("error", err))
			return
		}
		if h.persistable == nil {
			slog.Debug("Using fresh state (no persist fields)",
				slog.String("component", "live_handler"),
				slog.String("group_id", groupID))
		} else {
			slog.Info("Created new session group",
				slog.String("component", "live_handler"),
				slog.String("group_id", groupID))
		}
	}

	// Initialize upload registry for this connection
	uploadRegistry := h.newUploadRegistry()
	for name, config := range h.config.UploadConfigs {
		if err := uploadRegistry.CreateUpload(name, config); err != nil {
			slog.Warn("Failed to create upload",
				slog.String("component", "live_handler"),
				slog.String("upload_name", name),
				slog.Any("error", err))
		}
	}
	connTmpl.SetUploadRegistry(uploadRegistry)

	// Create Connection and register in registry
	connection := &session.Connection{
		Conn:     conn,
		GroupID:  groupID,
		UserID:   userID,
		Template: connTmpl,
		Stores:   typedState, // Store typed state for peer-fan-out
		Uploads:  uploadRegistry,
	}

	h.registry.Register(connection, h.config.wsBufferSize)
	defer h.registry.Unregister(connection)
	// Registered after the Unregister defer so LIFO runs this first, while
	// conn.subscribedTopics is still live (Unregister nils it). Balances the
	// cross-instance topic SUBSCRIBEs relayed during this connection's life.
	defer h.releaseRelayedTopics(connection)
	defer func() {
		if h.tempFileManager == nil {
			return
		}
		if err := h.tempFileManager.RemoveSession(groupID); err != nil {
			slog.Warn("Failed to clean up temp files",
				slog.String("component", "live_handler"),
				slog.String("group_id", groupID),
				slog.Any("error", err))
		}
	}()
	slog.Debug("Registered connection",
		slog.String("component", "live_handler"),
		slog.Int("total_connections", h.registry.Count()),
		slog.Int("total_groups", h.registry.GroupCount()))

	// Subscribe to scoped pub/sub channels for cross-instance broadcasting.
	// Subscribers retry internally; errors here mean retries were exhausted.
	//
	// Each successful Subscribe* increments a refcount in the broadcaster.
	// The matching deferred Unsubscribe* below decrements on disconnect so
	// idle channels are torn down (#214). Only successful subscribes are
	// recorded so partial-failure setup paths don't over-release.
	if ds, ok := h.config.PubSubBroadcaster.(pubsub.DynamicSubscriber); ok {
		var (
			subscribedGroup        bool
			subscribedGroupAction  bool
			subscribedUser         bool
			subscribedServerAction bool
		)
		gas, hasGroupAction := h.config.PubSubBroadcaster.(pubsub.GroupActionSubscriber)

		if err := ds.SubscribeToGroup(groupID); err != nil {
			slog.Error("Failed to subscribe to group channel",
				slog.String("component", "live_handler"),
				slog.String("group_id", groupID),
				slog.Any("error", err))
		} else {
			subscribedGroup = true
		}
		if hasGroupAction {
			if err := gas.SubscribeToGroupAction(groupID); err != nil {
				slog.Error("Failed to subscribe to group action channel",
					slog.String("component", "live_handler"),
					slog.String("group_id", groupID),
					slog.Any("error", err))
			} else {
				subscribedGroupAction = true
			}
		}
		if userID != "" {
			if err := ds.SubscribeToUser(userID); err != nil {
				slog.Error("Failed to subscribe to user channel",
					slog.String("component", "live_handler"),
					slog.String("user_id", userID),
					slog.Any("error", err))
			} else {
				subscribedUser = true
			}
			if err := ds.SubscribeToServerAction(userID); err != nil {
				slog.Error("Failed to subscribe to server action channel",
					slog.String("component", "live_handler"),
					slog.String("user_id", userID),
					slog.Any("error", err))
			} else {
				subscribedServerAction = true
			}
		}

		defer func() {
			if subscribedGroup {
				if err := ds.UnsubscribeFromGroup(groupID); err != nil {
					slog.Warn("Failed to unsubscribe from group channel",
						slog.String("component", "live_handler"),
						slog.String("group_id", groupID),
						slog.Any("error", err))
				}
			}
			if subscribedGroupAction {
				if err := gas.UnsubscribeFromGroupAction(groupID); err != nil {
					slog.Warn("Failed to unsubscribe from group action channel",
						slog.String("component", "live_handler"),
						slog.String("group_id", groupID),
						slog.Any("error", err))
				}
			}
			if subscribedUser {
				if err := ds.UnsubscribeFromUser(userID); err != nil {
					slog.Warn("Failed to unsubscribe from user channel",
						slog.String("component", "live_handler"),
						slog.String("user_id", userID),
						slog.Any("error", err))
				}
			}
			if subscribedServerAction {
				if err := ds.UnsubscribeFromServerAction(userID); err != nil {
					slog.Warn("Failed to unsubscribe from server action channel",
						slog.String("component", "live_handler"),
						slog.String("user_id", userID),
						slog.Any("error", err))
				}
			}
		}()
	}

	// Create connection state (messages are per-connection, not shared)
	connSt := &connState{
		state:    typedState,
		messages: make(map[string]string),
		groupID:  groupID,
	}

	// Use r.Context() (not context.Background()) so Mount/OnConnect cancel on disconnect (#303).
	wsQueryData := send.QueryParamsToData(r)
	connectKind := ConnectKindNewConnect
	if isReconnect {
		connectKind = ConnectKindReconnect
	}
	lifecycleCtx := NewContext(ctx, "", wsQueryData)
	lifecycleCtx = lifecycleCtx.WithUserID(userID)
	lifecycleCtx = lifecycleCtx.WithGroupID(groupID)
	lifecycleCtx = lifecycleCtx.WithTopicSubscriber(h.topicSubscriberFor(connection, r))
	lifecycleCtx = lifecycleCtx.WithFlashSetter(connSt)
	lifecycleCtx = lifecycleCtx.WithSession(newLocalSession(h, groupID))
	lifecycleCtx = lifecycleCtx.WithConnectKind(connectKind)

	// Call Mount on every WebSocket connect (new session AND reconnect).
	// Mount() refreshes state from the database, ensuring actions always
	// work with fresh data. Keep Mount cheap — it runs on every connect.
	newState, err := callMount(h.config.Controller, connSt.state, lifecycleCtx)
	if err != nil {
		// Non-TopicForbidden Mount error: genuine server fault → log and close
		// (the deferred Unregister at the top of this handler closes the WS).
		var tfe *TopicForbiddenError
		if !errors.As(err, &tfe) {
			slog.Error("Mount failed",
				slog.String("component", "live_handler"),
				slog.Any("error", err))
			return
		}
		// ACL-denied ctx.Subscribe in WS-connect Mount: expected access-control
		// outcome, not a server fault. Emit the envelope so the TS client
		// surfaces it as lvt:error, then keep the connection open — closing
		// trips the client's auto-reconnect into a re-Mount → re-deny → storm
		// (V14 / phase-4.md). Fall through with the controller's returned
		// newState: for the canonical `return s, err`, that is the pre-Subscribe
		// state and is correct; if the controller mutates `s` before the denied
		// Subscribe, that partially-modified state is silently adopted (no
		// rollback, consistent with Go error-handling conventions — callers
		// who need a clean rollback must not mutate before a may-deny Subscribe).
		h.sendTopicForbiddenEnvelope(connection, tfe.Topic)
		slog.Warn("Mount Subscribe denied by topic ACL; surfaced to client, connection kept open",
			slog.String("component", "live_handler"),
			slog.String("event", "topic_acl_denied_keep_open"),
			slog.String("topic", tfe.Topic),
			slog.Any("error", err))
	}
	connSt.state = newState
	h.persistState(ctx, groupID, connSt.state)

	// Call OnConnect lifecycle method
	newState, err = callOnConnect(h.config.Controller, connSt.state, lifecycleCtx)
	if err != nil {
		slog.Warn("OnConnect failed",
			slog.String("component", "live_handler"),
			slog.Any("error", err))
		// Continue anyway - OnConnect errors are non-fatal
	} else {
		connSt.state = newState
	}

	// Drain ctx.Publish issued from Mount/OnConnect (post-persistState, so a
	// reconciler re-reading shared state sees committed data). Per the spec
	// (§"Publish on the GET-phase Mount is not a no-op"), Publish in Mount IS
	// processed and fans out on every connect/load — the documented footgun
	// callers guard with ctx.IsInitialMount() / ctx.IsReconnect().
	h.processTopicPublishes(connection, lifecycleCtx.pendingTopicPublishes())

	// Schedule OnDisconnect call when WebSocket closes
	defer callOnDisconnect(h.config.Controller)

	// Send initial tree
	var buf bytes.Buffer
	err = connTmpl.ExecuteUpdates(&buf, connSt.state, connSt.getMessages())
	if err != nil {
		slog.Error("Failed to generate initial tree",
			slog.String("component", "live_handler"),
			slog.Any("error", err))
		return
	}

	var tree map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &tree); err != nil {
		slog.Error("Failed to parse initial tree",
			slog.String("component", "live_handler"),
			slog.Any("error", err))
		return
	}

	response := UpdateResponse{
		Tree: tree,
		Meta: &ResponseMetadata{
			Success:      !connSt.hasErrors(),
			Errors:       connSt.getErrorsOnly(),
			Capabilities: h.config.Capabilities,
		},
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		slog.Error("Failed to marshal initial response",
			slog.String("component", "live_handler"),
			slog.Any("error", err))
		return
	}

	if err = writeUpdateWebSocket(connection, responseBytes); err != nil {
		slog.Error("Failed to send initial tree",
			slog.String("component", "live_handler"),
			slog.Any("error", err))
		return
	}

	// Create rate limiter for this connection (prevents DoS attacks)
	var limiter *rate.Limiter
	if h.config.Template.config.MessageRateLimit > 0 {
		burst := h.config.Template.config.MessageRateBurst
		if burst < 1 {
			burst = 1
		}
		limiter = rate.NewLimiter(rate.Limit(h.config.Template.config.MessageRateLimit), burst)
	}

	// Start readPump goroutine: reads from WebSocket and sends to readChan.
	// This decouples WebSocket reads from state mutations, allowing the event
	// loop to also process broadcast dispatches via DispatchChan.
	// Buffer of 1: deliberately serializes client messages with dispatch processing.
	// During dispatch handling, the readPump blocks after buffering one message.
	// This bounds memory and ensures state mutations are strictly sequential.
	// Tradeoff: slow dispatched actions (e.g., DB calls) pause client message processing.
	// Increasing the buffer would NOT help — state mutations must still be sequential.
	readChan := make(chan wsReadMessage, 1)
	go func() {
		defer close(readChan)
		for {
			_, data, err := conn.ReadMessage()
			// If both readChan and Done are ready, Go's select may pick Done,
			// discarding the error. The event loop still exits via readChan close.
			select {
			case readChan <- wsReadMessage{data: data, err: err}:
			case <-connection.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()

	// Event loop: processes both client messages (readChan) and
	// broadcast action dispatches (DispatchChan) serially.
	// All state mutations happen in this single goroutine — no mutex needed.
eventLoop:
	for {
		select {
		case rm, ok := <-readChan:
			if !ok {
				break eventLoop
			}
			if rm.err != nil {
				if WSIsUnexpectedCloseError(rm.err, WSCloseGoingAway, WSCloseAbnormalClosure) {
					slog.Warn("WebSocket error",
						slog.String("component", "live_handler"),
						slog.Any("error", rm.err))
				}
				break eventLoop
			}

			// Rate limiting check
			if limiter != nil && !limiter.Allow() {
				errorResp := UpdateResponse{
					Tree: nil,
					Meta: &ResponseMetadata{
						Success: false,
						Errors:  map[string]string{"_rate_limit": "Too many requests. Please slow down."},
					},
				}
				if respBytes, err := json.Marshal(errorResp); err == nil {
					_ = writeUpdateWebSocket(connection, respBytes)
				}
				continue
			}

			// Parse message
			msg, err := parseActionFromWebSocket(rm.data)
			if err != nil {
				slog.Warn("Failed to parse message",
					slog.String("component", "live_handler"),
					slog.Any("error", err))
				continue
			}

			// Check if this is an upload-related action
			uploadHandled, err := h.handleUploadAction(r.Context(), rm.data, msg, connSt, uploadRegistry, connection)
			if err != nil {
				slog.Warn("Upload action error",
					slog.String("component", "live_handler"),
					slog.Any("error", err))
				continue
			}
			if uploadHandled {
				continue
			}

			// Route forms without explicit action to the conventional Submit() method.
			applyDefaultAction(&msg)

			// Clear previous errors
			connSt.clearErrors()

			// Create Context for action dispatch.
			// Note: Query params from initial WS connection are NOT included here.
			// They're already available in Mount/OnConnect via wsQueryData.
			// WebSocket actions use only msg.Data from the client message.
			actionCtx := NewContext(r.Context(), msg.Action, msg.Data)
			actionCtx = actionCtx.WithUserID(userID)
			actionCtx = actionCtx.WithGroupID(groupID)
			actionCtx = actionCtx.WithTopicSubscriber(h.topicSubscriberFor(connection, r))
			actionCtx = actionCtx.WithUploads(uploadRegistry)
			actionCtx = actionCtx.WithFlashSetter(connSt)
			actionCtx = actionCtx.WithSession(newLocalSession(h, groupID))
			if schema := connTmpl.formSchema; schema != nil {
				actionCtx = actionCtx.WithFormSchema(schema)
			}

			// actionNavigate re-runs Mount with msg.Data as query params. Rebind
			// actionCtx itself (not a discarded copy) so ctx.Publish calls in
			// Mount land on the context that processTopicPublishes reads.
			var newState interface{}
			var actionErr error
			if msg.Action == actionNavigate {
				actionCtx = actionCtx.WithAction("") // ctx.Action()=="" matches connect-time Mount
				newState, actionErr = callMount(h.config.Controller, connSt.state, actionCtx)
			} else {
				newState, actionErr = DispatchWithState(h.config.Controller, connSt.state, actionCtx)
			}
			if actionErr != nil {
				// Handle errors
				switch e := actionErr.(type) {
				case FieldError:
					connSt.setError(e.Field, e.Message)
				case MultiError:
					for _, fieldErr := range e {
						connSt.setError(fieldErr.Field, fieldErr.Message)
					}
				default:
					if !errors.Is(actionErr, ErrMethodNotFound) {
						connSt.setError("_general", actionErr.Error())
					}
				}
			} else {
				connSt.state = newState
			}

			if actionErr == nil {
				h.persistState(r.Context(), groupID, connSt.state)
				connection.Stores = connSt.state
				h.processTopicPublishes(connection, actionCtx.pendingTopicPublishes())
			}

			// Generate tree update
			buf.Reset()
			if err = connTmpl.ExecuteUpdates(&buf, connSt.state, connSt.getMessages()); err != nil {
				slog.Error("Template update execution failed",
					slog.String("component", "live_handler"),
					slog.Any("error", err))
				continue
			}

			var tree map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &tree); err != nil {
				slog.Error("Failed to parse tree",
					slog.String("component", "live_handler"),
					slog.Any("error", err))
				continue
			}

			response := UpdateResponse{
				Tree: tree,
				Meta: &ResponseMetadata{
					Success: !connSt.hasErrors(),
					Errors:  connSt.getErrorsOnly(),
					Action:  msg.Action,
				},
			}

			responseBytes, err := json.Marshal(response)
			if err != nil {
				slog.Error("Failed to marshal response",
					slog.String("component", "live_handler"),
					slog.Any("error", err))
				continue
			}

			if err = writeUpdateWebSocket(connection, responseBytes); err != nil {
				slog.Error("WebSocket write failed",
					slog.String("component", "live_handler"),
					slog.Any("error", err))
				break eventLoop
			}

		case req := <-connection.DispatchChan:
			h.handleDispatchedAction(connSt, connection, req, userID)
		}
	}

	slog.Info("Client disconnected",
		slog.String("component", "live_handler"),
		slog.String("user_id", userID),
		slog.String("group_id", groupID),
		slog.Int("remaining_connections", h.registry.Count()))
}

// wantsJSON returns true if the client expects a JSON response.
// JS clients (fetch/XHR) send Accept: application/json, while browsers send text/html.
// This is used for progressive enhancement to detect whether to return HTML or JSON.
//
// The function parses the Accept header and checks if the first meaningful media type
// is application/json (or a +json subtype). This avoids treating browsers that include
// application/json as a secondary option as JSON clients.
// knownAssetExts lists file extensions that browsers request automatically
// (favicon, manifest, etc.) and should not trigger pathChanged navigation logic.
var knownAssetExts = map[string]bool{
	".ico": true, ".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true, ".webp": true,
	".css": true, ".js": true, ".mjs": true, ".map": true, ".woff": true, ".woff2": true, ".ttf": true,
	".json": true, ".xml": true, ".txt": true, ".webmanifest": true,
}

// isKnownAssetExt checks if the extension (from path.Ext, includes leading dot)
// matches a known static asset type. Uses ToLower for case-insensitive matching.
func isKnownAssetExt(ext string) bool {
	return knownAssetExts[strings.ToLower(ext)]
}

func wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return false
	}

	// Parse the Accept header and consider only the first meaningful media range.
	// This avoids treating browsers that primarily prefer text/html as JSON clients
	// when they include application/json as a secondary option.
	for _, part := range strings.Split(accept, ",") {
		mt := strings.TrimSpace(part)
		if mt == "" {
			continue
		}

		// Strip any parameters (e.g. ";q=0.9").
		if semi := strings.Index(mt, ";"); semi != -1 {
			mt = strings.TrimSpace(mt[:semi])
		}

		// Ignore wildcard entries like "*/*".
		if mt == "*/*" {
			continue
		}

		// Treat explicit JSON types (including +json subtypes) as JSON.
		return mt == "application/json" || strings.HasSuffix(mt, "+json")
	}

	return false
}

// setCookieIfNew sets the livetemplate-id cookie if it doesn't already exist
func setCookieIfNew(w http.ResponseWriter, r *http.Request, groupID string, cookieMaxAge time.Duration) {
	// Check if cookie already exists
	if cookie, err := r.Cookie("livetemplate-id"); err == nil && cookie.Value == groupID {
		// Cookie exists and matches - no need to set again
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "livetemplate-id",
		Value:    groupID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(cookieMaxAge.Seconds()),
	})
}

func (h *liveHandler) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle HEAD request for capability check
	if r.Method == http.MethodHead {
		return
	}

	// Authenticate user and get session group
	userID, err := h.config.Authenticator.Identify(r)
	if err != nil {
		slog.Error("HTTP authentication failed",
			slog.String("component", "live_handler"),
			slog.Any("error", err))
		h.writeUnauthorized(w)
		return
	}

	groupID, err := h.config.Authenticator.GetSessionGroup(r, userID)
	if err != nil {
		slog.Error("Failed to get session group for HTTP",
			slog.String("component", "live_handler"),
			slog.Any("error", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set session cookie if this is a new session
	setCookieIfNew(w, r, groupID, h.config.CookieMaxAge)

	// Get or create state for this session group
	ctx := r.Context()
	isNewSession := false
	var typedState interface{}

	// Detect URL path change on GET requests to reset stale cached state
	// (e.g., page-mode navigation from /posts/alpha to /posts/beta).
	// POST requests are excluded — they target actions, not page navigations.
	// Paths are normalized via path.Clean to treat /a/ and /a as identical.
	currentPath := path.Clean(r.URL.Path)
	if currentPath == "." {
		currentPath = "/"
	}
	// Asset requests (favicon.ico, manifest.json, etc.) that hit catch-all
	// handlers are not page navigations and must not trigger pathChanged.
	isAssetRequest := isKnownAssetExt(path.Ext(currentPath))
	pathChanged := false
	if h.persistable != nil && r.Method == http.MethodGet && !isAssetRequest {
		if prev, loaded := h.httpLastPaths.Load(groupID); loaded {
			pathChanged = prev.(string) != currentPath
		}
	}

	if pathChanged {
		// Path changed — use fresh state for the new URL (persist fields reset).
		typedState, err = h.cloneStateTyped()
		if err != nil {
			slog.Error("Failed to clone per-request state",
				slog.String("component", "live_handler"),
				slog.Any("error", err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		isNewSession = true
		h.httpTemplates.Delete(groupID)
		slog.Debug("Refreshing session for new URL path",
			slog.String("component", "live_handler"),
			slog.String("group_id", groupID),
			slog.String("path", currentPath))
	} else if restored, ok := h.restorePersistedState(ctx, groupID); ok {
		typedState = restored
		slog.Debug("Using existing session group",
			slog.String("component", "live_handler"),
			slog.String("group_id", groupID))
	} else {
		// No stored state or no persist fields — start fresh
		typedState, err = h.cloneStateTyped()
		if err != nil {
			slog.Error("Failed to clone state",
				slog.String("component", "live_handler"),
				slog.Any("error", err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if h.persistable == nil {
			// No persist fields — ephemeral mode.
			// On GET, delete cached template so first render sends full statics.
			// On POST, keep cache so responses are incremental diffs.
			if r.Method == http.MethodGet {
				h.httpTemplates.Delete(groupID)
			}
		} else {
			isNewSession = true
			h.httpTemplates.Delete(groupID)
			slog.Info("Created new session group",
				slog.String("component", "live_handler"),
				slog.String("group_id", groupID))
		}
	}

	// Create connection state (messages are per-request)
	connSt := &connState{
		state:    typedState,
		messages: make(map[string]string),
		groupID:  groupID,
	}

	// Create lifecycle context with query params
	queryData := send.QueryParamsToData(r)
	connectKind := ConnectKindAction
	if r.Method == http.MethodGet && !isAssetRequest {
		connectKind = ConnectKindInitialMount
	}
	lifecycleCtx := NewContext(ctx, "", queryData)
	lifecycleCtx = lifecycleCtx.WithUserID(userID)
	lifecycleCtx = lifecycleCtx.WithGroupID(groupID)
	lifecycleCtx = lifecycleCtx.WithTopicSubscriber(h.topicSubscriberFor(nil, r))
	lifecycleCtx = lifecycleCtx.WithFlashSetter(connSt)
	lifecycleCtx = lifecycleCtx.WithSession(newLocalSession(h, groupID))
	lifecycleCtx = lifecycleCtx.WithConnectKind(connectKind)

	// Read flash messages from cookie (set by POST redirect)
	if flashCookie, err := r.Cookie("lvt-flash"); err == nil && flashCookie.Value != "" {
		if flashValues, err := url.ParseQuery(flashCookie.Value); err == nil {
			for key, values := range flashValues {
				if len(values) > 0 {
					connSt.setFlash(key, values[0], 0)
				}
			}
		}
		// Clear cookie immediately (one-time read)
		http.SetCookie(w, &http.Cookie{
			Name:   "lvt-flash",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
	}

	isHTTPGet := r.Method == http.MethodGet && !isAssetRequest
	isPageRequest := !isAssetRequest
	if isNewSession || isPageRequest {
		newState, err := callMount(h.config.Controller, connSt.state, lifecycleCtx)
		if err != nil {
			// httpLastPaths still holds the previous path (Store is deferred
			// until after success), so retries naturally re-detect the change.
			slog.Error("Mount failed",
				slog.String("component", "live_handler"),
				slog.Any("error", err))
			http.Error(w, "Failed to initialize application state", http.StatusInternalServerError)
			return
		}
		connSt.state = newState
		// Persist after Mount on GET/new-session only. On POST the action handler
		// will persist after the action succeeds, avoiding a redundant Set.
		if isNewSession || isHTTPGet {
			h.persistState(ctx, groupID, connSt.state)
		}
		// Drain ctx.Publish issued from Mount on the HTTP path. Per the spec,
		// Publish in Mount is NOT a no-op (unlike Subscribe) — it is
		// transport-agnostic and fans out to existing subscribers on every
		// GET/POST (excludeConn nil: the HTTP responder is not a WS subscriber).
		h.processTopicPublishes(nil, lifecycleCtx.pendingTopicPublishes())
		// Commit path after successful Mount (not before, to allow retries).
		// Skip when no persist fields — pathChanged is never checked.
		if isHTTPGet && h.persistable != nil {
			h.httpLastPaths.Store(groupID, currentPath)
		}
	}

	// Handle GET request
	if r.Method == http.MethodGet {
		if wantsJSON(r) {
			// JS client in HTTP mode: return initial tree as JSON
			httpTmpl, cloneErr := h.config.Template.Clone()
			if cloneErr != nil {
				http.Error(w, "Failed to clone template", http.StatusInternalServerError)
				return
			}

			var buf bytes.Buffer
			if err := httpTmpl.ExecuteUpdates(&buf, connSt.state, connSt.getMessages()); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			var tree map[string]any
			if err := json.Unmarshal(buf.Bytes(), &tree); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			response := UpdateResponse{
				Tree: tree,
				Meta: &ResponseMetadata{
					Success:      true,
					Capabilities: h.config.Capabilities,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		// Browser: return initial HTML page
		err := h.config.Template.Execute(w, connSt.state, connSt.getMessages())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Handle POST request for actions
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Initialize upload registry for HTTP requests
	uploadRegistry := h.newUploadRegistry()
	for name, config := range h.config.UploadConfigs {
		if err := uploadRegistry.CreateUpload(name, config); err != nil {
			slog.Warn("Failed to create upload",
				slog.String("component", "live_handler"),
				slog.String("upload_name", name),
				slog.Any("error", err))
		}
	}

	ct := r.Header.Get("Content-Type")

	// Upload handshake over HTTP (WS-disabled fallback): the client posts the
	// upload_start JSON when the socket isn't open. Answer with the same
	// UploadStartResponse the WS path produces so mode dispatch + Direct presign
	// work without a WebSocket. JSON body, peeked for an upload action.
	if strings.HasPrefix(ct, "application/json") && len(h.config.UploadConfigs) > 0 {
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		if upload.IsUploadStart(body) {
			response, buildErr := h.buildUploadStartResponse(body, groupID, uploadRegistry)
			if buildErr != nil {
				http.Error(w, buildErr.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				slog.Warn("Failed to write upload_start HTTP response",
					slog.String("component", "live_handler"),
					slog.Any("error", err))
			}
			return
		}
		// Not an upload handshake — restore the body for normal action parsing.
		r.Body = io.NopCloser(bytes.NewReader(body))
	}

	// Proxied streaming uploads: a multipart request carrying a field whose Mode
	// is Proxied is iterated via MultipartReader (zero disk) and each file part is
	// streamed straight to Controller.OnUpload. This bypasses ParseMultipartForm
	// (which would stage large parts to os.TempDir) and the byte-progress wrapper
	// (which would consume the body before MultipartReader sees it).
	streamingRequest := strings.HasPrefix(ct, "multipart/form-data") && h.hasProxiedUpload()

	var msg message
	var streamErr error

	if streamingRequest {
		streamCtx := NewContext(r.Context(), "", nil)
		streamCtx = streamCtx.WithUserID(userID)
		streamCtx = streamCtx.WithGroupID(groupID)
		streamCtx = streamCtx.WithTopicSubscriber(h.topicSubscriberFor(nil, r))
		streamCtx = streamCtx.WithHTTP(w, r)
		streamCtx = streamCtx.WithUploads(uploadRegistry)
		streamCtx = streamCtx.WithSession(newLocalSession(h, groupID))

		values, serr := upload.StreamMultipart(r,
			func(field string) bool {
				c, ok := h.config.UploadConfigs[field]
				return ok && c.Mode == uploadtypes.UploadModeProxied
			},
			func(part *multipart.Part) error {
				return h.streamProxiedPart(part, uploadRegistry, streamCtx)
			},
			nil, // no staged sink: non-streaming file parts in a Proxied request are ignored
		)
		streamErr = serr
		msg = send.BuildActionFromValues(values)
		applyDefaultAction(&msg)
	} else {
		// Tier 1 file uploads: wrap request body with progress tracking before
		// any multipart parsing occurs. ParseMultipartForm reads through this
		// wrapper, giving us byte-level progress for WebSocket clients.
		if strings.HasPrefix(ct, "multipart/form-data") && r.ContentLength > 0 && len(h.config.UploadConfigs) > 0 {
			pr := upload.NewProgressReader(r.Body, r.ContentLength)
			pr.OnProgress = func(bytesRead, total int64) {
				conns := h.registry.GetByGroupExcept(groupID, nil)
				if len(conns) == 0 {
					return
				}
				pct := int(bytesRead * 100 / total)
				progressMsg := &upload.UploadProgressMessage{
					Type:       "upload_progress",
					UploadName: "multipart",
					Progress:   pct,
					BytesRecv:  bytesRead,
					BytesTotal: total,
				}
				progressBytes, err := upload.SerializeUploadProgressMessage(progressMsg)
				if err != nil {
					return
				}
				for _, conn := range conns {
					if err := conn.Send(WSTextMessage, progressBytes); err != nil {
						// Client may have disconnected — non-fatal for progress updates
						continue
					}
				}
			}
			r.Body = pr
		}

		// Parse message
		var err error
		msg, err = parseActionFromHTTP(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Route browser form submissions without explicit action to Submit().
		// Only apply for form Content-Types, not JSON action requests.
		if strings.HasPrefix(ct, "application/x-www-form-urlencoded") || strings.HasPrefix(ct, "multipart/form-data") {
			applyDefaultAction(&msg)
		}

		// Tier 1 file uploads: extract multipart files into the upload registry.
		// ParseMultipartForm was already called by parseActionFromHTTP above,
		// so r.MultipartForm is populated. We iterate configured upload fields
		// and extract matching files.
		if strings.HasPrefix(ct, "multipart/form-data") && len(h.config.UploadConfigs) > 0 {
			if registry, ok := uploadRegistry.(*upload.Registry); ok {
				tempMgr, tempErr := upload.NewTempFileManager("")
				if tempErr != nil {
					slog.Warn("Failed to create temp file manager for multipart upload",
						slog.String("component", "live_handler"),
						slog.Any("error", tempErr))
				} else {
					for name := range h.config.UploadConfigs {
						u := registry.GetUpload(name)
						if u == nil {
							continue
						}
						upl, ok := u.(*upload.Upload)
						if !ok {
							continue
						}
						entries, err := upload.ParseMultipartUpload(r, name, upl.Config, groupID, tempMgr)
						if err != nil {
							// No files for this field or parse error — not fatal
							slog.Debug("Multipart upload parse",
								slog.String("component", "live_handler"),
								slog.String("upload_name", name),
								slog.Any("result", err.Error()))
							continue
						}
						for _, entry := range entries {
							if err := upl.AddEntry(entry); err != nil {
								slog.Debug("Multipart upload entry rejected",
									slog.String("component", "live_handler"),
									slog.String("upload_name", name),
									slog.Any("error", err))
							}
						}
					}
				}
			}
		}
	}

	// Clear previous errors
	connSt.clearErrors()

	// Surface a streaming-upload failure as a field error before dispatch, so the
	// follow-on action still runs (and reads whatever uploads did succeed).
	if streamErr != nil {
		switch e := streamErr.(type) {
		case FieldError:
			connSt.setError(e.Field, e.Message)
		case MultiError:
			for _, fe := range e {
				connSt.setError(fe.Field, fe.Message)
			}
		case *upload.ValidationError:
			connSt.setError(e.Field, e.Message)
		default:
			connSt.setError("_general", streamErr.Error())
		}
	}

	// __navigate__ is a WebSocket-only reserved action. Reject early so HTTP
	// clients get a clear "wrong transport" error instead of a confusing
	// ErrMethodNotFound response.
	if msg.Action == actionNavigate {
		http.Error(w, "action __navigate__ is only supported over WebSocket", http.StatusBadRequest)
		return
	}

	// Merge query params with form data (form data takes precedence)
	mergedData := send.MergeData(queryData, msg.Data)

	// Create Context for action dispatch (with HTTP context for SetCookie, Redirect)
	actionCtx := NewContext(r.Context(), msg.Action, mergedData)
	actionCtx = actionCtx.WithUserID(userID)
	actionCtx = actionCtx.WithGroupID(groupID)
	actionCtx = actionCtx.WithTopicSubscriber(h.topicSubscriberFor(nil, r))
	actionCtx = actionCtx.WithHTTP(w, r)
	actionCtx = actionCtx.WithUploads(uploadRegistry)
	actionCtx = actionCtx.WithFlashSetter(connSt)
	actionCtx = actionCtx.WithSession(newLocalSession(h, groupID))
	if schema := h.config.Template.formSchema; schema != nil {
		actionCtx = actionCtx.WithFormSchema(schema)
	}

	// Dispatch action using Controller+State pattern
	newState, actionErr := DispatchWithState(h.config.Controller, connSt.state, actionCtx)
	if actionErr != nil {
		switch e := actionErr.(type) {
		case FieldError:
			connSt.setError(e.Field, e.Message)
		case MultiError:
			for _, fieldErr := range e {
				connSt.setError(fieldErr.Field, fieldErr.Message)
			}
		default:
			if !errors.Is(actionErr, ErrMethodNotFound) {
				connSt.setError("_general", actionErr.Error())
			}
		}
	} else {
		connSt.state = newState
	}

	if actionErr == nil {
		h.persistState(r.Context(), groupID, connSt.state)
	}

	// Get or create cached template for this session group.
	// Unlike WebSocket (which keeps a clone per connection), HTTP needs
	// a cache keyed by groupID so that subsequent POSTs can produce
	// diffs against the previous tree instead of full renders.
	// A per-entry mutex serializes concurrent requests for the same group
	// (e.g., multiple browser tabs) to avoid data races on template state.
	var entry *httpTemplateCacheEntry
	if cached, ok := h.httpTemplates.Load(groupID); ok {
		entry = cached.(*httpTemplateCacheEntry)
	} else {
		cloned, cloneErr := h.config.Template.Clone()
		if cloneErr != nil {
			http.Error(w, "Failed to clone template", http.StatusInternalServerError)
			return
		}
		newEntry := &httpTemplateCacheEntry{tmpl: cloned}
		newEntry.lastAccessed.Store(time.Now().Unix())
		if existing, loaded := h.httpTemplates.LoadOrStore(groupID, newEntry); loaded {
			entry = existing.(*httpTemplateCacheEntry)
		} else {
			entry = newEntry
		}
	}
	if h.persistable == nil {
		entry.lastAccessed.Store(time.Now().Unix())
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	httpTmpl := entry.tmpl
	httpTmpl.SetUploadRegistry(uploadRegistry)

	if actionErr == nil {
		h.processTopicPublishes(nil, actionCtx.pendingTopicPublishes())
	}

	// Check if we should return HTML for progressive enhancement
	// Progressive enhancement is enabled AND client does not want JSON
	if h.config.ProgressiveEnhancement && !wantsJSON(r) {
		// If the action handler already sent a redirect (via ctx.Redirect()),
		// skip the PRG redirect to avoid a superfluous redirect response.
		// Flash set before the redirect is lost — no cookie is written here.
		if actionCtx.redirected != nil && *actionCtx.redirected {
			return
		}

		// Non-JS client: return HTML response using POST-Redirect-GET pattern
		if connSt.hasErrors() {
			// Validation errors: re-render page with errors inline (no redirect)
			// Write to buffer first to handle template errors gracefully
			var buf bytes.Buffer
			if err := httpTmpl.Execute(&buf, connSt.state, connSt.getMessages()); err != nil {
				slog.Error("Template execution failed",
					slog.String("component", "live_handler"),
					slog.Any("error", err))
				http.Error(w, "An error occurred rendering the page. Please try again.", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if _, err := w.Write(buf.Bytes()); err != nil {
				slog.Warn("Failed to write validation error response",
					slog.String("component", "live_handler"),
					slog.Any("error", err))
			}
			// Flash messages are preserved so they show in the re-rendered page
			return
		}

		// Success: redirect to prevent duplicate submissions on refresh (PRG pattern)
		redirectURL := r.URL.Path
		if encoded := r.URL.Query().Encode(); encoded != "" {
			redirectURL += "?" + encoded
		}

		// Set flash messages via cookie (consumed on next GET)
		if flashVals := connSt.getFlashValues(); len(flashVals) > 0 {
			http.SetCookie(w, &http.Cookie{
				Name:     "lvt-flash",
				Value:    flashVals.Encode(),
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				MaxAge:   10,
			})
		}

		http.Redirect(w, r, redirectURL, http.StatusSeeOther) // 303 See Other
		return
	}

	// JS client: return JSON tree update (existing behavior)
	var buf bytes.Buffer
	if err = httpTmpl.ExecuteUpdates(&buf, connSt.state, connSt.getMessages()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var tree map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &tree); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := UpdateResponse{
		Tree: tree,
		Meta: &ResponseMetadata{
			Success: !connSt.hasErrors(),
			Errors:  connSt.getErrorsOnly(),
			Action:  msg.Action,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

// newUploadRegistry creates a new upload registry instance.
// This is called for each connection to create isolated upload state.
func (h *liveHandler) newUploadRegistry() uploadRegistry {
	return h.config.Template.newUploadRegistry()
}

// =============================================================================
// Controller+State Pattern Helpers
// =============================================================================

// cloneStateTyped creates a typed clone using the State interface.
// Returns the underlying value (not the State wrapper).
// This ensures state contains only pure data (serialization = purity marker).
func (h *liveHandler) cloneStateTyped() (interface{}, error) {
	if h.config.State == nil {
		return nil, fmt.Errorf("no state configured")
	}

	// Serialize the initial state
	data, err := h.config.State.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize initial state: %w", err)
	}

	// Get the type of the inner value
	innerVal := h.config.State.Inner()
	innerType := reflect.TypeOf(innerVal)
	if innerType.Kind() == reflect.Pointer {
		innerType = innerType.Elem()
	}

	// Create new instance
	newStatePtr := reflect.New(innerType)

	// Deserialize into it (JSON since that's what jsonState uses)
	if err := json.Unmarshal(data, newStatePtr.Interface()); err != nil {
		return nil, fmt.Errorf("failed to deserialize state: %w", err)
	}

	// Return the dereferenced value (to match method signatures)
	return newStatePtr.Elem().Interface(), nil
}

// persistState saves persist-tagged fields to the SessionStore.
// No-op if the state has no persist fields.
func (h *liveHandler) persistState(ctx context.Context, groupID string, state interface{}) {
	if h.persistable == nil {
		return
	}
	data, err := h.persistable.ExtractPersistFields(state)
	if err != nil {
		slog.Error("Failed to extract persist fields",
			slog.String("component", "live_handler"),
			slog.String("group_id", groupID),
			slog.Any("error", err))
		return
	}
	h.config.SessionStore.Set(ctx, groupID, data)
}

// restorePersistedState creates a state with persist fields restored from SessionStore.
// Returns nil if no stored state exists or no persist fields are configured.
func (h *liveHandler) restorePersistedState(ctx context.Context, groupID string) (interface{}, bool) {
	if h.persistable == nil {
		return nil, false
	}
	stored := h.config.SessionStore.Get(ctx, groupID)
	if stored == nil {
		return nil, false
	}
	// stored can be []byte (from MemorySessionStore or RedisSessionStore)
	data, ok := stored.([]byte)
	if !ok {
		slog.Warn("Unexpected stored state type, ignoring",
			slog.String("component", "live_handler"),
			slog.String("group_id", groupID),
			slog.String("type", fmt.Sprintf("%T", stored)))
		return nil, false
	}
	state, err := h.persistable.InjectPersistFields(data)
	if err != nil {
		slog.Error("Failed to inject persist fields",
			slog.String("component", "live_handler"),
			slog.String("group_id", groupID),
			slog.Any("error", err))
		return nil, false
	}
	return state, true
}

// processTopicPublishes drains pending ctx.Publish requests to topic
// subscribers. Called at the post-action, post-persistState site so a
// reconciler that re-reads the group-keyed session store sees the
// originator's committed write (persist-before-publish ordering — a hard
// requirement).
func (h *liveHandler) processTopicPublishes(excludeConn *session.Connection, pubs []topicPublish) {
	for _, p := range pubs {
		h.dispatchToTopic(p.Topic, excludeConn, p.Action, p.Data)
	}
}

// dispatchToTopic fans out one Publish to the local subscribers of a topic and
// (when a PubSubBroadcaster is wired) republishes it for cross-instance
// delivery. Each receiver runs the action against its OWN state via
// handleDispatchedAction — that is what makes the per-connection-state/
// reconciler guarantee free (no merge machinery). segmentMatch must be passed
// in: internal/session cannot import the root package (import cycle), and nil +
// pattern subscribers panics by design (Phase 0 contract — never a silent
// exact-only degradation).
func (h *liveHandler) dispatchToTopic(topic string, excludeConn *session.Connection, action string, data map[string]interface{}) {
	// Local fan-out: every local subscriber runs the action against its OWN
	// state via handleDispatchedAction. Kind is KindAction (the only v1 value),
	// set explicitly as a forward-compat placeholder.
	conns := h.registry.GetByTopicExcept(topic, excludeConn, segmentMatch)
	for _, conn := range conns {
		conn.EnqueueDispatch(&session.DispatchRequest{Action: action, Data: data, Kind: session.KindAction})
	}

	// Remote fan-out: publish to Redis over the single exact channel
	// livetemplate:topic:{name} for other instances. The publishing instance's
	// own SUBSCRIBE round-trips its message back, but RedisBroadcaster.handleMessage
	// drops same-instance messages (InstanceID filter) so local connections are
	// not double-dispatched.
	if tab, ok := h.config.PubSubBroadcaster.(pubsub.TopicActionBroadcaster); ok {
		if err := tab.PublishToTopic(topic, action, data); err != nil {
			slog.Warn("Failed to publish topic action to PubSub",
				slog.String("component", "live_handler"),
				slog.String("topic", topic),
				slog.String("action", action),
				slog.Any("error", err))
		}
	}

	slog.Debug("Dispatched publish to topic",
		slog.String("component", "live_handler"),
		slog.String("topic", topic),
		slog.String("action", action),
		slog.Int("local_connections", len(conns)))
}

// Publish is the out-of-band entry point (LiveHandler.Publish): same topic
// fan-out as ctx.Publish but with no Context. Validation mirrors ctx.Publish
// (empty checks; developer-grammar for non-lvt: topics; lvt: permitted on the
// send side without a SelfTopic()-equality check — anti-spoof is Subscribe-side
// only, §3). It runs NO ACL (send-side ungated, §3) and NO recursion guard
// (not inside a dispatched action — there is no Context to have one). There is
// no sender connection, so excludeConn is nil (every subscriber receives it).
//
// The symmetry-collision slog.Warn (the ctx.Publish footgun guard for
// copy-pasted client scaffolds reusing a wired action name) is intentionally
// NOT emitted here: out-of-band Publish is deliberate trusted server code, not
// a scaffold, and there is no per-Context template binding to resolve a
// client-wired name set against. (Recorded in learnings/phase-2.md.)
//
// Unlike ctx.Publish there is no action/persistState cycle to order against:
// an out-of-band caller is responsible for committing any state mutation
// before calling Publish (the same contract any external mutation has), so the
// dispatch happens immediately rather than being queued for a post-action
// drain.
func (h *liveHandler) Publish(topic, action string, data map[string]interface{}) error {
	if topic == "" {
		return fmt.Errorf("livetemplate: cannot Publish to an empty topic")
	}
	if action == "" {
		return fmt.Errorf("livetemplate: cannot Publish with an empty action")
	}
	// Publish targets a concrete topic; "*" is Subscribe-only (same rule as ctx.Publish — reject, don't panic GetByTopicExcept).
	if isPatternTopic(topic) {
		return fmt.Errorf("livetemplate: cannot Publish to wildcard pattern %q — publish to a concrete topic; patterns are Subscribe-only", topic)
	}
	if !isReservedTopic(topic) {
		if err := validateDeveloperTopic(topic); err != nil {
			return err
		}
	}

	h.dispatchToTopic(topic, nil, action, data)
	return nil
}

// handleDispatchedAction processes a broadcast action received via DispatchChan.
// Called from the connection's event loop goroutine, so all state access is serialized.
// Rate limiting is intentionally not applied — these are server-originated dispatches.
func (h *liveHandler) handleDispatchedAction(connSt *connState, connection *session.Connection, req *session.DispatchRequest, userID string) {
	connSt.clearErrors()

	ctx := NewContext(context.Background(), req.Action, req.Data)
	ctx = ctx.WithUserID(userID)
	ctx = ctx.WithGroupID(connSt.groupID)
	ctx = ctx.WithTopicSubscriber(h.topicSubscriberFor(connection, nil))
	ctx = ctx.WithFlashSetter(connSt)
	// Wire Session so dispatched actions (from Publish fan-out or
	// Session.TriggerAction) can also call ctx.Session().TriggerAction
	// for follow-on server pushes. pendingTopicPublishes from ctx is still
	// dropped below to prevent storm loops, but TriggerAction goes
	// through a different queue (EnqueueDispatch directly) and is
	// allowed — each hop runs through a connection event loop, so the
	// only unbounded failure mode is a handler that recursively
	// re-triggers itself, which is a caller bug rather than framework
	// amplification. The dispatched-flagged session emits an
	// observability log on chained TriggerAction (#337).
	ctx = ctx.WithSession(newLocalSessionFromDispatched(h, connSt.groupID))
	if schema := h.config.Template.formSchema; schema != nil {
		ctx = ctx.WithFormSchema(schema)
	}

	newState, err := DispatchWithState(h.config.Controller, connSt.state, ctx)
	if err != nil {
		if !errors.Is(err, ErrMethodNotFound) {
			slog.Warn("Dispatched action failed",
				slog.String("component", "live_handler"),
				slog.String("action", req.Action),
				slog.String("group_id", connSt.groupID),
				slog.Any("error", err))
		}
		return
	}

	connSt.state = newState
	connection.Stores = connSt.state
	h.persistState(context.Background(), connSt.groupID, connSt.state)

	// Chained Publish calls from dispatched actions are intentionally dropped:
	// the shared resolver means a topic action could re-Publish on every peer,
	// so the one-hop guard bounds the cascade (not the first hop).
	if dropped := ctx.pendingTopicPublishes(); len(dropped) > 0 {
		slog.Error("Publish calls inside a dispatched action are ignored (prevents broadcast storms)",
			slog.String("component", "live_handler"),
			slog.String("action", req.Action),
			slog.Int("dropped_count", len(dropped)))
	}

	if err := h.sendUpdate(connection, connSt.state, connSt.getMessages()); err != nil {
		slog.Warn("sendUpdate failed during dispatched action",
			slog.String("component", "live_handler"),
			slog.String("action", req.Action),
			slog.Any("error", err))
	}
}

// httpTemplateSweepLoop periodically removes cached HTTP templates to prevent
// unbounded memory growth. In persistent mode, evicts entries whose session no
// longer exists in the SessionStore. In ephemeral mode (no SessionStore), evicts
// entries idle for longer than ephemeralSweepTTL.
func (h *liveHandler) httpTemplateSweepLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-h.shutdownChan:
			return
		case <-ticker.C:
			h.sweepStaleHTTPTemplates()
		}
	}
}

func (h *liveHandler) sweepStaleHTTPTemplates() {
	sweptSessions := make(map[string]struct{})

	if h.persistable == nil {
		// No persist fields: no SessionStore to check. Evict entries idle for >30 minutes.
		cutoff := time.Now().Add(-ephemeralSweepTTL).Unix()
		h.httpTemplates.Range(func(key, value any) bool {
			groupID := key.(string)
			entry := value.(*httpTemplateCacheEntry)
			if entry.lastAccessed.Load() < cutoff {
				h.httpTemplates.Delete(groupID)
				h.httpLastPaths.Delete(groupID)
				sweptSessions[groupID] = struct{}{}
			}
			return true
		})
	} else {
		activeSessions := make(map[string]struct{})
		for _, groupID := range h.config.SessionStore.List(context.Background()) {
			activeSessions[groupID] = struct{}{}
		}

		h.httpTemplates.Range(func(key, value any) bool {
			groupID := key.(string)
			if _, active := activeSessions[groupID]; !active {
				h.httpTemplates.Delete(groupID)
				h.httpLastPaths.Delete(groupID)
				sweptSessions[groupID] = struct{}{}
			}
			return true
		})

		// Sweep orphaned httpLastPaths entries from GET-only sessions that
		// never created httpTemplates entries.
		h.httpLastPaths.Range(func(key, value any) bool {
			groupID := key.(string)
			if _, active := activeSessions[groupID]; !active {
				h.httpLastPaths.Delete(groupID)
				sweptSessions[groupID] = struct{}{}
			}
			return true
		})
	}

	if len(sweptSessions) > 0 {
		slog.Debug("Swept stale HTTP cache entries",
			slog.Int("sessions", len(sweptSessions)))
	}
}

// hasStaticsInTree recursively checks if a tree contains any statics.
// Used to determine if this is a full tree send or dynamics-only update.
func hasStaticsInTree(tree map[string]interface{}) bool {
	if tree == nil {
		return false
	}
	// Check for statics at current level
	if s, ok := tree["s"]; ok {
		if arr, ok := s.([]interface{}); ok && len(arr) > 0 {
			return true
		}
		if arr, ok := s.([]string); ok && len(arr) > 0 {
			return true
		}
	}
	// Recursively check nested objects
	for _, v := range tree {
		if nested, ok := v.(map[string]interface{}); ok {
			if hasStaticsInTree(nested) {
				return true
			}
		}
	}
	return false
}

// writeUnauthorized sends a 401 response, adding WWW-Authenticate if the
// authenticator implements ChallengeAuthenticator (e.g., BasicAuthenticator).
func (h *liveHandler) writeUnauthorized(w http.ResponseWriter) {
	if ca, ok := h.config.Authenticator.(ChallengeAuthenticator); ok {
		w.Header().Set("WWW-Authenticate", ca.WWWAuthenticate())
	}
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

// sendUpdate generates and sends a template update to a single connection.
// If messages is nil, no errors/flash will be included in the template.
func (h *liveHandler) sendUpdate(conn *session.Connection, data interface{}, messages map[string]string) error {
	// Use the connection's cloned template for independent tree diffing
	var buf bytes.Buffer

	// Type assert Template from interface{} to *Template
	tmpl, ok := conn.Template.(*Template)
	if !ok {
		return fmt.Errorf("invalid template type in connection")
	}

	// Generate update using the connection's template
	err := tmpl.ExecuteUpdates(&buf, data, messages)
	if err != nil {
		return fmt.Errorf("template update failed: %w", err)
	}

	// Parse tree from buffer
	var tree map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &tree); err != nil {
		return fmt.Errorf("failed to parse tree: %w", err)
	}

	// Wrap with metadata
	response := UpdateResponse{
		Tree: tree,
		Meta: &ResponseMetadata{
			Success: true,
			Errors:  nil,
		},
	}

	// Encode response
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Debug log wire format metrics
	includesStatics := hasStaticsInTree(tree)
	slog.Debug("sendUpdate",
		slog.String("component", "live_handler"),
		slog.Int("payload_bytes", len(responseBytes)),
		slog.Bool("includes_statics", includesStatics),
	)

	// Send using the connection's Send method (thread-safe)
	return conn.Send(WSTextMessage, responseBytes)
}

// handlePubSubMessage handles incoming pub/sub broadcast messages from other instances.
//
// This is called by the RedisBroadcaster subscriber when a message is received.
// It deserializes the payload and fans it out to relevant local connections.
func (h *liveHandler) handlePubSubMessage(msg *pubsub.BroadcastMessage) error {
	// Deserialize the payload
	var data interface{}
	if err := json.Unmarshal(msg.Payload, &data); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Fan out based on message scope
	switch msg.Scope {
	case pubsub.ScopeGlobal:
		// Broadcast to all local connections
		connections := h.registry.GetAll()
		for _, conn := range connections {
			if err := h.sendUpdate(conn, data, nil); err != nil {
				slog.Warn("Failed to send broadcast",
					slog.String("component", "pubsub_handler"),
					slog.String("scope", "global"),
					slog.Any("error", err))
			}
		}
		slog.Debug("Fanned out broadcast",
			slog.String("component", "pubsub_handler"),
			slog.String("scope", "global"),
			slog.Int("count", len(connections)))

	case pubsub.ScopeGroup:
		// Broadcast to all connections in the group
		connections := h.registry.GetByGroup(msg.GroupID)
		for _, conn := range connections {
			if err := h.sendUpdate(conn, data, nil); err != nil {
				slog.Warn("Failed to send broadcast",
					slog.String("component", "pubsub_handler"),
					slog.String("scope", "group"),
					slog.String("target_id", msg.GroupID),
					slog.Any("error", err))
			}
		}
		slog.Debug("Fanned out broadcast",
			slog.String("component", "pubsub_handler"),
			slog.String("scope", "group"),
			slog.String("target_id", msg.GroupID),
			slog.Int("count", len(connections)))

	case pubsub.ScopeUser:
		// Broadcast to all connections for the user
		connections := h.registry.GetByUser(msg.UserID)
		for _, conn := range connections {
			if err := h.sendUpdate(conn, data, nil); err != nil {
				slog.Warn("Failed to send broadcast",
					slog.String("component", "pubsub_handler"),
					slog.String("scope", "user"),
					slog.String("target_id", msg.UserID),
					slog.Any("error", err))
			}
		}
		slog.Debug("Fanned out broadcast",
			slog.String("component", "pubsub_handler"),
			slog.String("scope", "user"),
			slog.String("target_id", msg.UserID),
			slog.Int("count", len(connections)))

	default:
		return fmt.Errorf("unknown broadcast scope: %s", msg.Scope)
	}

	return nil
}

// handleGroupActionMessage handles incoming group action messages from other instances.
//
// This is called by the RedisBroadcaster when a group action message is received from
// a remote instance. It enqueues the action dispatch on all local connections in the
// target group via their DispatchChan, ensuring state mutations happen in each
// connection's event loop goroutine.
// Note: own-instance messages are already filtered by RedisBroadcaster.handleMessage
// (line ~434 in redis.go) before routing here. No InstanceID check needed.
func (h *liveHandler) handleGroupActionMessage(msg *pubsub.GroupActionMessage) error {
	connections := h.registry.GetByGroup(msg.GroupID)
	if len(connections) == 0 {
		slog.Debug("No local connections for group action",
			slog.String("component", "pubsub_handler"),
			slog.String("group_id", msg.GroupID),
			slog.String("action", msg.Action))
		return nil
	}

	for _, conn := range connections {
		conn.EnqueueDispatch(&session.DispatchRequest{
			Action: msg.Action,
			Data:   msg.Data,
		})
	}

	slog.Debug("Enqueued group action for local connections",
		slog.String("component", "pubsub_handler"),
		slog.String("group_id", msg.GroupID),
		slog.String("action", msg.Action),
		slog.Int("connection_count", len(connections)))

	return nil
}

// handleTopicActionMessage handles incoming topic action messages from other
// instances. Called by the RedisBroadcaster when a "topic_action" message is
// received from a remote instance: it resolves the concrete topic to the local
// deduped subscriber set (exact ∪ matching patterns) and enqueues the action on
// each connection's DispatchChan, so each receiver runs it against its OWN
// state via handleDispatchedAction (the reconciler guarantee, cross-instance).
//
// excludeConn is nil: the publisher's sender-exclusion (use case I) is applied
// on the ORIGINATING instance's local fan-out (dispatchToTopic); the sending
// connection does not exist on this receiving instance. Own-instance messages
// are already dropped by RedisBroadcaster.handleMessage (InstanceID filter)
// before routing here, so the publisher's own subscribers are not
// double-dispatched.
func (h *liveHandler) handleTopicActionMessage(msg *pubsub.GroupActionMessage) error {
	connections := h.registry.GetByTopicExcept(msg.Topic, nil, segmentMatch)
	if len(connections) == 0 {
		slog.Debug("No local connections for topic action",
			slog.String("component", "pubsub_handler"),
			slog.String("topic", msg.Topic),
			slog.String("action", msg.Action))
		return nil
	}

	for _, conn := range connections {
		conn.EnqueueDispatch(&session.DispatchRequest{
			Action: msg.Action,
			Data:   msg.Data,
			Kind:   session.KindAction,
		})
	}

	slog.Debug("Enqueued topic action for local connections",
		slog.String("component", "pubsub_handler"),
		slog.String("topic", msg.Topic),
		slog.String("action", msg.Action),
		slog.Int("connection_count", len(connections)))

	return nil
}

// handleServerActionMessage handles incoming server action messages from other instances.
//
// This is called by the RedisBroadcaster subscriber when a server action message is received.
// It dispatches to the matching controller method and sends updates to local connections for the target user.
func (h *liveHandler) handleServerActionMessage(msg *pubsub.ServerActionMessage) error {
	// Get all connections for this user
	connections := h.registry.GetByUser(msg.UserID)
	if len(connections) == 0 {
		slog.Debug("No local connections for server action",
			slog.String("component", "pubsub_handler"),
			slog.String("user_id", msg.UserID),
			slog.String("action", msg.Action))
		return nil
	}

	slog.Debug("Handling server action",
		slog.String("component", "pubsub_handler"),
		slog.String("user_id", msg.UserID),
		slog.String("action", msg.Action),
		slog.Int("connection_count", len(connections)))

	// Process action for each connection.
	// Track last successful state per groupID for deduped persistence.
	var errCount int
	groupStates := make(map[string]interface{})
	for _, conn := range connections {
		// Create connection state for this action
		state := &connState{
			state:    conn.Stores, // conn.Stores holds the typed state
			messages: make(map[string]string),
			groupID:  conn.GroupID,
		}

		// Create context with timeout for server-initiated actions
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		// Create Context for action dispatch.
		//
		// Asymmetry note: this actionCtx has a live Session attached, so
		// a server-action handler can chain session.TriggerAction(...) to
		// re-enqueue more dispatches. The pendingTopicPublishes() drop below
		// only catches ctx.Publish calls — it does not intercept chained
		// TriggerAction calls, which bypass the publish queue entirely and
		// go straight to EnqueueDispatch. In practice this is not a footgun
		// because chained TriggerAction requires explicit caller intent and
		// each hop still runs through the per-connection event loop (no
		// unbounded recursion on a single goroutine), but handlers that
		// recursively trigger themselves will loop until the session
		// disconnects. The dispatched-flagged session emits an observability
		// log on chained TriggerAction (#337).
		actionCtx := NewContext(ctx, msg.Action, msg.Data)
		actionCtx = actionCtx.WithUserID(msg.UserID)
		actionCtx = actionCtx.WithGroupID(conn.GroupID)
		actionCtx = actionCtx.WithTopicSubscriber(h.topicSubscriberFor(conn, nil))
		actionCtx = actionCtx.WithFlashSetter(state)
		actionCtx = actionCtx.WithSession(newLocalSessionFromDispatched(h, conn.GroupID))

		// Dispatch action using Controller+State pattern
		newState, actionErr := DispatchWithState(h.config.Controller, state.state, actionCtx)
		cancel()

		if actionErr != nil {
			// Handle errors
			switch e := actionErr.(type) {
			case FieldError:
				state.setError(e.Field, e.Message)
			case MultiError:
				for _, fieldErr := range e {
					state.setError(fieldErr.Field, fieldErr.Message)
				}
			default:
				if !errors.Is(actionErr, ErrMethodNotFound) {
					slog.Warn("Action dispatch failed",
						slog.String("component", "pubsub_handler"),
						slog.String("user_id", msg.UserID),
						slog.String("action", msg.Action),
						slog.Any("error", actionErr))
					errCount++
					continue
				}
			}
		} else {
			state.state = newState
			conn.Stores = newState
			groupStates[conn.GroupID] = newState
		}

		// Chained Publish calls from server-initiated actions are
		// intentionally not processed. Server actions already fan out to all
		// the user's connections via this loop, and chaining Publish on top
		// would cause duplicate fan-out plus storm risk — matching the
		// behavior of handleDispatchedAction above. Log them so the failure
		// is observable instead of silent.
		if dropped := actionCtx.pendingTopicPublishes(); len(dropped) > 0 {
			slog.Error("Publish calls inside a server-initiated action are ignored (prevents fan-out amplification and broadcast storms)",
				slog.String("component", "pubsub_handler"),
				slog.String("action", msg.Action),
				slog.String("user_id", msg.UserID),
				slog.Int("dropped_count", len(dropped)))
		}

		// Send update to this connection (with flash messages)
		if err := h.sendUpdate(conn, state.state, state.getMessages()); err != nil {
			slog.Warn("sendUpdate failed for server action",
				slog.String("component", "pubsub_handler"),
				slog.String("user_id", msg.UserID),
				slog.String("action", msg.Action),
				slog.Any("error", err))
			errCount++
			continue
		}
	}

	// Persist once per distinct groupID (avoids N writes for multi-tab users
	// sharing a groupID, while correctly handling multi-device users with
	// different groupIDs).
	successCount := len(connections) - errCount
	if len(groupStates) > 0 && len(groupStates) < successCount {
		slog.Debug("Server action: multi-tab state deduped for persistence",
			slog.String("component", "pubsub_handler"),
			slog.String("action", msg.Action),
			slog.Int("connections", successCount),
			slog.Int("groups_persisted", len(groupStates)))
	}
	for gid, st := range groupStates {
		h.persistState(context.Background(), gid, st)
	}

	if errCount > 0 {
		slog.Warn("Server action failed for some connections",
			slog.String("component", "pubsub_handler"),
			slog.String("user_id", msg.UserID),
			slog.String("action", msg.Action),
			slog.Int("errors", errCount),
			slog.Int("total", len(connections)))
	} else {
		slog.Debug("Server action completed successfully",
			slog.String("component", "pubsub_handler"),
			slog.String("user_id", msg.UserID),
			slog.String("action", msg.Action),
			slog.Int("connection_count", len(connections)))
	}

	return nil
}

// Shutdown gracefully shuts down the LiveHandler.
//
// It stops accepting new WebSocket connections, sends close frames to all
// active connections, and waits for connections to finish (respecting ctx timeout).
//
// This method can be called multiple times safely (only first call has effect).
func (h *liveHandler) Shutdown(ctx context.Context) error {
	var shutdownErr error

	h.shutdownOnce.Do(func() {
		slog.Info("Starting graceful shutdown",
			slog.String("component", "live_handler"))

		// Mark as shutting down (stops accepting new connections)
		h.isShutdown.Store(true)
		close(h.shutdownChan)

		// Get all active connections
		connections := h.registry.GetAll()
		slog.Info("Closing active connections",
			slog.String("component", "live_handler"),
			slog.Int("count", len(connections)))

		// Send close frames to all connections
		closeMessage := WSFormatCloseMessage(WSCloseGoingAway, "Server shutting down")
		for _, conn := range connections {
			// Send close frame (best effort, ignore errors)
			if conn.Conn != nil {
				_ = conn.Send(WSCloseMessage, closeMessage)
			}
		}

		// Wait for all connection goroutines to finish
		done := make(chan struct{})
		go func() {
			h.shutdownWg.Wait()
			close(done)
		}()

		// Wait for shutdown or timeout
		select {
		case <-done:
			slog.Info("All connections closed gracefully",
				slog.String("component", "live_handler"))
		case <-ctx.Done():
			slog.Warn("Shutdown timeout reached, forcing close of remaining connections",
				slog.String("component", "live_handler"))
			shutdownErr = ctx.Err()

			// Force close remaining connections
			for _, conn := range h.registry.GetAll() {
				if conn.Conn != nil {
					if err := conn.Conn.Close(); err != nil {
						slog.Warn("Failed to force close connection",
							slog.String("component", "live_handler"),
							slog.Any("error", err))
					}
				}
			}
		}

		slog.Info("Shutdown complete",
			slog.String("component", "live_handler"))
	})

	return shutdownErr
}

// MetricsHandler returns an HTTP handler that exports Prometheus metrics.
func (h *liveHandler) MetricsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only allow GET requests
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Set content type for Prometheus text format
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		// Write metrics
		if err := h.metricsExporter.WriteMetrics(w); err != nil {
			slog.Error("Error writing metrics",
				slog.String("component", "live_handler"),
				slog.Any("error", err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	})
}

// handleUploadAction routes upload-related WebSocket actions to appropriate handlers.
// Returns (handled=true, err) if this was an upload action, (handled=false, nil) otherwise.
func (h *liveHandler) handleUploadAction(ctx context.Context, rawData []byte, msg message, state *connState, uploadRegistry uploadRegistry, connection *session.Connection) (bool, error) {
	switch msg.Action {
	case "upload_start", "upload_chunk", "upload_complete", "cancel_upload":
		// handled below
	default:
		return false, nil // Not an upload action
	}

	// No blanket temp-manager requirement: only Volume mode stages to disk, and
	// its handshake branch guards a nil manager. Direct/Proxied/Preview need no
	// local filesystem, so their handshake/complete actions work without one.

	switch msg.Action {
	case "upload_start":
		return true, h.handleUploadStart(rawData, state, uploadRegistry, connection)
	case "upload_chunk":
		return true, h.handleUploadChunk(rawData, uploadRegistry, connection)
	case "upload_complete":
		return true, h.handleUploadComplete(ctx, rawData, state, uploadRegistry, connection)
	default: // cancel_upload — gate switch above guarantees only upload actions reach here
		return true, h.handleCancelUpload(rawData, uploadRegistry, connection)
	}
}

// hasProxiedUpload reports whether any configured upload field uses Proxied mode.
func (h *liveHandler) hasProxiedUpload() bool {
	for _, c := range h.config.UploadConfigs {
		if c.Mode == uploadtypes.UploadModeProxied {
			return true
		}
	}
	return false
}

// streamProxiedPart streams one Proxied file part straight to Controller.OnUpload
// with zero local-disk staging, then records the result as a completed upload
// entry (ExternalRef set by the handler via SetResult) so the follow-on action
// reads it via ctx.GetCompletedUploads. Accept is validated from the header
// before any bytes are read; MaxFileSize is enforced mid-stream by LimitGuard.
func (h *liveHandler) streamProxiedPart(part *multipart.Part, uploadRegistry uploadRegistry, ctx *Context) error {
	field := part.FormName()
	cfg := h.config.UploadConfigs[field]

	if err := upload.ValidateFileHeader(part.FileName(), part.Header.Get("Content-Type"), cfg); err != nil {
		return err
	}

	streamer, ok := h.config.Controller.(UploadStreamer)
	if !ok {
		return &upload.ValidationError{Field: field, Message: "controller does not implement OnUpload for streaming uploads"}
	}

	entryID, err := upload.GenerateEntryID()
	if err != nil {
		return fmt.Errorf("failed to generate entry ID: %w", err)
	}

	entry := &uploadtypes.UploadEntry{
		ID:         entryID,
		ClientName: part.FileName(),
		ClientType: part.Header.Get("Content-Type"),
		ClientSize: -1,
	}

	guard := upload.NewLimitGuard(part, cfg.MaxFileSize)
	up := &UploadPart{
		Reader:     guard,
		Field:      field,
		Filename:   entry.ClientName,
		ClientType: entry.ClientType,
		ClientSize: -1,
		entry:      entry,
	}

	if err := streamer.OnUpload(up, ctx); err != nil {
		return err
	}

	entry.ClientSize = guard.Count()
	entry.Valid = true
	entry.Done = true
	entry.Progress = 100

	if reg, ok := uploadRegistry.(*upload.Registry); ok {
		if u := reg.GetUpload(field); u != nil {
			if upl, ok := u.(*upload.Upload); ok {
				if addErr := upl.AddEntry(entry); addErr != nil {
					return &upload.ValidationError{Field: field, Message: addErr.Error()}
				}
			}
		}
	}
	return nil
}

// handleUploadStart processes upload_start action from WebSocket client.
// Client sends file metadata, server creates upload entries and responds with entry IDs.
func (h *liveHandler) handleUploadStart(rawData []byte, state *connState, uploadRegistry uploadRegistry, connection *session.Connection) error {
	response, err := h.buildUploadStartResponse(rawData, state.groupID, uploadRegistry)
	if err != nil {
		return err
	}

	// Send UploadStartResponse to client so it can create upload entries
	responseData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal upload_start response: %w", err)
	}

	if err := connection.Send(WSTextMessage, responseData); err != nil {
		return fmt.Errorf("failed to send upload_start response: %w", err)
	}

	// Note: We don't send a tree update here because upload entries are created
	// client-side based on the UploadStartResponse. Tree updates happen after
	// upload completion when store data actually changes.

	return nil
}

// buildUploadStartResponse validates the requested files and produces the
// per-entry handshake reply (mode, validation, presigned meta) WITHOUT sending
// it. Both the WebSocket handler and the HTTP fallback (used when the socket is
// disabled) call this and transmit the result over their own transport.
func (h *liveHandler) buildUploadStartResponse(rawData []byte, sessionID string, uploadRegistry uploadRegistry) (*upload.UploadStartResponse, error) {
	// Parse upload_start message from raw transport data
	startMsg, err := upload.ParseUploadStartMessage(rawData)
	if err != nil {
		return nil, fmt.Errorf("invalid upload_start message: %w", err)
	}

	// Type assert to get concrete Registry type
	registry, ok := uploadRegistry.(*upload.Registry)
	if !ok {
		return nil, fmt.Errorf("invalid upload registry type")
	}

	// Get upload configuration
	uploadObj := registry.GetUpload(startMsg.UploadName)
	if uploadObj == nil {
		return nil, fmt.Errorf("upload %q not configured", startMsg.UploadName)
	}

	uploadInstance, ok := uploadObj.(*upload.Upload)
	if !ok {
		return nil, fmt.Errorf("invalid upload object type")
	}

	// Validate file count
	if err := upload.ValidateCount(len(startMsg.Files), uploadInstance.Config); err != nil {
		return nil, fmt.Errorf("file count validation failed: %w", err)
	}

	// Create upload entries for each file
	response := &upload.UploadStartResponse{
		UploadName: startMsg.UploadName,
		Entries:    make([]upload.UploadEntryInfo, 0, len(startMsg.Files)),
	}

	autoUpload := uploadInstance.Config.AutoUpload
	mode := uploadInstance.Config.Mode

	for _, fileMeta := range startMsg.Files {
		// Generate entry ID
		entryID, err := upload.GenerateEntryID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate entry ID: %w", err)
		}

		// Create upload entry with metadata
		entry := &uploadtypes.UploadEntry{
			ID:         entryID,
			ClientName: fileMeta.Name,
			ClientType: fileMeta.Type,
			ClientSize: fileMeta.Size,
			Progress:   0,
			Done:       false,
			Valid:      false, // Will be validated when entry is added
			BytesRecv:  0,
		}

		entryInfo := upload.UploadEntryInfo{
			EntryID:    entryID,
			ClientName: fileMeta.Name,
			AutoUpload: autoUpload,
		}

		switch mode {
		case uploadtypes.UploadModeDirect:
			// Direct: presign so the browser PUTs straight to cloud storage.
			presignMeta, err := uploadInstance.Config.External.Presign(entry)
			if err != nil {
				entryInfo.Error = fmt.Sprintf("failed to presign: %v", err)
			} else {
				entry.ExternalRef = presignMeta.URL
				if err := uploadInstance.AddEntry(entry); err != nil {
					entryInfo.Error = err.Error()
				} else {
					entryInfo.Valid = true
					entryInfo.External = &upload.ExternalUploadMeta{
						Uploader: presignMeta.Uploader,
						URL:      presignMeta.URL,
						Fields:   presignMeta.Fields,
						Headers:  presignMeta.Headers,
					}
				}
			}

		case uploadtypes.UploadModeProxied, uploadtypes.UploadModePreview:
			// Proxied bytes arrive via a follow-on HTTP multipart POST; Preview
			// bytes never leave the device. Neither stages to disk here — the
			// handshake only validates metadata so the client can dispatch.
			if err := upload.ValidateEntry(entry, uploadInstance.Config); err != nil {
				entryInfo.Error = err.Error()
			} else {
				entryInfo.Valid = true
			}

		default: // UploadModeVolume
			// Create the staging file the chunk handler appends to. With Dir set
			// the file is retained there (the app owns its lifecycle); otherwise
			// it stages under the session temp dir and is cleaned on disconnect.
			var tempPath string
			var err error
			if dir := uploadInstance.Config.Dir; dir != "" {
				tempPath, err = upload.CreateRetainedFile(dir, startMsg.UploadName, entryID)
			} else if tfm, ok := h.tempFileManager.(*upload.TempFileManager); ok && tfm != nil {
				tempPath, err = tfm.CreateTempFile(sessionID, startMsg.UploadName, entryID)
			} else {
				entryInfo.Error = "uploads unavailable: temp file manager not initialized"
				break
			}
			if err != nil {
				entryInfo.Error = fmt.Sprintf("failed to create upload file: %v", err)
			} else {
				entry.TempPath = tempPath
				if err := uploadInstance.AddEntry(entry); err != nil {
					entryInfo.Error = err.Error()
					if rmErr := os.Remove(tempPath); rmErr != nil {
						slog.Warn("Failed to remove upload file",
							slog.String("component", "upload_handler"),
							slog.String("path", tempPath),
							slog.Any("error", rmErr))
					}
				} else {
					entryInfo.Valid = true
				}
			}
		}

		entryInfo.Mode = mode.String()
		response.Entries = append(response.Entries, entryInfo)
	}

	return response, nil
}

// handleUploadChunk processes upload_chunk action from WebSocket client.
// Client sends base64-encoded chunk data, server decodes and appends to temp file.
func (h *liveHandler) handleUploadChunk(rawData []byte, uploadRegistry uploadRegistry, connection *session.Connection) error {
	// Parse upload_chunk message from raw WebSocket data
	chunkMsg, err := upload.ParseUploadChunkMessage(rawData)
	if err != nil {
		return fmt.Errorf("invalid upload_chunk message: %w", err)
	}

	// Type assert to get concrete Registry type
	registry, ok := uploadRegistry.(*upload.Registry)
	if !ok {
		return fmt.Errorf("invalid upload registry type")
	}

	// Find which upload this entry belongs to
	var targetUpload *upload.Upload
	for _, uploadObj := range registry.GetAllUploads() {
		if uploadObj.GetEntry(chunkMsg.EntryID) != nil {
			targetUpload = uploadObj
			break
		}
	}

	if targetUpload == nil {
		return fmt.Errorf("entry %q not found in any upload", chunkMsg.EntryID)
	}

	// Get the entry
	entry := targetUpload.GetEntry(chunkMsg.EntryID)
	if entry == nil {
		return fmt.Errorf("entry %q not found", chunkMsg.EntryID)
	}

	// Check if entry is valid
	if !entry.Valid {
		return fmt.Errorf("entry %q is invalid: %s", chunkMsg.EntryID, entry.Error)
	}

	// Decode base64 chunk
	chunkData, err := base64.StdEncoding.DecodeString(chunkMsg.ChunkBase64)
	if err != nil {
		return fmt.Errorf("failed to decode chunk: %w", err)
	}

	// Open temp file and append chunk
	tempFile, err := os.OpenFile(entry.TempPath, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open temp file: %w", err)
	}

	// Write chunk
	written, err := tempFile.Write(chunkData)
	if err != nil {
		if closeErr := tempFile.Close(); closeErr != nil {
			return fmt.Errorf("failed to write chunk: %w (close error: %v)", err, closeErr)
		}
		return fmt.Errorf("failed to write chunk: %w", err)
	}

	// Close file and check for errors (buffered writes may fail here)
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Update entry progress
	err = targetUpload.UpdateEntry(chunkMsg.EntryID, func(e *uploadtypes.UploadEntry) {
		e.BytesRecv += int64(written)
		if e.ClientSize > 0 {
			e.Progress = int((e.BytesRecv * 100) / e.ClientSize)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to update entry: %w", err)
	}

	// Broadcast progress update to this connection only
	progressMsg := &upload.UploadProgressMessage{
		Type:       "upload_progress",
		UploadName: targetUpload.Name,
		EntryID:    entry.ID,
		ClientName: entry.ClientName,
		Progress:   entry.Progress,
		BytesRecv:  entry.BytesRecv,
		BytesTotal: entry.ClientSize,
	}

	progressBytes, err := upload.SerializeUploadProgressMessage(progressMsg)
	if err != nil {
		slog.Warn("Failed to serialize progress message",
			slog.String("component", "upload_handler"),
			slog.Any("error", err))
		return nil // Don't fail chunk processing due to progress message error
	}

	if err := writeUpdateWebSocket(connection, progressBytes); err != nil {
		slog.Warn("Failed to send progress update",
			slog.String("component", "upload_handler"),
			slog.Any("error", err))
		// Don't fail - progress updates are best-effort
	}

	return nil
}

// handleUploadComplete processes upload_complete action from WebSocket client.
// Client indicates all chunks sent, server marks entries as done and calls ConsumeUpload.
func (h *liveHandler) handleUploadComplete(ctx context.Context, rawData []byte, state *connState, uploadRegistry uploadRegistry, connection *session.Connection) error {
	// Parse upload_complete message from raw WebSocket data
	completeMsg, err := upload.ParseUploadCompleteMessage(rawData)
	if err != nil {
		return fmt.Errorf("invalid upload_complete message: %w", err)
	}

	// Type assert to get concrete Registry type
	registry, ok := uploadRegistry.(*upload.Registry)
	if !ok {
		return fmt.Errorf("invalid upload registry type")
	}

	// Get upload object
	uploadObj := registry.GetUpload(completeMsg.UploadName)
	if uploadObj == nil {
		return fmt.Errorf("upload %q not found", completeMsg.UploadName)
	}

	uploadInstance, ok := uploadObj.(*upload.Upload)
	if !ok {
		return fmt.Errorf("invalid upload object type")
	}

	// Mark all entries as done
	for _, entryID := range completeMsg.EntryIDs {
		err := uploadInstance.UpdateEntry(entryID, func(e *uploadtypes.UploadEntry) {
			e.Done = true
			e.Progress = 100
		})
		if err != nil {
			slog.Warn("Failed to mark entry as done",
				slog.String("component", "upload_handler"),
				slog.String("entry_id", entryID),
				slog.Any("error", err))
		}
	}

	// Get completed valid entries
	completedEntries := uploadInstance.GetCompletedEntries()

	// Prepare response
	response := &upload.UploadCompleteResponse{
		UploadName: completeMsg.UploadName,
		Success:    true,
		Error:      "",
	}

	// Hybrid approach: Trigger action with special upload action name
	// Controllers can handle "upload_<field>_complete" action
	if len(completedEntries) > 0 {
		uploadAction := fmt.Sprintf("upload_%s_complete", completeMsg.UploadName)

		// Create Context for action dispatch
		actionCtx := NewContext(ctx, uploadAction, make(map[string]interface{}))
		actionCtx = actionCtx.WithUploads(uploadRegistry)
		actionCtx = actionCtx.WithUserID(connection.UserID)
		actionCtx = actionCtx.WithGroupID(connection.GroupID)
		actionCtx = actionCtx.WithTopicSubscriber(h.topicSubscriberFor(connection, nil))
		actionCtx = actionCtx.WithFlashSetter(state)
		actionCtx = actionCtx.WithSession(newLocalSession(h, connection.GroupID))

		// Dispatch action using Controller+State pattern
		newState, actionErr := DispatchWithState(h.config.Controller, state.state, actionCtx)
		if actionErr != nil && !errors.Is(actionErr, ErrMethodNotFound) {
			slog.Warn("Upload action failed",
				slog.String("component", "upload_handler"),
				slog.String("action", uploadAction),
				slog.Any("error", actionErr))
			response.Success = false
			response.Error = actionErr.Error()
		} else if actionErr == nil {
			state.state = newState
			// Drain ctx.Publish from an upload-complete handler — consistent
			// with the WS-action and HTTP-POST action paths.
			h.processTopicPublishes(connection, actionCtx.pendingTopicPublishes())
		}
	}

	// Send tree update to current connection to show upload completion immediately
	// This replaces the old upload_complete response to avoid duplicate messages
	if err := h.sendUpdate(connection, state.state, state.getMessages()); err != nil {
		slog.Warn("Failed to send tree update after upload",
			slog.String("component", "upload_handler"),
			slog.Any("error", err))
		return nil // Don't fail the upload, just skip the update
	}

	return nil
}

// handleCancelUpload processes cancel_upload action from WebSocket client.
// Client cancels an upload, server cleans up temp file and removes entry.
func (h *liveHandler) handleCancelUpload(rawData []byte, uploadRegistry uploadRegistry, connection *session.Connection) error {
	// Parse cancel_upload message from raw WebSocket data
	cancelMsg, err := upload.ParseCancelUploadMessage(rawData)
	if err != nil {
		return fmt.Errorf("invalid cancel_upload message: %w", err)
	}

	// Type assert to get concrete Registry type
	registry, ok := uploadRegistry.(*upload.Registry)
	if !ok {
		return fmt.Errorf("invalid upload registry type")
	}

	// Find which upload this entry belongs to
	var targetUpload *upload.Upload
	for _, uploadObj := range registry.GetAllUploads() {
		if uploadObj.GetEntry(cancelMsg.EntryID) != nil {
			targetUpload = uploadObj
			break
		}
	}

	response := &upload.CancelUploadResponse{
		EntryID: cancelMsg.EntryID,
		Success: true,
	}

	if targetUpload == nil {
		// Entry not found - might have already been removed
		slog.Warn("Entry not found for cancellation",
			slog.String("component", "upload_handler"),
			slog.String("entry_id", cancelMsg.EntryID))
	} else {
		// Get entry to find temp file path
		entry := targetUpload.GetEntry(cancelMsg.EntryID)
		if entry != nil && entry.TempPath != "" {
			// Remove temp file directly
			if err := os.Remove(entry.TempPath); err != nil {
				slog.Warn("Failed to remove temp file for entry",
					slog.String("component", "upload_handler"),
					slog.String("entry_id", cancelMsg.EntryID),
					slog.Any("error", err))
			}
		}

		// Remove entry from registry
		targetUpload.RemoveEntry(cancelMsg.EntryID)
	}

	// Serialize and send response
	respBytes, err := upload.SerializeCancelUploadResponse(response)
	if err != nil {
		return fmt.Errorf("failed to serialize response: %w", err)
	}

	if err := writeUpdateWebSocket(connection, respBytes); err != nil {
		return fmt.Errorf("failed to send cancel_upload response: %w", err)
	}

	return nil
}
