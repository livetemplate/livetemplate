package livetemplate

import "testing"

// A persist field that is nil when the state is saved comes back as JSON null.
// jsoniter decodes null into a zero-length json.RawMessage (encoding/json yields
// the literal "null"), so restoring used to fail on those empty bytes — and because
// InjectPersistFields returns on the first error, ONE nil field discarded the WHOLE
// restored state. Every other persist field silently reverted to its zero value on
// reconnect, which is the kind of loss a reviewer only notices after refreshing.
//
// Nil must round-trip to nil, and must not disturb its neighbours.
func TestInjectPersistFields_NilFieldDoesNotDiscardState(t *testing.T) {
	type persistState struct {
		RepoPath string          `json:"repo_path" lvt:"persist"`
		Expanded map[string]bool `json:"expanded"  lvt:"persist"`
		Tags     []string        `json:"tags"      lvt:"persist"`
	}

	s := AsState(&persistState{}).(*jsonState[persistState])

	// The state a fresh session saves: the map and slice are still nil.
	data, err := s.ExtractPersistFields(persistState{RepoPath: "/repo"})
	if err != nil {
		t.Fatalf("ExtractPersistFields: %v", err)
	}

	got, err := s.InjectPersistFields(data)
	if err != nil {
		t.Fatalf("a nil persist field must not fail the restore: %v", err)
	}

	restored := got.(persistState)
	if restored.RepoPath != "/repo" {
		t.Errorf("RepoPath = %q, want %q — the nil field took the rest of the state with it",
			restored.RepoPath, "/repo")
	}
	if restored.Expanded != nil {
		t.Errorf("Expanded = %v, want nil", restored.Expanded)
	}
	if restored.Tags != nil {
		t.Errorf("Tags = %v, want nil", restored.Tags)
	}

	// The other shape a null arrives in: a LITERAL 4-byte "null", which is what
	// encoding/json writes — so it is what any session stored before the move to
	// json-iterator holds. That one never reaches the skip above (its RawMessage is
	// not empty); it decodes, and must decode to nil rather than erroring.
	stored := []byte(`{"repo_path":"/repo","expanded":null,"tags":null}`)
	got, err = s.InjectPersistFields(stored)
	if err != nil {
		t.Fatalf("a literal \"null\" persist field must not fail the restore: %v", err)
	}
	restored = got.(persistState)
	if restored.RepoPath != "/repo" || restored.Expanded != nil || restored.Tags != nil {
		t.Errorf("restored = %+v, want RepoPath=/repo with nil Expanded/Tags", restored)
	}
}
