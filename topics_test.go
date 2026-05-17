package livetemplate

import "testing"

func TestUserTopic(t *testing.T) {
	if got := UserTopic("alice"); got != "lvt:user:alice" {
		t.Errorf("UserTopic(alice) = %q, want lvt:user:alice", got)
	}
	if got := UserTopic(""); got != "lvt:user:" {
		t.Errorf("UserTopic(\"\") = %q, want lvt:user:", got)
	}
	if !isReservedTopic(UserTopic("bob")) {
		t.Errorf("UserTopic output must be in the reserved namespace")
	}
}

func TestIsReservedTopic(t *testing.T) {
	tests := []struct {
		topic string
		want  bool
	}{
		{"lvt:user:alice", true},
		{"lvt:session:abc123", true},
		{"lvt:", true},
		{"room/42", false},
		{"announcements", false},
		{"lvtx:foo", false}, // prefix is exactly "lvt:", not "lvt"
		{"", false},
	}
	for _, tt := range tests {
		if got := isReservedTopic(tt.topic); got != tt.want {
			t.Errorf("isReservedTopic(%q) = %v, want %v", tt.topic, got, tt.want)
		}
	}
}

func TestValidateDeveloperTopic(t *testing.T) {
	valid := []string{
		"room/42",
		"room/*",
		"a/*/b/*",
		"*/alice",
		"org/*/room/*",
		"public/feed",
		"a",
		"*",
		"foo-bar_baz/9",
	}
	for _, topic := range valid {
		if err := validateDeveloperTopic(topic); err != nil {
			t.Errorf("validateDeveloperTopic(%q) = %v, want nil", topic, err)
		}
	}

	invalid := []string{
		"",               // empty
		"lvt:user:alice", // reserved prefix — not a developer topic
		"room/",          // trailing slash → empty segment
		"/room",          // leading slash → empty segment
		"room//x",        // double slash → empty segment
		"ro*m",           // partial wildcard — "*" must be a whole segment
		"room/4*2",       // "*" mixed into a literal segment
		"room/4 2",       // space is not an allowed segment char
		"a:b",            // ":" is excluded (keeps disjoint from lvt:)
	}
	for _, topic := range invalid {
		if err := validateDeveloperTopic(topic); err == nil {
			t.Errorf("validateDeveloperTopic(%q) = nil, want error", topic)
		}
	}
}

// TestSegmentMatch is the executable specification for the segmentMatch
// learning-mode contribution. Every row encodes a rule from proposal §2
// ("Matcher" / "Grammar"). The implementation in topics.go must make all rows
// pass — do not change this table to fit an implementation; implement to fit
// this table.
func TestSegmentMatch(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		concrete string
		want     bool
	}{
		// Exact (all-literal pattern == segment-wise equality).
		{"exact equal", "room/42", "room/42", true},
		{"exact differ", "room/42", "room/43", false},
		{"exact single seg", "a", "a", true},
		{"exact single seg differ", "a", "b", false},

		// Single "*": matches exactly one non-empty segment, never across "/".
		{"star one segment", "*", "foo", true},
		{"star not across slash", "*", "a/b", false},
		{"star not empty", "*", "", false}, // "*" matches NON-empty only

		// Trailing "*".
		{"trailing star match", "room/*", "room/42", true},
		{"trailing star count mismatch deeper", "room/*", "room/42/log", false},
		{"trailing star count mismatch shallower", "room/*", "room", false},
		{"trailing star literal differ", "room/*", "hall/42", false},

		// Leading "*".
		{"leading star match", "*/alice", "room/alice", true},
		{"leading star literal differ", "*/alice", "room/bob", false},
		{"leading star count mismatch", "*/alice", "a/b/alice", false},
		{"leading star x match", "*/x", "a/x", true},
		{"leading star x count mismatch", "*/x", "a/b/x", false},
		{"leading star x literal differ", "*/x", "a/y", false},

		// Multi-segment "*" (general case — Phase 3 valve depends on this).
		{"multi star match", "a/*/b/*", "a/1/b/2", true},
		{"multi star count mismatch", "a/*/b/*", "a/1/b", false},
		{"multi star literal differ", "a/*/b/*", "a/1/c/2", false},
		{"org room match", "org/*/room/*", "org/9/room/42", true},
		{"org room literal differ", "org/*/room/*", "org/9/lobby/42", false},

		// "*" never matches an empty interior segment.
		{"mid star match", "a/*/c", "a/b/c", true},
		{"mid star empty segment", "a/*/c", "a//c", false},

		// All-"*" patterns.
		{"all star two", "*/*", "a/b", true},
		{"all star count low", "*/*", "a", false},
		{"all star count high", "*/*", "a/b/c", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := segmentMatch(tt.pattern, tt.concrete); got != tt.want {
				t.Errorf("segmentMatch(%q, %q) = %v, want %v", tt.pattern, tt.concrete, got, tt.want)
			}
		})
	}
}
