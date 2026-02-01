package app

import (
	"fmt"
	"math/rand"

	"github.com/livetemplate/livetemplate/internal/fuzz/mutations"
)

var bulkItemCounter int

// ApplyBulkMutation applies a mutation to the BulkState.
// Returns an error if the mutation cannot be applied.
func ApplyBulkMutation(state *BulkState, mutation mutations.Mutation) error {
	switch mutation.Type {
	case mutations.MutToggleSelect:
		if mutation.Target == "" {
			return fmt.Errorf("MutToggleSelect requires item ID in Target")
		}
		if state.SelectedIDs[mutation.Target] {
			delete(state.SelectedIDs, mutation.Target)
		} else {
			state.SelectedIDs[mutation.Target] = true
		}
		// Clear SelectAll if we're manually toggling
		state.SelectAll = false

	case mutations.MutSelectAll:
		state.SelectAll = true
		// Clear individual selections since SelectAll covers everything
		state.SelectedIDs = make(map[string]bool)

	case mutations.MutDeselectAll:
		state.SelectAll = false
		state.SelectedIDs = make(map[string]bool)

	case mutations.MutInvertSelection:
		if state.SelectAll {
			// Inverting "all selected" means "none selected"
			state.SelectAll = false
			state.SelectedIDs = make(map[string]bool)
		} else {
			// Invert individual selections
			newSelected := make(map[string]bool)
			for _, item := range state.Items {
				if !state.SelectedIDs[item.ID] {
					newSelected[item.ID] = true
				}
			}
			state.SelectedIDs = newSelected
		}

	case mutations.MutBulkDelete:
		if !state.HasSelection() {
			return nil // Nothing to delete
		}
		var remaining []Item
		for _, item := range state.Items {
			isSelected := state.SelectAll || state.SelectedIDs[item.ID]
			if !isSelected {
				remaining = append(remaining, item)
			}
		}
		state.Items = remaining
		state.SelectAll = false
		state.SelectedIDs = make(map[string]bool)

	case mutations.MutBulkUpdate:
		if !state.HasSelection() {
			return nil // Nothing to update
		}
		updates, ok := mutation.Value.(map[string]any)
		if !ok {
			return fmt.Errorf("MutBulkUpdate requires map[string]any updates, got %T", mutation.Value)
		}
		for i := range state.Items {
			isSelected := state.SelectAll || state.SelectedIDs[state.Items[i].ID]
			if isSelected {
				applyItemUpdates(&state.Items[i], updates)
			}
		}

	case mutations.MutAppendSlice:
		item, ok := mutation.Value.(Item)
		if !ok {
			if m, ok := mutation.Value.(map[string]any); ok {
				item = itemFromMap(m)
			} else {
				return fmt.Errorf("MutAppendSlice requires Item value, got %T", mutation.Value)
			}
		}
		state.Items = append(state.Items, item)

	case mutations.MutRemoveSlice:
		if mutation.Index < 0 || mutation.Index >= len(state.Items) {
			return fmt.Errorf("index %d out of range [0, %d)", mutation.Index, len(state.Items))
		}
		removedID := state.Items[mutation.Index].ID
		state.Items = append(state.Items[:mutation.Index], state.Items[mutation.Index+1:]...)
		// Clean up selection state for removed item
		delete(state.SelectedIDs, removedID)

	case mutations.MutUpdateItem:
		if mutation.Index < 0 || mutation.Index >= len(state.Items) {
			return fmt.Errorf("index %d out of range [0, %d)", mutation.Index, len(state.Items))
		}
		updates, ok := mutation.Value.(map[string]any)
		if !ok {
			return fmt.Errorf("MutUpdateItem requires map[string]any updates, got %T", mutation.Value)
		}
		applyItemUpdates(&state.Items[mutation.Index], updates)

	case mutations.MutClearSlice:
		state.Items = nil
		state.SelectAll = false
		state.SelectedIDs = make(map[string]bool)

	case mutations.MutToggleBool:
		// Toggle SelectAll
		if mutation.Target == "SelectAll" {
			state.SelectAll = !state.SelectAll
			if state.SelectAll {
				state.SelectedIDs = make(map[string]bool)
			}
		}

	default:
		return fmt.Errorf("unsupported mutation type for BulkState: %s", mutation.Type)
	}

	return nil
}

// GenBulkMutation generates a random mutation for BulkState based on weights.
func GenBulkMutation(rng *rand.Rand, state *BulkState, weights mutations.MutationWeights) mutations.Mutation {
	type weightedMutation struct {
		mutType mutations.MutationType
		weight  float64
	}

	options := []weightedMutation{
		{mutations.MutToggleSelect, weights.ToggleSelect},
		{mutations.MutSelectAll, weights.SelectAll},
		{mutations.MutDeselectAll, weights.DeselectAll},
		{mutations.MutInvertSelection, weights.InvertSelection},
		{mutations.MutBulkDelete, weights.BulkDelete},
		{mutations.MutBulkUpdate, weights.BulkUpdate},
		{mutations.MutAppendSlice, weights.AppendSlice},
		{mutations.MutRemoveSlice, weights.RemoveSlice},
		{mutations.MutUpdateItem, weights.UpdateItem},
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
		selectedType = mutations.MutAppendSlice // Fallback
	}

	return genBulkMutationOfType(rng, state, selectedType)
}

