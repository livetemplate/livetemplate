package livetemplate

import (
	"fmt"
	"strings"
)

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
