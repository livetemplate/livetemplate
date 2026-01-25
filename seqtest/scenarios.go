package seqtest

import "fmt"

// Pre-built scenarios for common testing patterns

// TodoScenarios provides common todo app test sequences
var TodoScenarios = []Scenario{
	{
		Name: "two_sequential_adds",
		Actions: []Action{
			{Name: "add", Data: map[string]interface{}{"title": "First item"}},
			{Name: "add", Data: map[string]interface{}{"title": "Second item"}},
		},
	},
	{
		Name: "three_sequential_marks",
		Actions: []Action{
			{Name: "add", Data: map[string]interface{}{"title": "Item 1"}},
			{Name: "add", Data: map[string]interface{}{"title": "Item 2"}},
			{Name: "add", Data: map[string]interface{}{"title": "Item 3"}},
			{Name: "toggle", Data: map[string]interface{}{"index": 0}},
			{Name: "toggle", Data: map[string]interface{}{"index": 1}},
			{Name: "toggle", Data: map[string]interface{}{"index": 2}},
		},
	},
	{
		Name: "add_toggle_add_toggle",
		Actions: []Action{
			{Name: "add", Data: map[string]interface{}{"title": "A"}},
			{Name: "toggle", Data: map[string]interface{}{"index": 0}},
			{Name: "add", Data: map[string]interface{}{"title": "B"}},
			{Name: "toggle", Data: map[string]interface{}{"index": 1}},
		},
	},
	{
		Name: "add_remove_add",
		Actions: []Action{
			{Name: "add", Data: map[string]interface{}{"title": "temporary"}},
			{Name: "remove", Data: map[string]interface{}{"index": 0}},
			{Name: "add", Data: map[string]interface{}{"title": "permanent"}},
		},
	},
	{
		Name: "toggle_untoggle",
		Actions: []Action{
			{Name: "add", Data: map[string]interface{}{"title": "item"}},
			{Name: "toggle", Data: map[string]interface{}{"index": 0}},
			{Name: "toggle", Data: map[string]interface{}{"index": 0}},
		},
	},
	{
		Name: "mixed_operations",
		Actions: []Action{
			{Name: "add", Data: map[string]interface{}{"title": "1"}},
			{Name: "add", Data: map[string]interface{}{"title": "2"}},
			{Name: "toggle", Data: map[string]interface{}{"index": 0}},
			{Name: "add", Data: map[string]interface{}{"title": "3"}},
			{Name: "remove", Data: map[string]interface{}{"index": 1}},
			{Name: "toggle", Data: map[string]interface{}{"index": 0}},
			{Name: "add", Data: map[string]interface{}{"title": "4"}},
		},
	},
	{
		Name: "bulk_add_then_bulk_toggle",
		Actions: append(
			RepeatAction(Action{Name: "add", Data: map[string]interface{}{"title": "item"}}, 5),
			Action{Name: "toggle", Data: map[string]interface{}{"index": 0}},
			Action{Name: "toggle", Data: map[string]interface{}{"index": 1}},
			Action{Name: "toggle", Data: map[string]interface{}{"index": 2}},
			Action{Name: "toggle", Data: map[string]interface{}{"index": 3}},
			Action{Name: "toggle", Data: map[string]interface{}{"index": 4}},
		),
	},
	{
		Name: "clear_completed",
		Actions: []Action{
			{Name: "add", Data: map[string]interface{}{"title": "keep"}},
			{Name: "add", Data: map[string]interface{}{"title": "remove1"}},
			{Name: "add", Data: map[string]interface{}{"title": "remove2"}},
			{Name: "toggle", Data: map[string]interface{}{"index": 1}},
			{Name: "toggle", Data: map[string]interface{}{"index": 2}},
			{Name: "clearCompleted"},
		},
	},
}

// CounterScenarios provides common counter test sequences
var CounterScenarios = []Scenario{
	{
		Name:    "ten_increments",
		Actions: RepeatAction(Action{Name: "increment"}, 10),
	},
	{
		Name: "increment_decrement_mix",
		Actions: []Action{
			{Name: "increment"},
			{Name: "increment"},
			{Name: "decrement"},
			{Name: "increment"},
			{Name: "decrement"},
			{Name: "decrement"},
			{Name: "increment"},
		},
	},
	{
		Name: "add_subtract",
		Actions: []Action{
			{Name: "add", Data: map[string]interface{}{"amount": 10}},
			{Name: "subtract", Data: map[string]interface{}{"amount": 3}},
			{Name: "add", Data: map[string]interface{}{"amount": 5}},
		},
	},
	{
		Name: "boundary_test",
		Actions: []Action{
			{Name: "set", Data: map[string]interface{}{"value": 0}},
			{Name: "decrement"}, // Should handle underflow
			{Name: "set", Data: map[string]interface{}{"value": 1000}},
			{Name: "increment"},
		},
	},
}

// FormScenarios provides common form interaction sequences
var FormScenarios = []Scenario{
	{
		Name: "fill_submit_clear",
		Actions: []Action{
			{Name: "setField", Data: map[string]interface{}{"field": "name", "value": "John"}},
			{Name: "setField", Data: map[string]interface{}{"field": "email", "value": "john@example.com"}},
			{Name: "submit"},
			{Name: "clear"},
		},
	},
	{
		Name: "edit_cancel_edit_submit",
		Actions: []Action{
			{Name: "setField", Data: map[string]interface{}{"field": "name", "value": "Draft"}},
			{Name: "cancel"},
			{Name: "setField", Data: map[string]interface{}{"field": "name", "value": "Final"}},
			{Name: "submit"},
		},
	},
	{
		Name: "validation_retry",
		Actions: []Action{
			{Name: "setField", Data: map[string]interface{}{"field": "email", "value": "invalid"}},
			{Name: "submit"}, // Should fail validation
			{Name: "setField", Data: map[string]interface{}{"field": "email", "value": "valid@example.com"}},
			{Name: "submit"}, // Should succeed
		},
	},
}

