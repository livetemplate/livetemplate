package app

import (
	"fmt"
	"math/rand"

	"github.com/livetemplate/livetemplate/internal/fuzz/mutations"
)

// ApplyMutation applies a mutation to the AppState and recomputes FilteredItems.
// Returns an error if the mutation cannot be applied.
func ApplyMutation(state *AppState, mutation mutations.Mutation) error {
	switch mutation.Type {
	case mutations.MutSetFilter:
		value, ok := mutation.Value.(string)
		if !ok {
			return fmt.Errorf("MutSetFilter requires string value, got %T", mutation.Value)
		}
		state.Filter = value

	case mutations.MutSetSort:
		value, ok := mutation.Value.(string)
		if !ok {
			return fmt.Errorf("MutSetSort requires string value, got %T", mutation.Value)
		}
		state.SortBy = value

	case mutations.MutToggleSortOrder:
		if state.SortOrder == "asc" {
			state.SortOrder = "desc"
		} else {
			state.SortOrder = "asc"
		}

	case mutations.MutSetSearch:
		value, ok := mutation.Value.(string)
		if !ok {
			return fmt.Errorf("MutSetSearch requires string value, got %T", mutation.Value)
		}
		state.SearchQuery = value

	case mutations.MutClearSearch:
		state.SearchQuery = ""

	case mutations.MutToggleBool:
		switch mutation.Target {
		case "ShowSearch":
			state.ShowSearch = !state.ShowSearch
		case "ShowFilters":
			state.ShowFilters = !state.ShowFilters
		default:
			return fmt.Errorf("unknown bool target: %s", mutation.Target)
		}

	case mutations.MutSetField:
		switch mutation.Target {
		case "SelectedID":
			value, ok := mutation.Value.(string)
			if !ok {
				return fmt.Errorf("SelectedID requires string value, got %T", mutation.Value)
			}
			state.SelectedID = value
		default:
			return fmt.Errorf("unknown field target: %s", mutation.Target)
		}

	case mutations.MutAppendSlice:
		item, ok := mutation.Value.(Item)
		if !ok {
			// Try map conversion
			if m, ok := mutation.Value.(map[string]any); ok {
				item = itemFromMap(m)
			} else {
				return fmt.Errorf("MutAppendSlice requires Item value, got %T", mutation.Value)
			}
		}
		state.Items = append(state.Items, item)

	case mutations.MutPrependSlice:
		item, ok := mutation.Value.(Item)
		if !ok {
			if m, ok := mutation.Value.(map[string]any); ok {
				item = itemFromMap(m)
			} else {
				return fmt.Errorf("MutPrependSlice requires Item value, got %T", mutation.Value)
			}
		}
		state.Items = append([]Item{item}, state.Items...)

	case mutations.MutRemoveSlice:
		if mutation.Index < 0 || mutation.Index >= len(state.Items) {
			return fmt.Errorf("index %d out of range [0, %d)", mutation.Index, len(state.Items))
		}
		state.Items = append(state.Items[:mutation.Index], state.Items[mutation.Index+1:]...)

	case mutations.MutClearSlice:
		state.Items = nil

	case mutations.MutReorderSlice:
		perm, ok := mutation.Value.([]int)
		if !ok {
			return fmt.Errorf("MutReorderSlice requires []int permutation, got %T", mutation.Value)
		}
		if len(perm) != len(state.Items) {
			return fmt.Errorf("permutation length %d != items length %d", len(perm), len(state.Items))
		}
		newItems := make([]Item, len(state.Items))
		for i, idx := range perm {
			if idx < 0 || idx >= len(state.Items) {
				return fmt.Errorf("invalid permutation index: %d", idx)
			}
			newItems[i] = state.Items[idx]
		}
		state.Items = newItems

	case mutations.MutUpdateItem:
		if mutation.Index < 0 || mutation.Index >= len(state.Items) {
			return fmt.Errorf("index %d out of range [0, %d)", mutation.Index, len(state.Items))
		}
		updates, ok := mutation.Value.(map[string]any)
		if !ok {
			return fmt.Errorf("MutUpdateItem requires map[string]any updates, got %T", mutation.Value)
		}
		item := &state.Items[mutation.Index]
		applyItemUpdates(item, updates)

	default:
		return fmt.Errorf("unsupported mutation type for AppState: %s", mutation.Type)
	}

	// Recompute derived view after any mutation
	DeriveFilteredItems(state)
	return nil
}

