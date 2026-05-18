package livetemplate

import (
	"errors"
	"fmt"
	"strings"
)

// ErrTopicForbidden is the sentinel a denied ctx.Subscribe matches under
// errors.Is. Under the deny-all default (no WithTopicACL and no
// WithOpenTopics) every developer topic is denied; only ctx.SelfTopic() is
// ACL-exempt. Because the ACL runs eagerly even on an HTTP GET, an
// unconfigured ACL surfaces this on the initial page render, not the WS
// upgrade.
//
// ctx.Subscribe returns a *TopicForbiddenError (which carries the offending
// topic for the WS error envelope) — `errors.Is(err, ErrTopicForbidden)` still
// holds, so the canonical `if err := ctx.Subscribe(t); err != nil { … }`
// pattern is unaffected.
var ErrTopicForbidden = errors.New("livetemplate: topic subscription forbidden")

// ErrNoRequestContext is the Cause of a TopicForbiddenError when a developer
// topic Subscribe is attempted from a server-originated Context (dispatched /
// server-initiated / upload-complete — there is no HTTP request to authorize
// against). The ACL hook is not consulted (invoking it with a nil request
// would panic any reasonable hook); the subscribe is denied by default.
// ctx.SelfTopic() is ACL-exempt and unaffected. errors.Is(err,
// ErrNoRequestContext) distinguishes this from a hook-driven denial.
var ErrNoRequestContext = errors.New("livetemplate: topic ACL not consulted — server-originated context has no HTTP request")

// TopicForbiddenError is the concrete error ctx.Subscribe returns when the ACL
// denies a subscription. Topic carries the offending topic so the WS-connect
// path can emit the {"type":"error","code":"topic_forbidden","topic":…}
// envelope (proposal §3 wire-format note) the TS client surfaces as an
// lvt:error CustomEvent. Cause is the ACL hook's explanatory error, if any.
type TopicForbiddenError struct {
	Topic string
	Cause error
}

func (e *TopicForbiddenError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("livetemplate: subscription to topic %q forbidden: %v", e.Topic, e.Cause)
	}
	return fmt.Sprintf("livetemplate: subscription to topic %q forbidden", e.Topic)
}

// Is makes errors.Is(err, ErrTopicForbidden) hold for any *TopicForbiddenError,
// preserving the documented sentinel contract regardless of Topic/Cause.
func (e *TopicForbiddenError) Is(target error) bool { return target == ErrTopicForbidden }

// Unwrap exposes the ACL hook's explanatory error to errors.Is/As chains.
func (e *TopicForbiddenError) Unwrap() error { return e.Cause }

// topicReservedPrefix is the reserved namespace for identity-derived topics
// (SelfTopic / UserTopic). Developer topic names must never start with it; a
// connection may only subscribe to a reserved topic exactly equal to its own
// SelfTopic() (the anti-spoof rule).
const topicReservedPrefix = "lvt:"

// UserTopic returns the reserved topic that addresses every connection of the
// given authenticated user, across all of that user's devices and tabs
// regardless of session-group strategy. It is a pure string constructor usable
// out-of-band (webhook/cron) — e.g. handler.Publish(UserTopic("alice"), …).
//
// This is the only identity-topic constructor by design: there is deliberately
// no SessionTopic() and no all-users GlobalTopic() (proposal §5 / Appendix B —
// an ACL-exempt all-users primitive is the highest-blast-radius footgun).
func UserTopic(userID string) string {
	return topicReservedPrefix + "user:" + userID
}

// sessionTopic returns the reserved topic addressing every connection of one
// anonymous browser session (the SelfTopic() anonymous path). It is
// deliberately unexported: there is no public SessionTopic() constructor
// (proposal §5 / Appendix B — out-of-band addressing of an anonymous session
// by group id has no concrete use case). Kept here, beside UserTopic, so the
// reserved-namespace vocabulary lives in one place and the "session:" segment
// cannot drift from "user:".
func sessionTopic(groupID string) string {
	return topicReservedPrefix + "session:" + groupID
}

// isReservedTopic reports whether topic is in the reserved lvt: namespace. It
// is the classification half of the anti-spoof rule: a reserved-prefixed topic
// is admissible only by *exact* string equality to the connection's own
// SelfTopic(); every other lvt: string must be rejected (prefix-equality would
// let lvt:user:alice* capture lvt:user:alice2). The exact-equality check itself
// lives in the caller; this predicate only answers "is this reserved?".
func isReservedTopic(topic string) bool {
	return strings.HasPrefix(topic, topicReservedPrefix)
}

// validateDeveloperTopic enforces the segment grammar for developer (non-lvt:)
// topics. It is never applied to reserved lvt: topics (they legitimately
// contain ':', validated separately by the reserved-namespace rule).
//
// Grammar (proposal §2): a non-empty, "/"-separated sequence of segments; each
// segment is either a literal in [a-zA-Z0-9_-]+ or the single character "*".
// Any number of "*" segments is allowed: only this validator may ever be
// narrowed to a stricter wildcard subset — the matcher stays general-case. No
// empty segments (rejects "", leading/trailing/double "/"). ":" is excluded by
// the segment charset.
func validateDeveloperTopic(topic string) error {
	if topic == "" {
		return fmt.Errorf("invalid topic: empty")
	}
	if isReservedTopic(topic) {
		return fmt.Errorf("invalid topic %q: developer topics must not start with %q", topic, topicReservedPrefix)
	}
	for _, segment := range strings.Split(topic, "/") {
		if segment == "*" {
			continue
		}
		if segment == "" {
			return fmt.Errorf("invalid topic %q: empty segment (no leading/trailing/double %q)", topic, "/")
		}
		for i := 0; i < len(segment); i++ {
			if !isValidSegmentChar(segment[i]) {
				return fmt.Errorf("invalid topic %q: segment %q has illegal character %q (allowed: [a-zA-Z0-9_-] or whole-segment %q)", topic, segment, segment[i], "*")
			}
		}
	}
	return nil
}

// isValidSegmentChar's exclusions carry the non-obvious WHY: ":" keeps
// developer topics disjoint from the reserved lvt: namespace; "*" is rejected
// inside a segment because a wildcard is a whole segment ("ro*m" is invalid).
func isValidSegmentChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	case b == '_' || b == '-':
		return true
	default:
		return false
	}
}

// segmentMatch reports whether a subscription pattern matches a concrete
// ("*"-free) topic. Two load-bearing invariants (proposal §2); TestSegmentMatch
// is the exhaustive executable spec:
//   - Equal segment count is required — this is what bounds "*" to exactly one
//     segment, never zero, never across "/". It is a topic-isolation boundary.
//   - "*" matches one NON-EMPTY segment; guarded explicitly so the matcher is
//     total even if a degenerate concrete bypasses validateDeveloperTopic.
//
// Stays general-case (multi-"*"): any wildcard tightening belongs in
// validateDeveloperTopic, never here.
func segmentMatch(pattern, concrete string) bool {
	patternSegs := strings.Split(pattern, "/")
	concreteSegs := strings.Split(concrete, "/")
	if len(patternSegs) != len(concreteSegs) {
		// Unequal counts never match. This single check is what bounds "*" to
		// exactly one segment — never zero, never spanning "/".
		return false
	}
	for i, seg := range patternSegs {
		if seg == "*" {
			if concreteSegs[i] == "" {
				return false // "*" matches exactly one NON-empty segment
			}
			continue
		}
		if seg != concreteSegs[i] {
			return false
		}
	}
	return true
}