// ListScenarios provides common list manipulation sequences
var ListScenarios = []Scenario{
	{
		Name: "insert_at_positions",
		Actions: []Action{
			{Name: "append", Data: map[string]interface{}{"item": "first"}},
			{Name: "prepend", Data: map[string]interface{}{"item": "new first"}},
			{Name: "insertAt", Data: map[string]interface{}{"item": "middle", "index": 1}},
		},
	},
	{
		Name: "reorder",
		Actions: []Action{
			{Name: "append", Data: map[string]interface{}{"item": "A"}},
			{Name: "append", Data: map[string]interface{}{"item": "B"}},
			{Name: "append", Data: map[string]interface{}{"item": "C"}},
			{Name: "moveUp", Data: map[string]interface{}{"index": 2}},   // C moves up
			{Name: "moveDown", Data: map[string]interface{}{"index": 0}}, // A moves down
		},
	},
	{
		Name: "filter_operations",
		Actions: []Action{
			{Name: "append", Data: map[string]interface{}{"item": "apple"}},
			{Name: "append", Data: map[string]interface{}{"item": "banana"}},
			{Name: "append", Data: map[string]interface{}{"item": "apricot"}},
			{Name: "setFilter", Data: map[string]interface{}{"filter": "ap"}},
			{Name: "clearFilter"},
		},
	},
}

// PaginationScenarios provides common pagination sequences
var PaginationScenarios = []Scenario{
	{
		Name: "navigate_pages",
		Actions: []Action{
			{Name: "nextPage"},
			{Name: "nextPage"},
			{Name: "previousPage"},
			{Name: "goToPage", Data: map[string]interface{}{"page": 5}},
			{Name: "firstPage"},
			{Name: "lastPage"},
		},
	},
	{
		Name: "change_page_size",
		Actions: []Action{
			{Name: "setPageSize", Data: map[string]interface{}{"size": 25}},
			{Name: "nextPage"},
			{Name: "setPageSize", Data: map[string]interface{}{"size": 10}},
		},
	},
}

// AuthScenarios provides common authentication sequences
var AuthScenarios = []Scenario{
	{
		Name: "login_logout",
		Actions: []Action{
			{Name: "login", Data: map[string]interface{}{"username": "user", "password": "pass"}},
			{Name: "logout"},
		},
	},
	{
		Name: "failed_login_retry",
		Actions: []Action{
			{Name: "login", Data: map[string]interface{}{"username": "user", "password": "wrong"}},
			{Name: "login", Data: map[string]interface{}{"username": "user", "password": "wrong"}},
			{Name: "login", Data: map[string]interface{}{"username": "user", "password": "correct"}},
		},
	},
	{
		Name: "session_refresh",
		Actions: []Action{
			{Name: "login", Data: map[string]interface{}{"username": "user", "password": "pass"}},
			{Name: "refreshSession"},
			{Name: "performAction"},
		},
	},
}

// ModalScenarios provides common modal interaction sequences
var ModalScenarios = []Scenario{
	{
		Name: "open_close",
		Actions: []Action{
			{Name: "openModal", Data: map[string]interface{}{"modal": "settings"}},
			{Name: "closeModal"},
		},
	},
	{
		Name: "open_submit_close",
		Actions: []Action{
			{Name: "openModal", Data: map[string]interface{}{"modal": "edit"}},
			{Name: "updateField", Data: map[string]interface{}{"field": "name", "value": "New Name"}},
			{Name: "submitModal"},
		},
	},
	{
		Name: "nested_modals",
		Actions: []Action{
			{Name: "openModal", Data: map[string]interface{}{"modal": "parent"}},
			{Name: "openModal", Data: map[string]interface{}{"modal": "child"}},
			{Name: "closeModal"},
			{Name: "closeModal"},
		},
	},
}

// CRUDScenarios provides common CRUD operation sequences
var CRUDScenarios = []Scenario{
	{
		Name: "create_read_update_delete",
		Actions: []Action{
			{Name: "create", Data: map[string]interface{}{"name": "Item 1"}},
			{Name: "read", Data: map[string]interface{}{"id": 1}},
			{Name: "update", Data: map[string]interface{}{"id": 1, "name": "Updated Item"}},
			{Name: "delete", Data: map[string]interface{}{"id": 1}},
		},
	},
	{
		Name: "bulk_create_bulk_delete",
		Actions: []Action{
			{Name: "create", Data: map[string]interface{}{"name": "A"}},
			{Name: "create", Data: map[string]interface{}{"name": "B"}},
			{Name: "create", Data: map[string]interface{}{"name": "C"}},
			{Name: "selectAll"},
			{Name: "bulkDelete"},
		},
	},
}

// StressScenarios provides high-volume operation sequences
var StressScenarios = []Scenario{
	{
		Name:    "100_adds",
		Actions: generateNumberedAdds(100),
	},
	{
		Name: "rapid_toggle",
		Actions: CycleActions([]Action{
			{Name: "add", Data: map[string]interface{}{"title": "item"}},
			{Name: "toggle", Data: map[string]interface{}{"index": 0}},
		}, 50),
	},
}

// generateNumberedAdds creates n add actions with numbered titles
func generateNumberedAdds(n int) []Action {
	actions := make([]Action, n)
	for i := 0; i < n; i++ {
		actions[i] = Action{
			Name: "add",
			Data: map[string]interface{}{"title": fmt.Sprintf("Item %d", i+1)},
		}
	}
	return actions
}
