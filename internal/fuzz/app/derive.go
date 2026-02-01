package app

import (
	"sort"
	"strings"
)

// DeriveFilteredItems computes the FilteredItems field from Items and view settings.
// This is the core function that simulates real application behavior where:
// 1. Filter controls which items are shown (all/active/completed)
// 2. SearchQuery filters by text match
// 3. SortBy + SortOrder control the display order
//
// When view settings change, the entire derived list transforms - this is
// fundamentally different from direct CRUD mutations and exercises different
// code paths in the diff algorithm.
func DeriveFilteredItems(state *AppState) {
	if state.Items == nil {
		state.FilteredItems = nil
		return
	}

	// Start with a copy of all items
	result := make([]Item, 0, len(state.Items))
	for _, item := range state.Items {
		result = append(result, item)
	}

	// Step 1: Filter by status
	result = filterByStatus(result, state.Filter)

	// Step 2: Filter by search query
	result = filterBySearch(result, state.SearchQuery)

	// Step 3: Sort
	sortItems(result, state.SortBy, state.SortOrder)

	state.FilteredItems = result
}

// filterByStatus filters items by completion status.
func filterByStatus(items []Item, filter string) []Item {
	switch filter {
	case "active":
		return filterItems(items, func(i Item) bool { return !i.Complete })
	case "completed":
		return filterItems(items, func(i Item) bool { return i.Complete })
	default: // "all"
		return items
	}
}

// filterBySearch filters items by search query (case-insensitive title match).
func filterBySearch(items []Item, query string) []Item {
	if query == "" {
		return items
	}
	query = strings.ToLower(query)
	return filterItems(items, func(i Item) bool {
		return strings.Contains(strings.ToLower(i.Title), query)
	})
}

// filterItems returns items matching the predicate.
func filterItems(items []Item, predicate func(Item) bool) []Item {
	result := make([]Item, 0, len(items))
	for _, item := range items {
		if predicate(item) {
			result = append(result, item)
		}
	}
	return result
}

// sortItems sorts items in place by the specified field and order.
func sortItems(items []Item, sortBy, sortOrder string) {
	if len(items) <= 1 {
		return
	}

	sort.SliceStable(items, func(i, j int) bool {
		var cmp int
		switch sortBy {
		case "alpha":
			cmp = strings.Compare(strings.ToLower(items[i].Title), strings.ToLower(items[j].Title))
		case "priority":
			cmp = priorityRank(items[i].Priority) - priorityRank(items[j].Priority)
		default: // "created"
			cmp = items[i].CreatedAt - items[j].CreatedAt
		}

		if sortOrder == "desc" {
			cmp = -cmp
		}

		return cmp < 0
	})
}

// priorityRank returns the numeric rank of a priority value.
// Lower number = higher priority (high < medium < low).
func priorityRank(priority string) int {
	switch priority {
	case "high":
		return 0
	case "medium":
		return 1
	case "low":
		return 2
	default:
		return 3
	}
}
