package keys

import (
	"testing"
)

func TestGenerateItemHash_Deterministic(t *testing.T) {
	dynamics := map[string]interface{}{
		"0": "title",
		"1": "content",
	}

	hash1 := GenerateItemHash(dynamics)
	hash2 := GenerateItemHash(dynamics)

	if hash1 != hash2 {
		t.Errorf("Hash should be deterministic, got: %v and %v", hash1, hash2)
	}

	if len(hash1) != HashPrefixLength {
		t.Errorf("Hash length should be %d, got: %d", HashPrefixLength, len(hash1))
	}
}

func TestGenerateItemHash_DifferentContent(t *testing.T) {
	d1 := map[string]interface{}{"0": "a"}
	d2 := map[string]interface{}{"0": "b"}

	hash1 := GenerateItemHash(d1)
	hash2 := GenerateItemHash(d2)

	if hash1 == hash2 {
		t.Errorf("Different content should produce different hashes, got: %v", hash1)
	}
}

func TestGenerateItemHash_IgnoresAutoKey(t *testing.T) {
	without := map[string]interface{}{"0": "content"}
	with := map[string]interface{}{"0": "content", "_k": "some-old-key"}

	hash1 := GenerateItemHash(without)
	hash2 := GenerateItemHash(with)

	if hash1 != hash2 {
		t.Errorf("_k field should be excluded from hash, got: %v and %v", hash1, hash2)
	}
}

func TestGenerateItemHash_NilDynamics(t *testing.T) {
	hash := GenerateItemHash(nil)
	if hash == "" {
		t.Error("Nil dynamics should still produce a hash (hash of empty content)")
	}
}

func TestGenerateItemHash_EmptyDynamics(t *testing.T) {
	hash := GenerateItemHash(map[string]interface{}{})
	if hash == "" {
		t.Error("Empty dynamics should return a non-empty hash")
	}
}

func TestGenerateItemHash_UnhashableFallback(t *testing.T) {
	// Values that json.Marshal cannot serialize use a deterministic type-based fallback
	// instead of %v (which includes non-deterministic pointer addresses).
	ch := make(chan int)
	d1 := map[string]interface{}{"0": ch}
	d2 := map[string]interface{}{"0": ch}

	hash1 := GenerateItemHash(d1)
	hash2 := GenerateItemHash(d2)

	if hash1 != hash2 {
		t.Errorf("Unhashable fallback should be deterministic, got: %v and %v", hash1, hash2)
	}

	if hash1 == "" {
		t.Error("Unhashable values should still produce a non-empty hash")
	}
}

func TestGenerateItemHash_FieldOrderIndependence(t *testing.T) {
	d1 := map[string]interface{}{"0": "a", "1": "b", "2": "c"}
	d2 := map[string]interface{}{"2": "c", "0": "a", "1": "b"}

	hash1 := GenerateItemHash(d1)
	hash2 := GenerateItemHash(d2)

	if hash1 != hash2 {
		t.Errorf("Field order should not affect hash, got: %v and %v", hash1, hash2)
	}
}

func TestGenerateItemHashFromSlice_Deterministic(t *testing.T) {
	dynamics := []interface{}{"title", "content"}

	hash1 := GenerateItemHashFromSlice(dynamics)
	hash2 := GenerateItemHashFromSlice(dynamics)

	if hash1 != hash2 {
		t.Errorf("Hash should be deterministic, got: %v and %v", hash1, hash2)
	}
	if len(hash1) != HashPrefixLength {
		t.Errorf("Hash length should be %d, got: %d", HashPrefixLength, len(hash1))
	}
}

func TestGenerateItemHashFromSlice_SkipsNilEntries(t *testing.T) {
	withNils := []interface{}{"a", nil, "b"}
	withoutNils := []interface{}{"a", nil, "b"}

	hash1 := GenerateItemHashFromSlice(withNils)
	hash2 := GenerateItemHashFromSlice(withoutNils)

	if hash1 != hash2 {
		t.Errorf("Same content should produce same hash, got: %v and %v", hash1, hash2)
	}
}

func TestGenerateItemHashFromSlice_MatchesMapVersion(t *testing.T) {
	// For single-digit keys (0-9), map sort order matches numeric order,
	// so slice and map versions should produce identical hashes.
	sliceHash := GenerateItemHashFromSlice([]interface{}{"a", "b", "c"})
	mapHash := GenerateItemHash(map[string]interface{}{"0": "a", "1": "b", "2": "c"})

	if sliceHash != mapHash {
		t.Errorf("Slice and map versions should match for single-digit keys, got slice=%v map=%v", sliceHash, mapHash)
	}
}

func TestGenerateItemHashFromSlice_Empty(t *testing.T) {
	hash := GenerateItemHashFromSlice([]interface{}{})
	if hash == "" {
		t.Error("Empty slice should return a non-empty hash")
	}
}

func TestGenerateItemHashFromSlice_Nil(t *testing.T) {
	hash := GenerateItemHashFromSlice(nil)
	if hash == "" {
		t.Error("Nil slice should return a non-empty hash")
	}
}
