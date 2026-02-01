package app

import (
	"fmt"
	"math/rand"

	"github.com/livetemplate/livetemplate/internal/fuzz/mutations"
)

var categoryCounter int

// ApplyNestedRangeMutation applies a mutation to the NestedRangeState.
// Returns an error if the mutation cannot be applied.
func ApplyNestedRangeMutation(state *NestedRangeState, mutation mutations.Mutation) error {
	switch mutation.Type {
	case mutations.MutToggleExpand:
		if mutation.Index < 0 || mutation.Index >= len(state.Categories) {
			return fmt.Errorf("category index %d out of range [0, %d)", mutation.Index, len(state.Categories))
		}
		state.Categories[mutation.Index].IsOpen = !state.Categories[mutation.Index].IsOpen

	case mutations.MutAddToCategory:
		if mutation.Index < 0 || mutation.Index >= len(state.Categories) {
			return fmt.Errorf("category index %d out of range [0, %d)", mutation.Index, len(state.Categories))
		}
		item, ok := mutation.Value.(Item)
		if !ok {
			if m, ok := mutation.Value.(map[string]any); ok {
				item = itemFromMap(m)
			} else {
				return fmt.Errorf("MutAddToCategory requires Item value, got %T", mutation.Value)
			}
		}
		state.Categories[mutation.Index].Items = append(state.Categories[mutation.Index].Items, item)

	case mutations.MutRemoveFromCategory:
		if mutation.Index < 0 || mutation.Index >= len(state.Categories) {
			return fmt.Errorf("category index %d out of range [0, %d)", mutation.Index, len(state.Categories))
		}
		cat := &state.Categories[mutation.Index]
		if mutation.Index2 < 0 || mutation.Index2 >= len(cat.Items) {
			return fmt.Errorf("item index %d out of range [0, %d)", mutation.Index2, len(cat.Items))
		}
		cat.Items = append(cat.Items[:mutation.Index2], cat.Items[mutation.Index2+1:]...)

	case mutations.MutMoveToCategory:
		// Value contains destination category index
		destCatIdx, ok := mutation.Value.(int)
		if !ok {
			return fmt.Errorf("MutMoveToCategory requires int destination, got %T", mutation.Value)
		}
		if mutation.Index < 0 || mutation.Index >= len(state.Categories) {
			return fmt.Errorf("source category index %d out of range", mutation.Index)
		}
		if destCatIdx < 0 || destCatIdx >= len(state.Categories) {
			return fmt.Errorf("destination category index %d out of range", destCatIdx)
		}
		srcCat := &state.Categories[mutation.Index]
		if mutation.Index2 < 0 || mutation.Index2 >= len(srcCat.Items) {
			return fmt.Errorf("item index %d out of range", mutation.Index2)
		}
		// Move item
		item := srcCat.Items[mutation.Index2]
		srcCat.Items = append(srcCat.Items[:mutation.Index2], srcCat.Items[mutation.Index2+1:]...)
		state.Categories[destCatIdx].Items = append(state.Categories[destCatIdx].Items, item)

	case mutations.MutAddCategory:
		cat, ok := mutation.Value.(Category)
		if !ok {
			if m, ok := mutation.Value.(map[string]any); ok {
				cat = categoryFromMap(m)
			} else {
				return fmt.Errorf("MutAddCategory requires Category value, got %T", mutation.Value)
			}
		}
		state.Categories = append(state.Categories, cat)

	case mutations.MutRemoveCategory:
		if mutation.Index < 0 || mutation.Index >= len(state.Categories) {
			return fmt.Errorf("category index %d out of range [0, %d)", mutation.Index, len(state.Categories))
		}
		state.Categories = append(state.Categories[:mutation.Index], state.Categories[mutation.Index+1:]...)

	case mutations.MutReorderWithin:
		if mutation.Index < 0 || mutation.Index >= len(state.Categories) {
			return fmt.Errorf("category index %d out of range", mutation.Index)
		}
		perm, ok := mutation.Value.([]int)
		if !ok {
			return fmt.Errorf("MutReorderWithin requires []int permutation, got %T", mutation.Value)
		}
		cat := &state.Categories[mutation.Index]
		if len(perm) != len(cat.Items) {
			return fmt.Errorf("permutation length %d != items length %d", len(perm), len(cat.Items))
		}
		newItems := make([]Item, len(cat.Items))
		for i, idx := range perm {
			if idx < 0 || idx >= len(cat.Items) {
				return fmt.Errorf("invalid permutation index: %d", idx)
			}
			newItems[i] = cat.Items[idx]
		}
		cat.Items = newItems

	case mutations.MutReorderCategories:
		perm, ok := mutation.Value.([]int)
		if !ok {
			return fmt.Errorf("MutReorderCategories requires []int permutation, got %T", mutation.Value)
		}
		if len(perm) != len(state.Categories) {
			return fmt.Errorf("permutation length %d != categories length %d", len(perm), len(state.Categories))
		}
		newCats := make([]Category, len(state.Categories))
		for i, idx := range perm {
			if idx < 0 || idx >= len(state.Categories) {
				return fmt.Errorf("invalid permutation index: %d", idx)
			}
			newCats[i] = state.Categories[idx]
		}
		state.Categories = newCats

	case mutations.MutUpdateCategory:
		if mutation.Index < 0 || mutation.Index >= len(state.Categories) {
			return fmt.Errorf("category index %d out of range", mutation.Index)
		}
		updates, ok := mutation.Value.(map[string]any)
		if !ok {
			return fmt.Errorf("MutUpdateCategory requires map[string]any updates, got %T", mutation.Value)
		}
		cat := &state.Categories[mutation.Index]
		if v, ok := updates["Name"].(string); ok {
			cat.Name = v
		}
		if v, ok := updates["IsOpen"].(bool); ok {
			cat.IsOpen = v
		}

	case mutations.MutUpdateItem:
		// Update item within a category
		if mutation.Index < 0 || mutation.Index >= len(state.Categories) {
			return fmt.Errorf("category index %d out of range", mutation.Index)
		}
		cat := &state.Categories[mutation.Index]
		if mutation.Index2 < 0 || mutation.Index2 >= len(cat.Items) {
			return fmt.Errorf("item index %d out of range", mutation.Index2)
		}
		updates, ok := mutation.Value.(map[string]any)
		if !ok {
			return fmt.Errorf("MutUpdateItem requires map[string]any updates, got %T", mutation.Value)
		}
		item := &cat.Items[mutation.Index2]
		applyItemUpdates(item, updates)

	case mutations.MutClearSlice:
		// Clear items from a category
		if mutation.Index < 0 || mutation.Index >= len(state.Categories) {
			return fmt.Errorf("category index %d out of range", mutation.Index)
		}
		state.Categories[mutation.Index].Items = nil

	default:
		return fmt.Errorf("unsupported mutation type for NestedRangeState: %s", mutation.Type)
	}

	return nil
}

