package keys

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
)

const (
	// HashPrefixLength is the number of characters to use from the generated hash
	// for compact item identifiers. 12 characters = 48 bits of entropy.
	HashPrefixLength = 12
)

// GenerateItemHash creates a stable hash for a range item based on its dynamic content.
// Uses FNV-1a hash for fast, non-cryptographic content fingerprinting.
// The "_k" key is excluded from hashing since it is the auto-generated key itself.
func GenerateItemHash(dynamics map[string]interface{}) string {
	dynKeys := make([]string, 0, len(dynamics))
	for k := range dynamics {
		if k != "_k" {
			dynKeys = append(dynKeys, k)
		}
	}
	sort.Strings(dynKeys)

	var parts []string
	for _, k := range dynKeys {
		val := dynamics[k]
		parts = append(parts, formatHashPart(k, val))
	}

	return hashParts(parts)
}

// GenerateItemHashFromSlice creates a stable hash from a dynamics slice.
// This avoids the overhead of converting []interface{} to map[string]interface{}
// when the dynamics are already in slice form. Nil entries are skipped.
func GenerateItemHashFromSlice(dynamics []interface{}) string {
	return hashParts(buildHashParts(dynamics))
}

// ItemHashUint64 hashes the given dynamics slice with FNV-1a 64-bit and returns
// the raw uint64. Mirrors GenerateItemHashFromSlice's value-formatting (same
// formatHashPart helper, same skip-nil rule), but returns the raw uint64
// instead of the 12-char hex prefix.
//
// nil-skip: nil entries are skipped — only non-nil entries' positions are
// encoded into the hash. A trailing-nil and a missing-trailing-position
// produce the same hash (consistent with the existing hashing rule).
//
// nil-vs-"" divergence: [nil, "x"] hashes to a different value than
// ["", "x"] because the empty-string entry IS included (formatted as
// `0:""`), while the nil entry is skipped entirely. A transition from
// "field set to empty" to "field omitted entirely" is correctly detected
// as a content change.
func ItemHashUint64(dynamics []interface{}) uint64 {
	content := strings.Join(buildHashParts(dynamics), "|")
	hasher := fnv.New64a()
	hasher.Write([]byte(content))
	return hasher.Sum64()
}

func formatHashPart(key string, val interface{}) string {
	valJSON, err := json.Marshal(val)
	if err != nil {
		return fmt.Sprintf("%s:<unhashable:%T>", key, val)
	}
	return fmt.Sprintf("%s:%s", key, string(valJSON))
}

func hashParts(parts []string) string {
	content := strings.Join(parts, "|")
	hasher := fnv.New64a()
	hasher.Write([]byte(content))
	hash := hex.EncodeToString(hasher.Sum(nil))

	if len(hash) >= HashPrefixLength {
		return hash[:HashPrefixLength]
	}
	return hash
}

// buildHashParts formats the dynamics slice into ordered hash parts (skipping
// nil entries). Shared by GenerateItemHashFromSlice and ItemHashUint64 so the
// two hash functions can never silently diverge in their input encoding.
func buildHashParts(dynamics []interface{}) []string {
	var parts []string
	for i, val := range dynamics {
		if val == nil {
			continue
		}
		parts = append(parts, formatHashPart(strconv.Itoa(i), val))
	}
	return parts
}