func genBulkMutationOfType(rng *rand.Rand, state *BulkState, mutType mutations.MutationType) mutations.Mutation {
	switch mutType {
	case mutations.MutToggleSelect:
		if len(state.Items) == 0 {
			return genBulkMutationOfType(rng, state, mutations.MutAppendSlice)
		}
		item := state.Items[rng.Intn(len(state.Items))]
		return mutations.Mutation{
			Type:   mutations.MutToggleSelect,
			Target: item.ID,
		}

	case mutations.MutSelectAll:
		return mutations.Mutation{Type: mutations.MutSelectAll}

	case mutations.MutDeselectAll:
		return mutations.Mutation{Type: mutations.MutDeselectAll}

	case mutations.MutInvertSelection:
		return mutations.Mutation{Type: mutations.MutInvertSelection}

	case mutations.MutBulkDelete:
		// Ensure something is selected first
		if !state.HasSelection() && len(state.Items) > 0 {
			// Select a random item first
			item := state.Items[rng.Intn(len(state.Items))]
			return mutations.Mutation{
				Type:   mutations.MutToggleSelect,
				Target: item.ID,
			}
		}
		return mutations.Mutation{Type: mutations.MutBulkDelete}

	case mutations.MutBulkUpdate:
		// Ensure something is selected first
		if !state.HasSelection() && len(state.Items) > 0 {
			// Select a random item first
			item := state.Items[rng.Intn(len(state.Items))]
			return mutations.Mutation{
				Type:   mutations.MutToggleSelect,
				Target: item.ID,
			}
		}
		updates := genBulkUpdateFields(rng)
		return mutations.Mutation{
			Type:  mutations.MutBulkUpdate,
			Value: updates,
		}

	case mutations.MutAppendSlice:
		return mutations.Mutation{
			Type:  mutations.MutAppendSlice,
			Value: genBulkItem(rng),
		}

	case mutations.MutRemoveSlice:
		if len(state.Items) == 0 {
			return genBulkMutationOfType(rng, state, mutations.MutAppendSlice)
		}
		return mutations.Mutation{
			Type:  mutations.MutRemoveSlice,
			Index: rng.Intn(len(state.Items)),
		}

	case mutations.MutUpdateItem:
		if len(state.Items) == 0 {
			return genBulkMutationOfType(rng, state, mutations.MutAppendSlice)
		}
		updates := make(map[string]any)
		if rng.Float32() > 0.5 {
			updates["Title"] = genTitle(rng)
		}
		if rng.Float32() > 0.5 {
			updates["Complete"] = rng.Float32() > 0.5
		}
		if len(updates) == 0 {
			updates["Complete"] = !state.Items[rng.Intn(len(state.Items))].Complete
		}
		return mutations.Mutation{
			Type:  mutations.MutUpdateItem,
			Index: rng.Intn(len(state.Items)),
			Value: updates,
		}

	default:
		return mutations.Mutation{
			Type:  mutations.MutAppendSlice,
			Value: genBulkItem(rng),
		}
	}
}

func genBulkItem(rng *rand.Rand) Item {
	bulkItemCounter++
	return Item{
		ID:        fmt.Sprintf("bulk-item-%d", bulkItemCounter),
		Title:     genTitle(rng),
		Body:      "",
		Complete:  rng.Float32() > 0.7,
		Priority:  PriorityValues[rng.Intn(len(PriorityValues))],
		CreatedAt: bulkItemCounter,
	}
}

func genBulkUpdateFields(rng *rand.Rand) map[string]any {
	updates := make(map[string]any)

	// Pick a random field to update
	switch rng.Intn(3) {
	case 0:
		updates["Complete"] = true // Mark all as complete
	case 1:
		updates["Complete"] = false // Mark all as incomplete
	case 2:
		updates["Priority"] = PriorityValues[rng.Intn(len(PriorityValues))]
	}

	return updates
}

// GenBulkState generates a random BulkState for testing.
func GenBulkState(rng *rand.Rand) *BulkState {
	state := DefaultBulkState()

	// Generate 3-10 items
	numItems := 3 + rng.Intn(8)
	state.Items = make([]Item, numItems)
	for i := range state.Items {
		state.Items[i] = genBulkItem(rng)
	}

	// Random selection state
	if rng.Float32() > 0.7 {
		state.SelectAll = true
	} else {
		// Select some items randomly
		for _, item := range state.Items {
			if rng.Float32() > 0.6 {
				state.SelectedIDs[item.ID] = true
			}
		}
	}

	return state
}