func categoryFromMap(m map[string]any) Category {
	cat := Category{}
	if v, ok := m["ID"].(string); ok {
		cat.ID = v
	}
	if v, ok := m["Name"].(string); ok {
		cat.Name = v
	}
	if v, ok := m["IsOpen"].(bool); ok {
		cat.IsOpen = v
	}
	return cat
}

// GenNestedRangeMutation generates a random mutation for NestedRangeState based on weights.
func GenNestedRangeMutation(rng *rand.Rand, state *NestedRangeState, weights mutations.MutationWeights) mutations.Mutation {
	type weightedMutation struct {
		mutType mutations.MutationType
		weight  float64
	}

	options := []weightedMutation{
		{mutations.MutToggleExpand, weights.ToggleExpand},
		{mutations.MutAddToCategory, weights.AddToCategory},
		{mutations.MutRemoveFromCategory, weights.RemoveFromCategory},
		{mutations.MutMoveToCategory, weights.MoveToCategory},
		{mutations.MutAddCategory, weights.AddCategory},
		{mutations.MutRemoveCategory, weights.RemoveCategory},
		{mutations.MutReorderWithin, weights.ReorderWithin},
		{mutations.MutReorderCategories, weights.ReorderCategories},
		{mutations.MutUpdateCategory, weights.UpdateCategory},
		{mutations.MutUpdateItem, weights.UpdateItem},
		{mutations.MutClearSlice, weights.ClearSlice},
	}

	choice := rng.Float64()
	cumulative := 0.0
	var selectedType mutations.MutationType

	for _, opt := range options {
		cumulative += opt.weight
		if choice < cumulative {
			selectedType = opt.mutType
			break
		}
	}

	if selectedType == "" {
		selectedType = mutations.MutAddCategory // Fallback
	}

	return genNestedMutationOfType(rng, state, selectedType)
}

