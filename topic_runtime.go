package livetemplate

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/livetemplate/livetemplate/internal/session"
)

// topicErrorEnvelope is the WS message emitted when a Subscribe is ACL-denied
// on the WS-connect path (proposal §3 wire-format note). It is a distinct
// envelope from UpdateResponse: the TS client (Phase 4) shape-tests
// `type === "error"` and surfaces it as an lvt:error CustomEvent; the stateless
// diff path is untouched.
type topicErrorEnvelope struct {
	Type  string `json:"type"`  // always "error"
	Code  string `json:"code"`  // "topic_forbidden"
	Topic string `json:"topic"` // the denied topic
}

// sendTopicForbiddenEnvelope best-effort emits the topic_forbidden error
// envelope to conn. It is queued on the connection's send buffer; writePump's
// drain-on-close flushes it even though the WS-connect Mount failure closes
// the connection immediately after. Phase 1 emits the envelope and keeps the
// existing close-on-Mount-error behavior; finalizing whether the socket should
// instead stay open is V14 / Phase 4's call (recorded in phase-1.md as a
// server-emitted-envelope note for the Phase 4 audit).
func (h *liveHandler) sendTopicForbiddenEnvelope(conn *session.Connection, topic string) {
	if conn == nil {
		return
	}
	payload, err := json.Marshal(topicErrorEnvelope{Type: "error", Code: "topic_forbidden", Topic: topic})
	if err != nil {
		return
	}
	if sendErr := conn.Send(WSTextMessage, payload); sendErr != nil {
		slog.Debug("failed to send topic_forbidden envelope",
			slog.String("component", "live_handler"),
			slog.String("topic", topic),
			slog.Any("error", sendErr))
	}
}

// liveTopicSubscriber is the handler-bound topicSubscriber the mount handler
// injects into every Context (mirrors localSession). conn is nil on the HTTP
// page-render / POST and server-originated paths (no long-lived WebSocket
// Connection) — registerTopic is then a no-op, but checkTopicACL has already
// run eagerly. r is the request that established this Context (the WS upgrade
// request on the WS path; a plain GET/POST on the HTTP path); it may be nil for
// server-originated contexts (dispatched/server-action/upload) — a developer
// topic Subscribe there is unusual, and a nil-tolerant ACL hook or the
// deny-all default still answers it (SelfTopic(), the common case, is
// ACL-exempt regardless).
type liveTopicSubscriber struct {
	h    *liveHandler
	conn *session.Connection
	r    *http.Request
}

// topicSubscriberFor builds the per-Context topic hook. Called at every
// Context-build site alongside WithUserID/WithGroupID.
func (h *liveHandler) topicSubscriberFor(conn *session.Connection, r *http.Request) topicSubscriber {
	return &liveTopicSubscriber{h: h, conn: conn, r: r}
}

// checkTopicACL resolves the three ACL configurations (proposal §3):
//   - WithOpenTopics(): every topic permitted.
//   - WithTopicACL(fn): the hook decides; (false, _) → ErrTopicForbidden, the
//     hook's explanatory error wrapped so errors.Is(err, ErrTopicForbidden)
//     still holds while the cause stays inspectable.
//   - neither: deny-all. The ACL is a topic's ONLY boundary (topics are
//     cross-user, not Authenticator-bounded), so defaulting it open would be a
//     footgun and would teach an insecure idiom via copy-pasted scaffolds.
func (s *liveTopicSubscriber) checkTopicACL(topic, userID string) error {
	cfg := s.h.config
	switch {
	case cfg.OpenTopics:
		return nil
	case cfg.TopicACL != nil:
		// Server-originated contexts (dispatched / server-initiated /
		// upload-complete) have no request. The spec's own canonical ACL
		// pattern dereferences r (r.Header.Get("Upgrade")), so invoking the
		// hook with a nil request would panic in a reasonable hook. A
		// server-originated developer-topic Subscribe is not a documented
		// Phase 1 use case (SelfTopic() is ACL-exempt and never reaches here);
		// deny honestly — there is no request to authorize against.
		if s.r == nil {
			return &TopicForbiddenError{Topic: topic}
		}
		allowed, err := cfg.TopicACL(topic, userID, s.r)
		if allowed {
			return nil
		}
		return &TopicForbiddenError{Topic: topic, Cause: err}
	default:
		return &TopicForbiddenError{Topic: topic}
	}
}

func (s *liveTopicSubscriber) registerTopic(topic string) {
	if s.conn == nil {
		return // HTTP GET/POST / server-originated: only materializes with a Connection
	}
	s.h.registry.SubscribeConnectionToTopic(s.conn, topic)
}

func (s *liveTopicSubscriber) unregisterTopic(topic string) {
	if s.conn == nil {
		return
	}
	s.h.registry.UnsubscribeConnectionFromTopic(s.conn, topic)
}

func (s *liveTopicSubscriber) isClientWiredAction(action string) bool {
	if s.h.config.Template == nil {
		return false
	}
	return s.h.config.Template.isClientWiredAction(action)
}
