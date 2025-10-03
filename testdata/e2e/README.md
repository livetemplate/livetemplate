# End-to-End Test Case: Complete Rendering Sequence

This directory contains the complete test case demonstrating LiveTemplate's rendering sequence from initial full page render through subsequent tree-based updates.

## Test Scenario

A task management application with:
- Dynamic counter that affects UI status
- Todo list with priorities and completion states  
- Real-time statistics and completion rate calculation
- Session tracking

## Files

### Input Template
- **`input.tmpl`** - Complete HTML template with Go template constructs
  - Full HTML document with DOCTYPE, head, styles
  - Conditional rendering based on counter value
  - Range iteration over todo items
  - Nested template logic and calculations

### Expected Outputs

#### 1. Initial Full Render
- **`rendered_00_initial.html`** - Complete HTML page with wrapper injection
  - Contains `data-lvt-id` attribute for update targeting
  - Shows initial state: 0 todos, counter=1, "Low Activity" status

#### 2. Add Todos Data & Update
- **`add_todos.json`** - Simple JSON array of new todo items to be added
  - Contains 3 todo items with text, completed status, and priority
  - This represents the data that would be sent to add new todos
- **`rendered_01_add_todos.html`** - Complete HTML page after adding todos
  - Shows state: 3 todos (1 completed), counter=3, still "Low Activity"
  - Full page state for visual review and comparison
- **`update_01_add_todos.json`** - LiveTemplate tree structure after adding todos
  - **First update**: Includes static structure (`"s"` key) for client-side caching
  - Dynamic segments (numbered keys `"0"`, `"1"`, etc.) contain changed values

Key dynamic segments in update_01_add_todos.json:
- `"1"`: Counter value "3"
- `"4"`: Total todos "3"  
- `"5"`: Completed count "1"
- `"6"`: Remaining count "2"
- `"7"`: Completion rate "33%"
- `"8"`: Complete todo list HTML

#### 3. Remove Todo Update
- **`rendered_02_remove_todo.html`** - Complete HTML page after removing todo
  - Shows state: 2 todos (1 completed, 1 remaining), counter=8, now "High Activity"
  - Visual diff shows "Build live updates" todo was removed and status changed to active
- **`update_02_remove_todo.json`** - Dynamics-only update (cache-aware)
  - **Second update**: NO static structure (`"s"` key) - demonstrates caching
  - Only changed dynamic values transmitted (client already has static structure)
  - Demonstrates todo removal and counter-based status change

Key changes in update_02_remove_todo.json:
- `"2"`: Counter value "8" 
- `"3"`: Status class "active" (was "inactive")
- `"4"`: Status text "High Activity" (was "Low Activity")
- `"5"`: Total todos "2" (reduced from 3 - one todo removed)
- `"6"`: Completed count "1" (unchanged)
- `"7"`: Remaining count "1" (reduced from 2)  
- `"8"`: Completion rate "50%" (changed from 33%)
- `"9"`: Updated todo list with removed item (only 2 todos remain)

## Tree Structure Explanation

The LiveTemplate tree structure uses:
- **`"s"` array**: Static HTML segments (sent only when needed)
- **Numbered keys**: Dynamic content that changes between renders
- **Efficient updates**: Only changed values transmitted after initial render

### Static/Dynamic Separation Example
```
Template: <p>Count: {{.Counter}}</p>
Tree: {
  "s": ["<p>Count: ", "</p>"],
  "0": "42"
}
```

### Range Handling Example  
```
Template: {{range .Items}}<li>{{.Text}}</li>{{end}}
Tree: {
  "s": ["", ""],
  "0": "<li>Item 1</li><li>Item 2</li>"
}
```

## Performance Characteristics

- **Initial render**: 1782 bytes (full HTML)
- **First update**: 695 bytes (includes static structure)
- **Subsequent updates**: 244 bytes (dynamics only) 
- **No-change updates**: 2 bytes (minimal JSON)
- **Average update time**: <120μs

## Bandwidth Savings

Compared to re-sending full HTML:
- **Update 1**: 61% savings (695 vs 1782 bytes)
- **Update 2**: 86% savings (244 vs 1782 bytes)
- **Static structure caching**: 65% reduction from first to second update (244 vs 695 bytes)
- **Total savings**: 85%+ in typical usage patterns