func genNestedMutationOfType(rng *rand.Rand, state *NestedRangeState, mutType mutations.MutationType) mutations.Mutation {
	switch mutType {
	case mutations.MutToggleExpand:
		if len(state.Categories) == 0 {
			return genNestedMutationOfType(rng, state, mutations.MutAddCategory)
		}
		return mutations.Mutation{
			Type:  mutations.MutToggleExpand,
			Index: rng.Intn(len(state.Categories)),
		}

	case mutations.MutAddToCategory:
		if len(state.Categories) == 0 {
			return genNestedMutationOfType(rng, state, mutations.MutAddCategory)
		}
		catIdx := rng.Intn(len(state.Categories))
		return mutations.Mutation{
			Type:  mutations.MutAddToCategory,
			Index: catIdx,
			Value: genItem(rng, totalItems(state)),
		}

	case mutations.MutRemoveFromCategory:
		catIdx, itemIdx := findCategoryWithItems(rng, state)
		if catIdx < 0 {
			return genNestedMutationOfType(rng, state, mutations.MutAddToCategory)
		}
		return mutations.Mutation{
			Type:   mutations.MutRemoveFromCategory,
			Index:  catIdx,
			Index2: itemIdx,
		}

	case mutations.MutMoveToCategory:
		if len(state.Categories) < 2 {
			return genNestedMutationOfType(rng, state, mutations.MutAddCategory)
		}
		srcCatIdx, itemIdx := findCategoryWithItems(rng, state)
		if srcCatIdx < 0 {
			return genNestedMutationOfType(rng, state, mutations.MutAddToCategory)
		}
		destCatIdx := rng.Intn(len(state.Categories))
		for destCatIdx == srcCatIdx {
			destCatIdx = rng.Intn(len(state.Categories))
		}
		return mutations.Mutation{
			Type:   mutations.MutMoveToCategory,
			Index:  srcCatIdx,
			Index2: itemIdx,
			Value:  destCatIdx,
		}

	case mutations.MutAddCategory:
		return mutations.Mutation{
			Type:  mutations.MutAddCategory,
			Value: genCategory(rng),
		}

	case mutations.MutRemoveCategory:
		if len(state.Categories) == 0 {
			return genNestedMutationOfType(rng, state, mutations.MutAddCategory)
		}
		return mutations.Mutation{
			Type:  mutations.MutRemoveCategory,
			Index: rng.Intn(len(state.Categories)),
		}

	case mutations.MutReorderWithin:
		catIdx, _ := findCategoryWithMultipleItems(rng, state)
		if catIdx < 0 {
			return genNestedMutationOfType(rng, state, mutations.MutAddToCategory)
		}
		perm := genPermutation(rng, len(state.Categories[catIdx].Items))
		return mutations.Mutation{
			Type:  mutations.MutReorderWithin,
			Index: catIdx,
			Value: perm,
		}

	case mutations.MutReorderCategories:
		if len(state.Categories) < 2 {
			return genNestedMutationOfType(rng, state, mutations.MutAddCategory)
		}
		perm := genPermutation(rng, len(state.Categories))
		return mutations.Mutation{
			Type:  mutations.MutReorderCategories,
			Value: perm,
		}

	case mutations.MutUpdateCategory:
		if len(state.Categories) == 0 {
			return genNestedMutationOfType(rng, state, mutations.MutAddCategory)
		}
		catIdx := rng.Intn(len(state.Categories))
		updates := make(map[string]any)
		if rng.Float32() > 0.5 {
			updates["Name"] = genCategoryName(rng)
		}
		if rng.Float32() > 0.5 {
			updates["IsOpen"] = rng.Float32() > 0.5
		}
		if len(updates) == 0 {
			updates["Name"] = genCategoryName(rng)
		}
		return mutations.Mutation{
			Type:  mutations.MutUpdateCategory,
			Index: catIdx,
			Value: updates,
		}

	case mutations.MutUpdateItem:
		catIdx, itemIdx := findCategoryWithItems(rng, state)
		if catIdx < 0 {
			return genNestedMutationOfType(rng, state, mutations.MutAddToCategory)
		}
		updates := make(map[string]any)
		if rng.Float32() > 0.5 {
			updates["Title"] = genTitle(rng)
		}
		if rng.Float32() > 0.5 {
			updates["Complete"] = rng.Float32() > 0.5
		}
		if rng.Float32() > 0.7 {
			updates["Priority"] = PriorityValues[rng.Intn(len(PriorityValues))]
		}
		if len(updates) == 0 {
			updates["Complete"] = !state.Categories[catIdx].Items[itemIdx].Complete
		}
		return mutations.Mutation{
			Type:   mutations.MutUpdateItem,
			Index:  catIdx,
			Index2: itemIdx,
			Value:  updates,
		}

	case mutations.MutClearSlice:
		if len(state.Categories) == 0 {
			return genNestedMutationOfType(rng, state, mutations.MutAddCategory)
		}
		return mutations.Mutation{
			Type:  mutations.MutClearSlice,
			Index: rng.Intn(len(state.Categories)),
		}

	default:
		return mutations.Mutation{
			Type:  mutations.MutAddCategory,
			Value: genCategory(rng),
		}
	}
}

