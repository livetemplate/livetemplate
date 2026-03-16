package diff

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"regexp"
	"sort"
	"strings"
)

const (
	// hashPrefixLength is the number of characters to use from the generated hash
	// for compact item identifiers. 12 characters = 48 bits of entropy.
	hashPrefixLength = 12
)

// findKeyAttrPosition searches for key attributes in a string slice.
func findKeyAttrPosition(statics []string, keyAttrs []string) int {
	for i, staticStr := range statics {
		for _, keyAttr := range keyAttrs {
			if strings.Contains(staticStr, keyAttr) {
				return i
			}
		}
	}
	return -1
}

// FindKeyPositionFromStatics parses the statics array to find which position contains the key.
// Supports both []string and []interface{} formats for backward compatibility.
func FindKeyPositionFromStatics(statics interface{}) int {
	keyAttrs := []string{`data-lvt-key="`, `data-key="`, `key="`, `id="`}

	if staticsArr, ok := statics.([]string); ok {
		return findKeyAttrPosition(staticsArr, keyAttrs)
	}

	if staticsArr, ok := statics.([]interface{}); ok {
		stringSlice := make([]string, 0, len(staticsArr))
		for _, static := range staticsArr {
			if staticStr, ok := static.(string); ok {
				stringSlice = append(stringSlice, staticStr)
			} else {
				stringSlice = append(stringSlice, "")
			}
		}
		return findKeyAttrPosition(stringSlice, keyAttrs)
	}

	return -1
}

// GetItemKey extracts the key from a range item using the statics structure.
func GetItemKey(item interface{}, statics interface{}) (string, bool) {
	itemNode, ok := item.(*TreeNode)
	if !ok {
		return "", false
	}

	// First, check for reserved auto-generated key field
	if autoKey, exists := itemNode.GetDynamic("_k"); exists {
		if keyStr, ok := autoKey.(string); ok {
			return keyStr, true
		}
	}

	keyPos := FindKeyPositionFromStatics(statics)

	if keyPos >= 0 {
		keyPosStr := fmt.Sprintf("%d", keyPos)
		if key, exists := itemNode.GetDynamic(keyPosStr); exists {
			if keyStr, ok := key.(string); ok {
				return keyStr, true
			}
		}
	}

	// No explicit key attribute — generate a content-based hash
	return GenerateItemHash(itemNode), true
}

// GenerateItemHash creates a stable hash for a range item based on its content.
// Uses FNV-1a hash for fast, non-cryptographic content fingerprinting.
func GenerateItemHash(item interface{}) string {
	itemNode, ok := item.(*TreeNode)
	if !ok {
		return ""
	}

	keys := make([]string, 0, len(itemNode.Dynamics))
	for k := range itemNode.Dynamics {
		if k != "_k" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		val, _ := itemNode.GetDynamic(k)
		valJSON, err := json.Marshal(val)
		if err != nil {
			parts = append(parts, fmt.Sprintf("%s:%v", k, val))
		} else {
			parts = append(parts, fmt.Sprintf("%s:%s", k, string(valJSON)))
		}
	}

	content := strings.Join(parts, "|")
	hasher := fnv.New64a()
	hasher.Write([]byte(content))
	hash := hex.EncodeToString(hasher.Sum(nil))

	if len(hash) >= hashPrefixLength {
		return hash[:hashPrefixLength]
	}
	return hash
}

// ExtractItemKeys extracts the keys from a slice of range items using the statics structure.
func ExtractItemKeys(items []interface{}, statics interface{}) []string {
	var keys []string
	for _, item := range items {
		if itemNode, ok := item.(*TreeNode); ok {
			if key, ok := GetItemKey(itemNode, statics); ok {
				keys = append(keys, key)
			}
		}
	}
	return keys
}

// DetectPositionField finds the field containing positional display like "#0", "#1", etc.
func DetectPositionField(itemsByKey map[string]interface{}) string {
	positionPattern := regexp.MustCompile(`^#\d+`)

	for _, item := range itemsByKey {
		if itemNode, ok := item.(*TreeNode); ok {
			for field, value := range itemNode.Dynamics {
				if strValue, ok := value.(string); ok {
					if positionPattern.MatchString(strValue) {
						return field
					}
				}
			}
		}
		break // Only check first item
	}
	return ""
}
