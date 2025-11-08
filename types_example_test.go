package livetemplate_test

import (
	"encoding/json"
	"fmt"

	"github.com/livetemplate/livetemplate"
)

// ExampleNewTreeNode demonstrates creating an empty TreeNode.
func ExampleNewTreeNode() {
	node := livetemplate.NewTreeNode()
	node.SetDynamic("0", "Hello, World!")

	content, _ := node.GetDynamic("0")
	fmt.Printf("Has dynamics: %v\n", node.HasDynamics())
	fmt.Printf("Content: %v\n", content)
	// Output:
	// Has dynamics: true
	// Content: Hello, World!
}

// ExampleNewTreeNodeWithStatics demonstrates creating a TreeNode with static HTML parts.
func ExampleNewTreeNodeWithStatics() {
	statics := []string{"<div class='greeting'>", "</div>"}
	node := livetemplate.NewTreeNodeWithStatics(statics)
	node.SetDynamic("0", "Welcome!")

	fmt.Printf("Has statics: %v\n", node.HasStatics())
	fmt.Printf("Static parts: %d\n", len(node.Statics))
	// Output:
	// Has statics: true
	// Static parts: 2
}

// ExampleNewRangeData demonstrates creating RangeData for list rendering.
func ExampleNewRangeData() {
	// Create items for a list
	items := []interface{}{
		map[string]interface{}{"name": "Alice", "age": 30},
		map[string]interface{}{"name": "Bob", "age": 25},
	}

	// Static parts that wrap each item
	statics := []string{"<li>", "</li>"}

	rangeData := livetemplate.NewRangeData(items, statics)

	fmt.Printf("Number of items: %d\n", len(rangeData.Items))
	fmt.Printf("Has statics: %v\n", len(rangeData.Statics) > 0)
	// Output:
	// Number of items: 2
	// Has statics: true
}

// ExampleNewTreeMetadata demonstrates creating metadata for tree nodes.
func ExampleNewTreeMetadata() {
	metadata := livetemplate.NewTreeMetadata("user-123")

	fmt.Printf("ID Key: %s\n", metadata.IDKey)
	// Output:
	// ID Key: user-123
}

// ExampleTreeNode_SetDynamic demonstrates setting dynamic values in a TreeNode.
func ExampleTreeNode_SetDynamic() {
	node := livetemplate.NewTreeNode()

	// Set different types of dynamic values
	node.SetDynamic("0", "String value")
	node.SetDynamic("1", 42)
	node.SetDynamic("2", true)
	node.SetDynamic("3", []string{"a", "b", "c"})

	str, _ := node.GetDynamic("0")
	num, _ := node.GetDynamic("1")
	bool, _ := node.GetDynamic("2")
	arr, _ := node.GetDynamic("3")

	fmt.Printf("String: %v\n", str)
	fmt.Printf("Number: %v\n", num)
	fmt.Printf("Boolean: %v\n", bool)
	fmt.Printf("Array length: %d\n", len(arr.([]string)))
	// Output:
	// String: String value
	// Number: 42
	// Boolean: true
	// Array length: 3
}

// ExampleTreeNode_MarshalJSON demonstrates JSON serialization of TreeNodes.
func ExampleTreeNode_MarshalJSON() {
	node := livetemplate.NewTreeNodeWithStatics([]string{"<div>", "</div>"})
	node.SetDynamic("0", "Content")
	node.Fingerprint = "abc123"

	data, _ := json.Marshal(node)

	// Parse to check structure (JSON key order is not guaranteed)
	var result map[string]interface{}
	json.Unmarshal(data, &result)

	fmt.Printf("Has 's' key: %v\n", result["s"] != nil)
	fmt.Printf("Has '0' key: %v\n", result["0"] != nil)
	fmt.Printf("Has 'f' key: %v\n", result["f"] != nil)
	// Output:
	// Has 's' key: true
	// Has '0' key: true
	// Has 'f' key: true
}

// ExampleTreeNode_Clone demonstrates deep cloning of TreeNodes.
func ExampleTreeNode_Clone() {
	original := livetemplate.NewTreeNode()
	original.SetDynamic("0", "Original value")

	clone := original.Clone()
	clone.SetDynamic("0", "Modified value")

	origVal, _ := original.GetDynamic("0")
	cloneVal, _ := clone.GetDynamic("0")

	fmt.Printf("Original: %v\n", origVal)
	fmt.Printf("Clone: %v\n", cloneVal)
	// Output:
	// Original: Original value
	// Clone: Modified value
}

// ExampleTreeNode_ToMap demonstrates converting a TreeNode to a map.
func ExampleTreeNode_ToMap() {
	node := livetemplate.NewTreeNodeWithStatics([]string{"<span>", "</span>"})
	node.SetDynamic("0", "Hello")

	m := node.ToMap()

	fmt.Printf("Type: %T\n", m)
	fmt.Printf("Has 's' key: %v\n", m["s"] != nil)
	fmt.Printf("Dynamic value: %v\n", m["0"])
	// Output:
	// Type: map[string]interface {}
	// Has 's' key: true
	// Dynamic value: Hello
}

// ExampleFromMap demonstrates creating a TreeNode from a map.
func ExampleFromMap() {
	data := map[string]interface{}{
		"s": []interface{}{"<div>", "</div>"},
		"0": "Dynamic content",
		"f": "fingerprint-hash",
	}

	node, _ := livetemplate.FromMap(data)
	content, _ := node.GetDynamic("0")

	fmt.Printf("Has statics: %v\n", node.HasStatics())
	fmt.Printf("Dynamic content: %v\n", content)
	fmt.Printf("Fingerprint: %v\n", node.Fingerprint)
	// Output:
	// Has statics: true
	// Dynamic content: Dynamic content
	// Fingerprint: fingerprint-hash
}

// ExampleNewUpdateContext demonstrates creating a context for template updates.
func ExampleNewUpdateContext() {
	clientStructures := make(map[string]bool)
	ctx := livetemplate.NewUpdateContext(clientStructures)

	fmt.Printf("Is first render: %v\n", ctx.IsFirstRender)
	fmt.Printf("Include statics: %v\n", ctx.IncludeStatics)
	// Output:
	// Is first render: false
	// Include statics: false
}

// ExampleTreeNode_nestedStructure demonstrates creating nested TreeNode structures.
func ExampleTreeNode_nestedStructure() {
	// Create a nested structure: outer div with inner span
	outer := livetemplate.NewTreeNodeWithStatics([]string{"<div>", "</div>"})

	inner := livetemplate.NewTreeNodeWithStatics([]string{"<span>", "</span>"})
	inner.SetDynamic("0", "Nested content")

	outer.SetDynamic("0", inner)

	innerNode, ok := outer.GetDynamic("0")

	fmt.Printf("Outer has dynamics: %v\n", outer.HasDynamics())
	fmt.Printf("Inner exists: %v\n", ok)
	// Type aliases show underlying type, which is expected
	_, isTreeNode := innerNode.(*livetemplate.TreeNode)
	fmt.Printf("Inner is TreeNode: %v\n", isTreeNode)
	// Output:
	// Outer has dynamics: true
	// Inner exists: true
	// Inner is TreeNode: true
}