func totalItems(state *NestedRangeState) int {
	count := 0
	for _, cat := range state.Categories {
		count += len(cat.Items)
	}
	return count
}

func findCategoryWithItems(rng *rand.Rand, state *NestedRangeState) (catIdx, itemIdx int) {
	// Find categories with items
	var withItems []int
	for i, cat := range state.Categories {
		if len(cat.Items) > 0 {
			withItems = append(withItems, i)
		}
	}
	if len(withItems) == 0 {
		return -1, -1
	}
	catIdx = withItems[rng.Intn(len(withItems))]
	itemIdx = rng.Intn(len(state.Categories[catIdx].Items))
	return catIdx, itemIdx
}

func findCategoryWithMultipleItems(rng *rand.Rand, state *NestedRangeState) (catIdx, itemCount int) {
	var withMultiple []int
	for i, cat := range state.Categories {
		if len(cat.Items) > 1 {
			withMultiple = append(withMultiple, i)
		}
	}
	if len(withMultiple) == 0 {
		return -1, 0
	}
	catIdx = withMultiple[rng.Intn(len(withMultiple))]
	return catIdx, len(state.Categories[catIdx].Items)
}

func genCategory(rng *rand.Rand) Category {
	categoryCounter++
	return Category{
		ID:     fmt.Sprintf("cat-%d", categoryCounter),
		Name:   genCategoryName(rng),
		Items:  nil,
		IsOpen: rng.Float32() > 0.3, // 70% chance of being open
	}
}

func genCategoryName(rng *rand.Rand) string {
	names := []string{"Work", "Personal", "Shopping", "Ideas", "Projects", "Archive", "Urgent", "Later"}
	return names[rng.Intn(len(names))] + fmt.Sprintf(" %d", rng.Intn(100))
}

// GenNestedRangeState generates a random NestedRangeState for testing.
func GenNestedRangeState(rng *rand.Rand) *NestedRangeState {
	state := DefaultNestedRangeState()

	// Generate 2-5 categories
	numCategories := 2 + rng.Intn(4)
	state.Categories = make([]Category, numCategories)
	for i := range state.Categories {
		state.Categories[i] = genCategory(rng)
		// Each category has 0-4 items
		numItems := rng.Intn(5)
		state.Categories[i].Items = make([]Item, numItems)
		for j := range state.Categories[i].Items {
			state.Categories[i].Items[j] = genItem(rng, totalItems(state))
		}
	}

	return state
}