// itemFromMap converts a map[string]any to an Item.
func itemFromMap(m map[string]any) Item {
	item := Item{}
	if v, ok := m["ID"].(string); ok {
		item.ID = v
	}
	if v, ok := m["Title"].(string); ok {
		item.Title = v
	}
	if v, ok := m["Body"].(string); ok {
		item.Body = v
	}
	if v, ok := m["Complete"].(bool); ok {
		item.Complete = v
	}
	if v, ok := m["Priority"].(string); ok {
		item.Priority = v
	}
	if v, ok := m["CreatedAt"].(int); ok {
		item.CreatedAt = v
	}
	return item
}

// applyItemUpdates applies updates to an item.
func applyItemUpdates(item *Item, updates map[string]any) {
	if v, ok := updates["Title"].(string); ok {
		item.Title = v
	}
	if v, ok := updates["Body"].(string); ok {
		item.Body = v
	}
	if v, ok := updates["Complete"].(bool); ok {
		item.Complete = v
	}
	if v, ok := updates["Priority"].(string); ok {
		item.Priority = v
	}
}

// GenMutation generates a random mutation for AppState based on weights.
func GenMutation(rng *rand.Rand, state *AppState, weights mutations.MutationWeights) mutations.Mutation {
	// Calculate cumulative weights
	type weightedMutation struct {
		mutType mutations.MutationType
		weight  float64
	}

	options := []weightedMutation{
		{mutations.MutSetFilter, weights.SetFilter},
		{mutations.MutSetSort, weights.SetSort},
		{mutations.MutToggleSortOrder, weights.ToggleSortOrder},
		{mutations.MutSetSearch, weights.SetSearch},
		{mutations.MutClearSearch, weights.ClearSearch},
		{mutations.MutToggleBool, weights.ToggleBool},
		{mutations.MutSetField, weights.SetField},
		{mutations.MutAppendSlice, weights.AppendSlice},
		{mutations.MutPrependSlice, weights.PrependSlice},
		{mutations.MutRemoveSlice, weights.RemoveSlice},
		{mutations.MutClearSlice, weights.ClearSlice},
		{mutations.MutReorderSlice, weights.ReorderSlice},
		{mutations.MutUpdateItem, weights.UpdateItem},
	}

	// Select mutation type by weight
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
		selectedType = mutations.MutSetFilter // Fallback
	}

	return genMutationOfType(rng, state, selectedType)
}

