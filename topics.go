package livetemplate

import (
	"fmt"
	"strings"
)

// topicReservedPrefix is the reserved namespace for identity-derived topics
// (SelfTopic / UserTopic). Developer topic names must never start with it; a
// connection may only subscribe to a reserved topic that is exactly its own
// SelfTopic() (the anti-spoof rule, wired in Phase 1's Subscribe).
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

// isReservedTopic reports whether topic is in the reserved lvt: namespace.
//
// Shared by the constructor (its output trivially satisfies this) and, in
// Phase 1, by Subscribe: a reserved-prefixed argument is admitted only by
// *exact* string equality to the caller's SelfTopic() — every other lvt:
// string is rejected (anti-spoof; prefix-equality would let lvt:user:alice*
// capture lvt:user:alice2). That exact `topic == ctx.SelfTopic()` comparison
// is a one-liner at the Phase-1 Subscribe call site; this predicate is the
// reusable classification half it branches on.
func isReservedTopic(topic string) bool {
	return strings.HasPrefix(topic, topicReservedPrefix)
}

// validateDeveloperTopic enforces the segment grammar for developer (non-lvt:)
// topics, used by Subscribe in Phase 1 (the reserved namespace is validated
// separately and first — this grammar is never applied to lvt: topics, which
// legitimately contain ':').
//
// Grammar (proposal §2): a non-empty, "/"-separated sequence of segments; each
// segment is either a literal in [a-zA-Z0-9_-]+ or the single character "*".
// Multiple "*" segments are allowed (general case — Phase 3's risk valve may
// later tighten *only this validator* to reject non-trailing/multiple "*",
// never the matcher). No empty segments (rejects "", leading/trailing/double
// "/"). ":" is excluded by the segment charset.
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

// isValidSegmentChar reports whether b is allowed in a literal topic segment:
// [a-zA-Z0-9_-]. Notably excludes ":" (keeps developer topics disjoint from
// the reserved lvt: namespace) and "*" (a wildcard is a whole segment, never a
// partial one — "ro*m" is invalid).
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
// (publishable, "*"-free) topic.
//
// CONTRACT (proposal §2 "Matcher" / "Grammar"):
//   - Split both arguments on "/" into segments.
//   - The pattern matches ONLY IF both have the SAME segment count. Unequal
//     counts never match — this is what makes "*" mean "exactly one segment,
//     never zero, never across '/'": e.g. "room/*" (2) matches "room/42" (2)
//     but NOT "room/42/log" (3) and NOT "room" (1).
//   - For equal counts, the pattern matches iff EVERY pattern segment either
//     is the literal "*" or is byte-equal to the concrete segment at the same
//     index. Multiple "*" segments are independent ("a/*/b/*", "*/alice").
//   - "*" matches exactly one NON-EMPTY segment: "*" must NOT match an empty
//     concrete segment (the upstream grammar validator already rejects empty
//     segments, but this matcher must be total and stay correct if a degenerate
//     concrete is ever passed directly — decide how defensive to be here).
//   - An all-literal pattern (no "*") is just exact equality, segment-wise.
//   - No regexp, no trie/radix — a flat O(P) segment scan, by design.
//
// This is the GENERAL-case matcher (multi-"*": "a/*/b/*", "*/alice"), never
// trailing-"*"-only — Phase 3's risk valve narrows only validateDeveloperTopic,
// never this function. Split-and-compare is chosen over a manual two-cursor
// walk: topics are short, so the two-slice allocation is negligible against the
// readability of an explicit positional scan in a topic-isolation boundary.
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
