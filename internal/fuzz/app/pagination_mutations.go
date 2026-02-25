package app

import (
	"fmt"
	"math/rand"

	"github.com/livetemplate/livetemplate/internal/fuzz/mutations"
)

var paginationItemCounter int

// ApplyPaginationMutation applies a mutation to the PaginatedState.
// Returns an error if the mutation cannot be applied.
func ApplyPaginationMutation(state *PaginatedState, mutation mutations.Mutation) error {
	switch mutation.Type {
	case mutations.MutLoadMore:
		if !state.HasMore {
			return fmt.Errorf("no more items to load")
		}
		state.Page++
		// HasMore is updated by DeriveVisibleItems

	case mutations.MutLoadPrevious:
		if state.Page <= 0 {
			return fmt.Errorf("already at first page")
		}
		state.Page--

	case mutations.MutResetPage:
		state.Page = 0

	case mutations.MutJumpToPage:
		page, ok := mutation.Value.(int)
		if !ok {
			return fmt.Errorf("MutJumpToPage requires int value, got %T", mutation.Value)
		}
		maxPage := max((len(state.Items)-1)/state.PageSize, 0)
		if page < 0 || page > maxPage {
			return fmt.Errorf("page %d out of range [0, %d]", page, maxPage)
		}
		state.Page = page

	case mutations.MutPageSizeChange:
		size, ok := mutation.Value.(int)
		if !ok {
			return fmt.Errorf("MutPageSizeChange requires int value, got %T", mutation.Value)
		}
		if size < 1 {
			return fmt.Errorf("page size must be at least 1")
		}
		state.PageSize = size
		// Reset to first page to avoid out-of-bounds
		state.Page = 0

	case mutations.MutToggleLoading:
		state.LoadingMore = !state.LoadingMore

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
		state.Items = append(state.Items[:mutation.Index], state.Items[mutation.Index+1:]...)

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

	case mutations.MutClearSlice:
		state.Items = nil
		state.Page = 0

	default:
		return fmt.Errorf("unsupported mutation type for PaginatedState: %s", mutation.Type)
	}

	return nil
}

// DeriveVisibleItems computes the visible items based on pagination settings.
func DeriveVisibleItems(state *PaginatedState) {
	if len(state.Items) == 0 {
		state.VisibleItems = nil
		state.HasMore = false
		return
	}

	start := state.Page * state.PageSize
	if start >= len(state.Items) {
		// Page out of bounds, reset to last valid page
		state.Page = (len(state.Items) - 1) / state.PageSize
		start = state.Page * state.PageSize
	}

	end := min(start+state.PageSize, len(state.Items))

	state.VisibleItems = make([]Item, end-start)
	copy(state.VisibleItems, state.Items[start:end])
	state.HasMore = end < len(state.Items)
}

// GenPaginationMutation generates a random mutation for PaginatedState based on weights.
func GenPaginationMutation(rng *rand.Rand, state *PaginatedState, weights mutations.MutationWeights) mutations.Mutation {
	type weightedMutation struct {
		mutType mutations.MutationType
		weight  float64
	}

	options := []weightedMutation{
		{mutations.MutLoadMore, weights.LoadMore},
		{mutations.MutLoadPrevious, weights.LoadPrevious},
		{mutations.MutResetPage, weights.ResetPage},
		{mutations.MutJumpToPage, weights.JumpToPage},
		{mutations.MutPageSizeChange, weights.PageSizeChange},
		{mutations.MutToggleLoading, weights.ToggleLoading},
		{mutations.MutAppendSlice, weights.AppendSlice},
		{mutations.MutRemoveSlice, weights.RemoveSlice},
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
		selectedType = mutations.MutAppendSlice // Fallback
	}

	return genPaginationMutationOfType(rng, state, selectedType)
}

func genPaginationMutationOfType(rng *rand.Rand, state *PaginatedState, mutType mutations.MutationType) mutations.Mutation {
	switch mutType {
	case mutations.MutLoadMore:
		if !state.HasMore || len(state.Items) == 0 {
			return genPaginationMutationOfType(rng, state, mutations.MutAppendSlice)
		}
		return mutations.Mutation{Type: mutations.MutLoadMore}

	case mutations.MutLoadPrevious:
		if state.Page <= 0 {
			return genPaginationMutationOfType(rng, state, mutations.MutAppendSlice)
		}
		return mutations.Mutation{Type: mutations.MutLoadPrevious}

	case mutations.MutResetPage:
		return mutations.Mutation{Type: mutations.MutResetPage}

	case mutations.MutJumpToPage:
		if len(state.Items) == 0 {
			return genPaginationMutationOfType(rng, state, mutations.MutAppendSlice)
		}
		maxPage := (len(state.Items) - 1) / state.PageSize
		return mutations.Mutation{
			Type:  mutations.MutJumpToPage,
			Value: rng.Intn(maxPage + 1),
		}

	case mutations.MutPageSizeChange:
		sizes := []int{3, 5, 10, 20}
		return mutations.Mutation{
			Type:  mutations.MutPageSizeChange,
			Value: sizes[rng.Intn(len(sizes))],
		}

	case mutations.MutToggleLoading:
		return mutations.Mutation{Type: mutations.MutToggleLoading}

	case mutations.MutAppendSlice:
		return mutations.Mutation{
			Type:  mutations.MutAppendSlice,
			Value: genPaginationItem(rng),
		}

	case mutations.MutRemoveSlice:
		if len(state.Items) == 0 {
			return genPaginationMutationOfType(rng, state, mutations.MutAppendSlice)
		}
		return mutations.Mutation{
			Type:  mutations.MutRemoveSlice,
			Index: rng.Intn(len(state.Items)),
		}

	case mutations.MutUpdateItem:
		if len(state.Items) == 0 {
			return genPaginationMutationOfType(rng, state, mutations.MutAppendSlice)
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

	case mutations.MutClearSlice:
		return mutations.Mutation{Type: mutations.MutClearSlice}

	default:
		return mutations.Mutation{
			Type:  mutations.MutAppendSlice,
			Value: genPaginationItem(rng),
		}
	}
}

func genPaginationItem(rng *rand.Rand) Item {
	paginationItemCounter++
	return Item{
		ID:        fmt.Sprintf("page-item-%d", paginationItemCounter),
		Title:     genTitle(rng),
		Body:      "",
		Complete:  rng.Float32() > 0.7,
		Priority:  PriorityValues[rng.Intn(len(PriorityValues))],
		CreatedAt: paginationItemCounter,
	}
}

// GenPaginatedState generates a random PaginatedState for testing.
func GenPaginatedState(rng *rand.Rand) *PaginatedState {
	state := DefaultPaginatedState()

	// Generate 5-20 items (multiple pages worth)
	numItems := 5 + rng.Intn(16)
	state.Items = make([]Item, numItems)
	for i := range state.Items {
		state.Items[i] = genPaginationItem(rng)
	}

	// Random page size
	pageSizes := []int{3, 5, 10}
	state.PageSize = pageSizes[rng.Intn(len(pageSizes))]

	// Random starting page
	maxPage := (len(state.Items) - 1) / state.PageSize
	state.Page = rng.Intn(maxPage + 1)

	// Compute derived values
	DeriveVisibleItems(state)

	return state
}
