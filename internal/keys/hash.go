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
//
// empty/nil input: an empty or nil dynamics slice produces FNV-1a's
// offset-basis constant (0xcbf29ce484222325) — a stable, non-zero,
// distinguishable hash for the "empty range item" case.
func ItemHashUint64(dynamics []interface{}) uint64 {
	content := strings.Join(buildHashParts(dynamics), "|")
	hasher := fnv.New64a()
	hasher.Write([]byte(content))
	return hasher.Sum64()
}

func formatHashPart(key string, val interface{}) string {
	buf := make([]byte, 0, len(key)+1+16)
	buf = append(buf, key...)
	buf = append(buf, ':')
	buf = appendJSONValue(buf, val)
	return string(buf)
}

// appendJSONValue appends a byte-equivalent encoding of val to buf, matching
// what encoding/json.Marshal would produce for every supported type. The fast
// paths cover the LargeTable hot types (string, int variants, bool); the
// fallback defers to json.Marshal so unknown types stay byte-stable.
//
// Wire-stability invariant: changing this function without preserving exact
// byte output for any type silently flips RangeStreamState.Hashes for that
// type, breaking stream-mode transition fingerprints across upgrades. The
// regression gate is TestFormatHashPart_ByteEquivalentToJSON.
func appendJSONValue(buf []byte, val interface{}) []byte {
	switch v := val.(type) {
	case string:
		return appendJSONString(buf, v)
	case int:
		return strconv.AppendInt(buf, int64(v), 10)
	case int64:
		return strconv.AppendInt(buf, v, 10)
	case int32:
		return strconv.AppendInt(buf, int64(v), 10)
	case int16:
		return strconv.AppendInt(buf, int64(v), 10)
	case int8:
		return strconv.AppendInt(buf, int64(v), 10)
	case uint:
		return strconv.AppendUint(buf, uint64(v), 10)
	case uint64:
		return strconv.AppendUint(buf, v, 10)
	case uint32:
		return strconv.AppendUint(buf, uint64(v), 10)
	case uint16:
		return strconv.AppendUint(buf, uint64(v), 10)
	case uint8:
		return strconv.AppendUint(buf, uint64(v), 10)
	case bool:
		if v {
			return append(buf, "true"...)
		}
		return append(buf, "false"...)
	}
	// Fallback: defer to json.Marshal (preserves byte-stability for slices,
	// maps, structs, floats, *TreeNode, and any unknown user type).
	jsonBytes, err := json.Marshal(val)
	if err != nil {
		return append(buf, fmt.Sprintf("<unhashable:%T>", val)...)
	}
	return append(buf, jsonBytes...)
}

// appendJSONString writes a JSON-encoded string to buf. Fast path for ASCII
// strings with no chars that json.Marshal would escape (~99% of LargeTable
// row content); falls back to json.Marshal for anything containing `"`, `\`,
// `<`, `>`, `&`, control bytes, or non-ASCII.
func appendJSONString(buf []byte, s string) []byte {
	if needsJSONEscape(s) {
		jsonBytes, _ := json.Marshal(s)
		return append(buf, jsonBytes...)
	}
	buf = append(buf, '"')
	buf = append(buf, s...)
	buf = append(buf, '"')
	return buf
}

// needsJSONEscape reports whether s contains any byte json.Marshal would
// escape with its default HTML-safe encoder. Conservative: any non-printable,
// any HTML-special, any non-ASCII triggers the fallback.
func needsJSONEscape(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		// Control chars (< 0x20), DEL (0x7F), or any non-ASCII (>= 0x80,
		// includes  /  which json escapes); the JSON-special
		// chars `"` and `\`; and the HTML-special chars `<` `>` `&` that
		// json.Marshal's default encoder escapes to < etc.
		if c < 0x20 || c == '"' || c == '\\' || c == '<' || c == '>' || c == '&' || c >= 0x7F {
			return true
		}
	}
	return false
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
