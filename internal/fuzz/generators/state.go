// Package generators provides random data generation for fuzz testing.
package generators

import (
	"fmt"
	"math/rand"

	"pgregory.net/rapid"
)

// FieldType represents the type of a state field.
type FieldType int

const (
	FieldString FieldType = iota
	FieldInt
	FieldBool
	FieldFloat
	FieldPointer
	FieldMap
)

// SliceShape describes the expected shape of a slice field.
type SliceShape struct {
	ItemShape *StateShape
	MinLen    int
	MaxLen    int
}

// StateShape describes the expected structure of state data.
type StateShape struct {
	Fields map[string]FieldType
	Slices map[string]SliceShape
	Nested map[string]*StateShape
}

// DefaultStateShape returns a common state shape for todo-like applications.
func DefaultStateShape() *StateShape {
	return &StateShape{
		Fields: map[string]FieldType{
			"Title":    FieldString,
			"Count":    FieldInt,
			"ShowMenu": FieldBool,
			"Status":   FieldString,
		},
		Slices: map[string]SliceShape{
			"Items": {
				ItemShape: &StateShape{
					Fields: map[string]FieldType{
						"ID":       FieldString,
						"Text":     FieldString,
						"Complete": FieldBool,
						"Priority": FieldString,
					},
				},
				MinLen: 0,
				MaxLen: 10,
			},
		},
		Nested: map[string]*StateShape{
			"User": {
				Fields: map[string]FieldType{
					"Name":   FieldString,
					"Email":  FieldString,
					"Active": FieldBool,
				},
			},
		},
	}
}

// RangeHeavyStateShape returns a state shape optimized for testing range operations.
// MinLen is 0 to test empty↔items transitions (oracle hardened to support this).
func RangeHeavyStateShape() *StateShape {
	return &StateShape{
		Fields: map[string]FieldType{
			"Title": FieldString,
		},
		Slices: map[string]SliceShape{
			"Items": {
				ItemShape: &StateShape{
					Fields: map[string]FieldType{
						"ID":   FieldString,
						"Text": FieldString,
					},
				},
				MinLen: 0, // ENABLED: tests empty→items transitions
				MaxLen: 20,
			},
			"Tags": {
				ItemShape: &StateShape{
					Fields: map[string]FieldType{
						"ID":    FieldString,
						"Label": FieldString,
						"Color": FieldString,
					},
				},
				MinLen: 0, // ENABLED: tests empty→items transitions
				MaxLen: 10,
			},
		},
	}
}

// NestedStateShape returns a state shape with nested ranges for complex testing.
func NestedStateShape() *StateShape {
	return &StateShape{
		Fields: map[string]FieldType{
			"Title": FieldString,
		},
		Slices: map[string]SliceShape{
			"Groups": {
				ItemShape: &StateShape{
					Fields: map[string]FieldType{
						"ID":   FieldString,
						"Name": FieldString,
					},
					Slices: map[string]SliceShape{
						"Items": {
							ItemShape: &StateShape{
								Fields: map[string]FieldType{
									"ID":   FieldString,
									"Text": FieldString,
								},
							},
							MinLen: 0,
							MaxLen: 5,
						},
					},
				},
				MinLen: 0,
				MaxLen: 5,
			},
		},
	}
}

// GenState generates state data matching a shape using rapid.
func GenState(shape *StateShape) *rapid.Generator[map[string]any] {
	return rapid.Custom(func(t *rapid.T) map[string]any {
		return genStateFromShape(t, shape, 0)
	})
}

func genStateFromShape(t *rapid.T, shape *StateShape, depth int) map[string]any {
	state := make(map[string]any)

	// Generate fields
	for name, ftype := range shape.Fields {
		state[name] = genFieldValue(t, ftype, name)
	}

	// Generate slices
	for name, sshape := range shape.Slices {
		length := rapid.IntRange(sshape.MinLen, sshape.MaxLen).Draw(t, name+"_len")
		items := make([]map[string]any, length)
		for i := 0; i < length; i++ {
			items[i] = genStateFromShape(t, sshape.ItemShape, depth+1)
			// Ensure unique ID for range tracking
			if _, hasID := items[i]["ID"]; !hasID {
				items[i]["ID"] = fmt.Sprintf("item-%d-%d", depth, i)
			} else {
				// Make ID unique across all items
				items[i]["ID"] = fmt.Sprintf("%v-%d", items[i]["ID"], i)
			}
		}
		state[name] = items
	}

	// Generate nested shapes
	for name, nested := range shape.Nested {
		state[name] = genStateFromShape(t, nested, depth+1)
	}

	return state
}

