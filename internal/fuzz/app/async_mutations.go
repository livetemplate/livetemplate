package app

import (
	"fmt"
	"math/rand"

	"github.com/livetemplate/livetemplate/internal/fuzz/mutations"
)

var asyncItemCounter int

// ApplyAsyncMutation applies a mutation to the AsyncState.
// Returns an error if the mutation cannot be applied.
func ApplyAsyncMutation(state *AsyncState, mutation mutations.Mutation) error {
	switch mutation.Type {
	case mutations.MutStartLoading:
		state.Loading = true

	case mutations.MutFinishLoading:
		state.Loading = false

	case mutations.MutStartItemLoad:
		if mutation.Target == "" {
			return fmt.Errorf("MutStartItemLoad requires item ID in Target")
		}
		state.ItemLoading[mutation.Target] = true

	case mutations.MutFinishItemLoad:
		if mutation.Target == "" {
			return fmt.Errorf("MutFinishItemLoad requires item ID in Target")
		}
		delete(state.ItemLoading, mutation.Target)

	case mutations.MutSetItemError:
		if mutation.Target == "" {
			return fmt.Errorf("MutSetItemError requires item ID in Target")
		}
		errorMsg, ok := mutation.Value.(string)
		if !ok {
			return fmt.Errorf("MutSetItemError requires string value, got %T", mutation.Value)
		}
		state.ItemErrors[mutation.Target] = errorMsg
		// Clear loading state on error
		delete(state.ItemLoading, mutation.Target)

	case mutations.MutClearItemError:
		if mutation.Target == "" {
			return fmt.Errorf("MutClearItemError requires item ID in Target")
		}
		delete(state.ItemErrors, mutation.Target)

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
		// Clean up loading/error state for removed item
		delete(state.ItemLoading, removedID)
		delete(state.ItemErrors, removedID)

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

	case mutations.MutOptimisticUpdate:
		// Optimistic update: apply changes and mark as loading
		if mutation.Index < 0 || mutation.Index >= len(state.Items) {
			return fmt.Errorf("index %d out of range [0, %d)", mutation.Index, len(state.Items))
		}
		updates, ok := mutation.Value.(map[string]any)
		if !ok {
			return fmt.Errorf("MutOptimisticUpdate requires map[string]any updates, got %T", mutation.Value)
		}
		item := &state.Items[mutation.Index]
		applyItemUpdates(item, updates)
		state.ItemLoading[item.ID] = true

	case mutations.MutRollbackUpdate:
		// Rollback: just clear loading state (actual rollback would need saved state)
		if mutation.Target == "" {
			return fmt.Errorf("MutRollbackUpdate requires item ID in Target")
		}
		delete(state.ItemLoading, mutation.Target)
		// In a real implementation, we'd restore the original value

	case mutations.MutClearSlice:
		state.Items = nil
		state.ItemLoading = make(map[string]bool)
		state.ItemErrors = make(map[string]string)

	default:
		return fmt.Errorf("unsupported mutation type for AsyncState: %s", mutation.Type)
	}

	return nil
}

