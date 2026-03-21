package diff

import (
	"regexp"
	"strings"

	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/keys"
)

var positionPattern = regexp.MustCompile(`^#\d+`)

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
	keyPos := FindKeyPositionFromStatics(statics)
	return getItemKeyWithPos(item, keyPos)
}

// getItemKeyWithPos extracts the key using pre-computed key position.
func getItemKeyWithPos(item interface{}, keyPos int) (string, bool) {
	itemNode, ok := item.(*TreeNode)
	if !ok {
		return "", false
	}

	if itemNode.AutoKey != "" {
		return itemNode.AutoKey, true
	}

	if keyPos >= 0 {
		if key, exists := itemNode.GetDynamic(keyPos); exists {
			if keyStr, ok := key.(string); ok {
				return keyStr, true
			}
		}
	}

	return GenerateItemHash(itemNode), true
}

// GenerateItemHash creates a stable hash for a range item based on its content.
// Uses FNV-1a hash for fast, non-cryptographic content fingerprinting.
func GenerateItemHash(item interface{}) string {
	itemNode, ok := item.(*TreeNode)
	if !ok {
		return ""
	}
	return keys.GenerateItemHashFromSlice(itemNode.Dynamics)
}

// ExtractItemKeys extracts the keys from a slice of range items using the statics structure.
func ExtractItemKeys(items []interface{}, statics interface{}) []string {
	var result []string
	for _, item := range items {
		if itemNode, ok := item.(*TreeNode); ok {
			if key, ok := GetItemKey(itemNode, statics); ok {
				result = append(result, key)
			}
		}
	}
	return result
}

// DetectPositionField finds the field containing positional display like "#0", "#1", etc.
// Returns the position as a string key (e.g., "1", "2") for use in wire format.
func DetectPositionField(itemsByKey map[string]interface{}) string {
	for _, item := range itemsByKey {
		if itemNode, ok := item.(*TreeNode); ok {
			for i, value := range itemNode.Dynamics {
				if value == nil {
					continue
				}
				if strValue, ok := value.(string); ok {
					if positionPattern.MatchString(strValue) {
						return build.PositionKey(i)
					}
				}
			}
		}
		break // Only check first item
	}
	return ""
}