func genFieldValue(t *rapid.T, ftype FieldType, name string) any {
	switch ftype {
	case FieldString:
		return genStringValue(t, name)
	case FieldInt:
		return rapid.IntRange(0, 100).Draw(t, name)
	case FieldBool:
		return rapid.Bool().Draw(t, name)
	case FieldFloat:
		return rapid.Float64Range(0, 100).Draw(t, name)
	case FieldPointer:
		if rapid.Bool().Draw(t, name+"_nil") {
			return nil
		}
		return genStringValue(t, name)
	case FieldMap:
		m := make(map[string]any)
		n := rapid.IntRange(0, 3).Draw(t, name+"_size")
		for i := 0; i < n; i++ {
			key := rapid.StringMatching(`[a-z]+`).Draw(t, fmt.Sprintf("%s_key_%d", name, i))
			m[key] = rapid.StringMatching(`[a-zA-Z0-9 ]+`).Draw(t, fmt.Sprintf("%s_val_%d", name, i))
		}
		return m
	default:
		return nil
	}
}

func genStringValue(t *rapid.T, name string) string {
	// Use name-appropriate strings
	switch name {
	case "ID":
		return "item-" + rapid.StringMatching(`[a-z0-9]{4}`).Draw(t, name)
	case "Title", "Name", "Text", "Label":
		words := []string{"todo", "task", "item", "work", "project", "feature", "bug", "test"}
		n := rapid.IntRange(1, 3).Draw(t, name+"_words")
		result := ""
		for i := 0; i < n; i++ {
			if i > 0 {
				result += " "
			}
			result += rapid.SampledFrom(words).Draw(t, fmt.Sprintf("%s_word_%d", name, i))
		}
		return result
	case "Email":
		return rapid.StringMatching(`[a-z]+@example\.com`).Draw(t, name)
	case "Status":
		return rapid.SampledFrom([]string{"active", "inactive", "pending", "complete"}).Draw(t, name)
	case "Priority":
		return rapid.SampledFrom([]string{"low", "medium", "high"}).Draw(t, name)
	case "Color":
		return rapid.SampledFrom([]string{"red", "green", "blue", "yellow", "purple"}).Draw(t, name)
	default:
		return rapid.StringMatching(`[a-zA-Z0-9 ]+`).Draw(t, name)
	}
}

// GenStateSimple generates state using standard library rand (not rapid).
// Useful for non-property-based tests.
func GenStateSimple(rng *rand.Rand, shape *StateShape) map[string]any {
	return genStateSimple(rng, shape, 0)
}

func genStateSimple(rng *rand.Rand, shape *StateShape, depth int) map[string]any {
	state := make(map[string]any)

	// Generate fields
	for name, ftype := range shape.Fields {
		state[name] = genFieldValueSimple(rng, ftype, name)
	}

	// Generate slices
	for name, sshape := range shape.Slices {
		length := sshape.MinLen + rng.Intn(sshape.MaxLen-sshape.MinLen+1)
		items := make([]map[string]any, length)
		for i := 0; i < length; i++ {
			items[i] = genStateSimple(rng, sshape.ItemShape, depth+1)
			// Ensure unique ID
			items[i]["ID"] = fmt.Sprintf("item-%d-%d", depth, i)
		}
		state[name] = items
	}

	// Generate nested
	for name, nested := range shape.Nested {
		state[name] = genStateSimple(rng, nested, depth+1)
	}

	return state
}

func genFieldValueSimple(rng *rand.Rand, ftype FieldType, name string) any {
	switch ftype {
	case FieldString:
		words := []string{"todo", "task", "item", "work", "project"}
		return words[rng.Intn(len(words))]
	case FieldInt:
		return rng.Intn(100)
	case FieldBool:
		return rng.Float32() > 0.5
	case FieldFloat:
		return rng.Float64() * 100
	case FieldPointer:
		if rng.Float32() > 0.5 {
			return nil
		}
		return "pointer-value"
	default:
		return nil
	}
}

// GenItem generates a single item matching a shape.
func GenItem(shape *StateShape) *rapid.Generator[map[string]any] {
	return rapid.Custom(func(t *rapid.T) map[string]any {
		item := genStateFromShape(t, shape, 0)
		// Ensure unique ID
		item["ID"] = "item-" + rapid.StringMatching(`[a-z0-9]{6}`).Draw(t, "item_id")
		return item
	})
}

// GenItemSimple generates a single item using standard rand.
func GenItemSimple(rng *rand.Rand, shape *StateShape) map[string]any {
	item := genStateSimple(rng, shape, 0)
	item["ID"] = fmt.Sprintf("item-%d", rng.Intn(10000))
	return item
}

// GenPermutation generates a random permutation of indices.
func GenPermutation(length int) *rapid.Generator[[]int] {
	return rapid.Custom(func(t *rapid.T) []int {
		perm := make([]int, length)
		for i := range perm {
			perm[i] = i
		}
		// Fisher-Yates shuffle
		for i := length - 1; i > 0; i-- {
			j := rapid.IntRange(0, i).Draw(t, fmt.Sprintf("perm_%d", i))
			perm[i], perm[j] = perm[j], perm[i]
		}
		return perm
	})
}

// GenPermutationSimple generates a permutation using standard rand.
func GenPermutationSimple(rng *rand.Rand, length int) []int {
	perm := make([]int, length)
	for i := range perm {
		perm[i] = i
	}
	rng.Shuffle(length, func(i, j int) {
		perm[i], perm[j] = perm[j], perm[i]
	})
	return perm
}