// GenAsyncMutation generates a random mutation for AsyncState based on weights.
func GenAsyncMutation(rng *rand.Rand, state *AsyncState, weights mutations.MutationWeights) mutations.Mutation {
	type weightedMutation struct {
		mutType mutations.MutationType
		weight  float64
	}

	options := []weightedMutation{
		{mutations.MutStartLoading, weights.StartLoading},
		{mutations.MutFinishLoading, weights.FinishLoading},
		{mutations.MutStartItemLoad, weights.StartItemLoad},
		{mutations.MutFinishItemLoad, weights.FinishItemLoad},
		{mutations.MutSetItemError, weights.SetItemError},
		{mutations.MutClearItemError, weights.ClearItemError},
		{mutations.MutAppendSlice, weights.AppendSlice},
		{mutations.MutRemoveSlice, weights.RemoveSlice},
		{mutations.MutUpdateItem, weights.UpdateItem},
		{mutations.MutOptimisticUpdate, weights.OptimisticUpdate},
		{mutations.MutRollbackUpdate, weights.RollbackUpdate},
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

	return genAsyncMutationOfType(rng, state, selectedType)
}

func genAsyncMutationOfType(rng *rand.Rand, state *AsyncState, mutType mutations.MutationType) mutations.Mutation {
	switch mutType {
	case mutations.MutStartLoading:
		return mutations.Mutation{Type: mutations.MutStartLoading}

	case mutations.MutFinishLoading:
		if !state.Loading {
			return genAsyncMutationOfType(rng, state, mutations.MutStartLoading)
		}
		return mutations.Mutation{Type: mutations.MutFinishLoading}

	case mutations.MutStartItemLoad:
		if len(state.Items) == 0 {
			return genAsyncMutationOfType(rng, state, mutations.MutAppendSlice)
		}
		item := state.Items[rng.Intn(len(state.Items))]
		return mutations.Mutation{
			Type:   mutations.MutStartItemLoad,
			Target: item.ID,
		}

	case mutations.MutFinishItemLoad:
		// Find an item that's loading
		var loadingItems []string
		for id, loading := range state.ItemLoading {
			if loading {
				loadingItems = append(loadingItems, id)
			}
		}
		if len(loadingItems) == 0 {
			return genAsyncMutationOfType(rng, state, mutations.MutStartItemLoad)
		}
		return mutations.Mutation{
			Type:   mutations.MutFinishItemLoad,
			Target: loadingItems[rng.Intn(len(loadingItems))],
		}

	case mutations.MutSetItemError:
		if len(state.Items) == 0 {
			return genAsyncMutationOfType(rng, state, mutations.MutAppendSlice)
		}
		item := state.Items[rng.Intn(len(state.Items))]
		return mutations.Mutation{
			Type:   mutations.MutSetItemError,
			Target: item.ID,
			Value:  genAsyncErrorMessage(rng),
		}

	case mutations.MutClearItemError:
		// Find an item with an error
		var errorItems []string
		for id, err := range state.ItemErrors {
			if err != "" {
				errorItems = append(errorItems, id)
			}
		}
		if len(errorItems) == 0 {
			return genAsyncMutationOfType(rng, state, mutations.MutSetItemError)
		}
		return mutations.Mutation{
			Type:   mutations.MutClearItemError,
			Target: errorItems[rng.Intn(len(errorItems))],
		}

	case mutations.MutAppendSlice:
		return mutations.Mutation{
			Type:  mutations.MutAppendSlice,
			Value: genAsyncItem(rng),
		}

	case mutations.MutRemoveSlice:
		if len(state.Items) == 0 {
			return genAsyncMutationOfType(rng, state, mutations.MutAppendSlice)
		}
		return mutations.Mutation{
			Type:  mutations.MutRemoveSlice,
			Index: rng.Intn(len(state.Items)),
		}

	case mutations.MutUpdateItem:
		if len(state.Items) == 0 {
			return genAsyncMutationOfType(rng, state, mutations.MutAppendSlice)
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

	case mutations.MutOptimisticUpdate:
		if len(state.Items) == 0 {
			return genAsyncMutationOfType(rng, state, mutations.MutAppendSlice)
		}
		idx := rng.Intn(len(state.Items))
		updates := map[string]any{
			"Complete": !state.Items[idx].Complete,
		}
		return mutations.Mutation{
			Type:  mutations.MutOptimisticUpdate,
			Index: idx,
			Value: updates,
		}

	case mutations.MutRollbackUpdate:
		// Find an item that's loading (simulating failed optimistic update)
		var loadingItems []string
		for id, loading := range state.ItemLoading {
			if loading {
				loadingItems = append(loadingItems, id)
			}
		}
		if len(loadingItems) == 0 {
			return genAsyncMutationOfType(rng, state, mutations.MutOptimisticUpdate)
		}
		return mutations.Mutation{
			Type:   mutations.MutRollbackUpdate,
			Target: loadingItems[rng.Intn(len(loadingItems))],
		}

	default:
		return mutations.Mutation{
			Type:  mutations.MutAppendSlice,
			Value: genAsyncItem(rng),
		}
	}
}

func genAsyncItem(rng *rand.Rand) Item {
	asyncItemCounter++
	return Item{
		ID:        fmt.Sprintf("async-item-%d", asyncItemCounter),
		Title:     genTitle(rng),
		Body:      "",
		Complete:  rng.Float32() > 0.7,
		Priority:  PriorityValues[rng.Intn(len(PriorityValues))],
		CreatedAt: asyncItemCounter,
	}
}

func genAsyncErrorMessage(rng *rand.Rand) string {
	errors := []string{
		"Network error",
		"Server unavailable",
		"Request timeout",
		"Permission denied",
		"Invalid data",
		"Resource not found",
		"Rate limit exceeded",
	}
	return errors[rng.Intn(len(errors))]
}

// GenAsyncState generates a random AsyncState for testing.
func GenAsyncState(rng *rand.Rand) *AsyncState {
	state := DefaultAsyncState()

	// Generate 3-8 items
	numItems := 3 + rng.Intn(6)
	state.Items = make([]Item, numItems)
	for i := range state.Items {
		state.Items[i] = genAsyncItem(rng)
	}

	// Random global loading state
	state.Loading = rng.Float32() > 0.7

	// Random per-item loading states
	for _, item := range state.Items {
		if rng.Float32() > 0.8 {
			state.ItemLoading[item.ID] = true
		}
		if rng.Float32() > 0.9 {
			state.ItemErrors[item.ID] = genAsyncErrorMessage(rng)
		}
	}

	return state
}