func genMutationOfType(rng *rand.Rand, state *AppState, mutType mutations.MutationType) mutations.Mutation {
	switch mutType {
	case mutations.MutSetFilter:
		// Pick a different filter than current
		values := FilterValues
		newValue := values[rng.Intn(len(values))]
		return mutations.Mutation{
			Type:  mutations.MutSetFilter,
			Value: newValue,
		}

	case mutations.MutSetSort:
		values := SortByValues
		newValue := values[rng.Intn(len(values))]
		return mutations.Mutation{
			Type:  mutations.MutSetSort,
			Value: newValue,
		}

	case mutations.MutToggleSortOrder:
		return mutations.Mutation{
			Type: mutations.MutToggleSortOrder,
		}

	case mutations.MutSetSearch:
		// Generate a search query
		queries := []string{
			"", // sometimes clear
			"a", "b", "the",
			"task", "todo", "item",
			"high", "low", "medium",
			"nonexistent", // test empty results
		}
		return mutations.Mutation{
			Type:  mutations.MutSetSearch,
			Value: queries[rng.Intn(len(queries))],
		}

	case mutations.MutClearSearch:
		return mutations.Mutation{
			Type: mutations.MutClearSearch,
		}

	case mutations.MutToggleBool:
		targets := []string{"ShowSearch", "ShowFilters"}
		return mutations.Mutation{
			Type:   mutations.MutToggleBool,
			Target: targets[rng.Intn(len(targets))],
		}

	case mutations.MutSetField:
		// SelectedID: either empty or a valid item ID
		value := ""
		if len(state.Items) > 0 && rng.Float32() > 0.3 {
			value = state.Items[rng.Intn(len(state.Items))].ID
		}
		return mutations.Mutation{
			Type:   mutations.MutSetField,
			Target: "SelectedID",
			Value:  value,
		}

	case mutations.MutAppendSlice:
		return mutations.Mutation{
			Type:  mutations.MutAppendSlice,
			Value: genItem(rng, len(state.Items)),
		}

	case mutations.MutPrependSlice:
		return mutations.Mutation{
			Type:  mutations.MutPrependSlice,
			Value: genItem(rng, len(state.Items)),
		}

	case mutations.MutRemoveSlice:
		if len(state.Items) == 0 {
			// Can't remove, fallback to append
			return genMutationOfType(rng, state, mutations.MutAppendSlice)
		}
		return mutations.Mutation{
			Type:  mutations.MutRemoveSlice,
			Index: rng.Intn(len(state.Items)),
		}

	case mutations.MutClearSlice:
		return mutations.Mutation{
			Type: mutations.MutClearSlice,
		}

	case mutations.MutReorderSlice:
		if len(state.Items) <= 1 {
			return genMutationOfType(rng, state, mutations.MutAppendSlice)
		}
		perm := genPermutation(rng, len(state.Items))
		return mutations.Mutation{
			Type:  mutations.MutReorderSlice,
			Value: perm,
		}

	case mutations.MutUpdateItem:
		if len(state.Items) == 0 {
			return genMutationOfType(rng, state, mutations.MutAppendSlice)
		}
		idx := rng.Intn(len(state.Items))
		updates := make(map[string]any)
		// Randomly update some fields
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
			updates["Complete"] = !state.Items[idx].Complete // Toggle at minimum
		}
		return mutations.Mutation{
			Type:   mutations.MutUpdateItem,
			Index:  idx,
			Value:  updates,
			Target: "Items",
		}

	default:
		return mutations.Mutation{
			Type:  mutations.MutSetFilter,
			Value: "all",
		}
	}
}

var itemCounter int

func genItem(rng *rand.Rand, numExisting int) Item {
	itemCounter++
	return Item{
		ID:        fmt.Sprintf("item-%d", itemCounter),
		Title:     genTitle(rng),
		Body:      genBody(rng),
		Complete:  rng.Float32() > 0.5,
		Priority:  PriorityValues[rng.Intn(len(PriorityValues))],
		CreatedAt: numExisting + 1,
	}
}

func genTitle(rng *rand.Rand) string {
	words := []string{"Todo", "Task", "Item", "Work", "Project", "Feature", "Bug", "Test", "Review", "Fix"}
	return words[rng.Intn(len(words))] + " " + fmt.Sprintf("%d", rng.Intn(100))
}

func genBody(rng *rand.Rand) string {
	if rng.Float32() > 0.5 {
		return "" // Sometimes empty
	}
	bodies := []string{
		"This is a description",
		"More details here",
		"Important note",
		"",
	}
	return bodies[rng.Intn(len(bodies))]
}

func genPermutation(rng *rand.Rand, length int) []int {
	perm := make([]int, length)
	for i := range perm {
		perm[i] = i
	}
	rng.Shuffle(length, func(i, j int) {
		perm[i], perm[j] = perm[j], perm[i]
	})
	return perm
}

// GenAppState generates a random AppState for testing.
func GenAppState(rng *rand.Rand) *AppState {
	state := DefaultAppState()

	// Generate 3-10 items
	numItems := 3 + rng.Intn(8)
	state.Items = make([]Item, numItems)
	for i := range state.Items {
		state.Items[i] = genItem(rng, i)
	}

	// Random view settings
	state.Filter = FilterValues[rng.Intn(len(FilterValues))]
	state.SortBy = SortByValues[rng.Intn(len(SortByValues))]
	state.SortOrder = SortOrderValues[rng.Intn(len(SortOrderValues))]
	state.ShowSearch = rng.Float32() > 0.3
	state.ShowFilters = rng.Float32() > 0.3

	// Compute initial filtered items
	DeriveFilteredItems(state)

	return state
}
