package keys

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
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
		valJSON, err := json.Marshal(val)
		if err != nil {
			parts = append(parts, fmt.Sprintf("%s:<unhashable:%T>", k, val))
		} else {
			parts = append(parts, fmt.Sprintf("%s:%s", k, string(valJSON)))
		}
	}

	content := strings.Join(parts, "|")
	hasher := fnv.New64a()
	hasher.Write([]byte(content))
	hash := hex.EncodeToString(hasher.Sum(nil))

	if len(hash) >= HashPrefixLength {
		return hash[:HashPrefixLength]
	}
	return hash
}
