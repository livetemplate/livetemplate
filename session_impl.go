package livetemplate

import (
	"fmt"
	"log/slog"

	"github.com/livetemplate/livetemplate/internal/session"
	"github.com/livetemplate/livetemplate/pubsub"
)

// localSession is the concrete implementation of the Session interface.
// It dispatches server-initiated actions to every connection in a specific
// session group via the existing per-connection dispatch queue
// (EnqueueDispatch), the same mechanism used by ctx.BroadcastAction. A
// PubSubBroadcaster, when configured, is also notified so remote
// instances can deliver to their own local connections.
//
// Scope is per-session-group, not per-user. A session group corresponds
// to one browser session (all tabs share one group via cookie). Anonymous
// and authenticated flows both work, because every connection has a
// groupID assigned at mount time regardless of auth state.
type localSession struct {
	handler *liveHandler
	groupID string
	// fromDispatched is true when this localSession was constructed inside
	// a dispatched-action handler (i.e. the action context originated from
	// EnqueueDispatch, not from an HTTP request or initial WS connect).
	// When set, TriggerAction emits a slog.Debug line on each invocation so
	// that infinite chained-dispatch loops are visible in logs (#337).
	fromDispatched bool
}

// newLocalSession constructs a localSession scoped to a specific session
// group. The returned value implements the Session interface and is safe
// to store in a controller across goroutines.
func newLocalSession(handler *liveHandler, groupID string) *localSession {
	return &localSession{handler: handler, groupID: groupID}
}

// newLocalSessionFromDispatched constructs a localSession that knows it
// was created inside a dispatched-action handler. TriggerAction calls on
// the returned value emit an observability log line so that recursive or
// runaway chained dispatches are detectable (#337 Option A).
func newLocalSessionFromDispatched(handler *liveHandler, groupID string) *localSession {
	return &localSession{handler: handler, groupID: groupID, fromDispatched: true}
}

// TriggerAction dispatches a server-initiated action to every connection
// in this session's group.
//
// Behavior:
//   - Local delivery enqueues the action onto each local connection's
//     event loop via EnqueueDispatch. Each connection processes the
//     request serially, creating a fresh action context, running the
//     controller method, and sending the resulting diff over WebSocket.
//     This is the same machinery used by ctx.BroadcastAction.
//   - If a PubSubBroadcaster implementing GroupActionBroadcaster is
//     configured, the action is also published to Redis for remote
//     instances. The RedisBroadcaster filters messages from its own
//     instance ID, so local connections are not double-dispatched.
//
// Origin of the Session handle:
//   - A Session obtained via ctx.Session() inside OnConnect (the typical
//     path) targets the WebSocket connections in the current group.
//   - A Session obtained via ctx.Session() inside an HTTP POST action
//     handler is also valid: it identifies the session group of the
//     HTTP request (e.g. via cookie) and will fan out to any WebSocket
//     connections registered in that same group. Callers should be
//     aware that TriggerAction from an HTTP context may have no effect
//     if the same browser tab made the POST without an open WebSocket
//     in the same group — the fan-out target is "peer WebSocket
//     connections", not "the HTTP responder".
//
// Disconnect semantics:
//
//   - In single-instance mode, when the group has no local connections
//     and no GroupActionBroadcaster is configured, TriggerAction returns
//     an error. Background goroutines using the recommended cancellation
//     pattern (`if err := session.TriggerAction(...); err != nil { return }`)
//     will exit cleanly.
//
//   - In multi-instance mode (PubSub configured), TriggerAction returns
//     nil even with zero local connections, because the user may be
//     connected to another instance. Goroutines that need a hard lifetime
//     bound in this mode should implement their own termination condition
//     (a done channel, a context with cancellation, or a bounded
//     iteration count).
//
// Important asymmetry for multi-instance users: a persistent PubSub
// outage produces silent "pubsub publish failed" warnings (one per
// call) and TriggerAction keeps returning nil. Goroutines that rely
// on the error return as their only stop signal will loop forever in
// this scenario. If you care about goroutine cleanup under a broken
// PubSub, do not depend on the return value — use a context you
// control, or the hard iteration bound pattern:
//
//	for i := 0; i < 100; i++ {
//	    select {
//	    case <-ctx.Done():
//	        return
//	    case <-time.After(interval):
//	    }
//	    _ = session.TriggerAction("tick", nil)
//	}
func (s *localSession) TriggerAction(action string, data map[string]interface{}) error {
	// Callers obtain a Session via ctx.Session(), which returns a nil
	// interface when WithSession was never called (and panics at the
	// interface dispatch site, not here). The framework's wiring always
	// passes a fully-constructed *localSession to WithSession, so by the
	// time we enter this method, s and s.handler are guaranteed non-nil.
	// The only input-level validation that belongs here is action/group
	// emptiness — the framework can't cause those either, but they're
	// cheap to check and document the contract.
	if action == "" {
		return fmt.Errorf("livetemplate: action cannot be empty")
	}
	if s.groupID == "" {
		return fmt.Errorf("livetemplate: session has no groupID")
	}

	// Observability log for chained TriggerAction calls (#337 Option A).
	// Emitted before EnqueueDispatch so that an unbounded recursion (a
	// handler that re-triggers its own action) shows up in logs even if
	// the WebSocket disconnects before the loop terminates.
	if s.fromDispatched {
		slog.Debug("Session.TriggerAction called from within a dispatched action",
			slog.String("component", "local_session"),
			slog.String("group_id", s.groupID),
			slog.String("action", action),
		)
	}

	connections := s.handler.registry.GetByGroup(s.groupID)
	gab, hasRemote := s.handler.config.PubSubBroadcaster.(pubsub.GroupActionBroadcaster)

	if len(connections) == 0 && !hasRemote {
		return fmt.Errorf("livetemplate: no connected sessions for group %q", s.groupID)
	}

	// Defensive shallow copy of the caller's data map. Dispatch happens
	// asynchronously (via EnqueueDispatch and/or a Redis publish), so if
	// the caller reused or mutated the same map after TriggerAction
	// returned, the dispatched action handlers could observe torn reads
	// or a mutated value. A one-time copy at this boundary makes the
	// payload immutable from the framework's perspective.
	var payload map[string]interface{}
	if len(data) > 0 {
		payload = make(map[string]interface{}, len(data))
		for k, v := range data {
			payload[k] = v
		}
	}

	// Local fan-out: enqueue the dispatch onto each connection's event
	// loop. The connection's own goroutine dequeues, runs the controller
	// method, and sends the diff back over WebSocket.
	for _, conn := range connections {
		conn.EnqueueDispatch(&session.DispatchRequest{Action: action, Data: payload})
	}

	// Remote fan-out: publish to Redis for other instances. The local
	// instance filters its own messages, so local connections are not
	// double-dispatched.
	//
	// Partial-success handling: if PubSub publishing fails after local
	// enqueue has already happened, we log at Warn level and return nil
	// instead of propagating the error. Returning an error would
	// encourage callers to retry, which would double-enqueue the action
	// on local connections. Logging + returning nil preserves
	// at-least-once delivery for local connections while surfacing the
	// remote-publish failure in operator logs. The goroutine-cancellation
	// contract is unaffected: returning nil only happens when local
	// delivery succeeded.
	if hasRemote {
		if err := gab.PublishGroupAction(s.groupID, action, payload); err != nil {
			slog.Warn("livetemplate: Session.TriggerAction pubsub publish failed",
				slog.String("component", "local_session"),
				slog.String("group_id", s.groupID),
				slog.String("action", action),
				slog.Int("local_connections", len(connections)),
				slog.Any("error", err))
		}
	}

	return nil
}
