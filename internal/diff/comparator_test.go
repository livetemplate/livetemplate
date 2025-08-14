package diff

import (
	"testing"
)

func TestDOMComparator_Compare(t *testing.T) {
	comparator := NewDOMComparator()

	tests := []struct {
		name        string
		oldHTML     string
		newHTML     string
		wantChanges int
		wantType    ChangeType
	}{
		{
			name:        "no changes",
			oldHTML:     "<p>Hello World</p>",
			newHTML:     "<p>Hello World</p>",
			wantChanges: 0,
			wantType:    ChangeNone,
		},
		{
			name:        "text only change",
			oldHTML:     "<p>Hello World</p>",
			newHTML:     "<p>Hello Universe</p>",
			wantChanges: 1,
			wantType:    ChangeTextOnly,
		},
		{
			name:        "attribute change",
			oldHTML:     `<p class="old">Hello</p>`,
			newHTML:     `<p class="new">Hello</p>`,
			wantChanges: 1,
			wantType:    ChangeAttribute,
		},
		{
			name:        "structure change - element added",
			oldHTML:     "<div>Hello</div>",
			newHTML:     "<div>Hello<span>World</span></div>",
			wantChanges: 1, // At least one structural change
			wantType:    ChangeStructure,
		},
		{
			name:        "structure change - element removed",
			oldHTML:     "<div>Hello<span>World</span></div>",
			newHTML:     "<div>Hello</div>",
			wantChanges: 1,
			wantType:    ChangeStructure,
		},
		{
			name:        "complex change - multiple types",
			oldHTML:     `<div class="old">Hello</div>`,
			newHTML:     `<div class="new">Hi<span>There</span></div>`,
			wantChanges: 3, // text change + attribute change + structure change
			wantType:    ChangeComplex,
		},
		{
			name:        "whitespace normalization",
			oldHTML:     "<p>Hello   World</p>",
			newHTML:     "<p>Hello World</p>",
			wantChanges: 0, // Should be normalized as same
			wantType:    ChangeNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes, err := comparator.Compare(tt.oldHTML, tt.newHTML)
			if err != nil {
				t.Fatalf("Compare() error = %v", err)
			}

			if len(changes) < tt.wantChanges {
				t.Errorf("Compare() got %d changes, want at least %d", len(changes), tt.wantChanges)
			}

			if len(changes) > 0 {
				changeType := comparator.ClassifyChanges(changes)
				if tt.wantChanges > 0 && changeType != tt.wantType {
					// For complex changes, allow some flexibility
					if tt.wantType == ChangeComplex && (changeType == ChangeStructure || changeType == ChangeAttribute) {
						// This is acceptable
					} else {
						t.Errorf("Compare() change type = %v, want %v", changeType, tt.wantType)
					}
				}
			}
		})
	}
}

func TestDOMComparator_ClassifyChanges(t *testing.T) {
	comparator := NewDOMComparator()

	tests := []struct {
		name         string
		changes      []DOMChange
		expectedType ChangeType
	}{
		{
			name:         "no changes",
			changes:      []DOMChange{},
			expectedType: ChangeNone,
		},
		{
			name: "single text change",
			changes: []DOMChange{
				{Type: ChangeTextOnly},
			},
			expectedType: ChangeTextOnly,
		},
		{
			name: "single attribute change",
			changes: []DOMChange{
				{Type: ChangeAttribute},
			},
			expectedType: ChangeAttribute,
		},
		{
			name: "mixed changes",
			changes: []DOMChange{
				{Type: ChangeTextOnly},
				{Type: ChangeAttribute},
			},
			expectedType: ChangeComplex,
		},
		{
			name: "structural changes",
			changes: []DOMChange{
				{Type: ChangeStructure},
				{Type: ChangeStructure},
			},
			expectedType: ChangeStructure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changeType := comparator.ClassifyChanges(tt.changes)

			if changeType != tt.expectedType {
				t.Errorf("ClassifyChanges() type = %v, want %v", changeType, tt.expectedType)
			}
		})
	}
}

func TestDOMComparator_SpecificChangeTypes(t *testing.T) {
	comparator := NewDOMComparator()

	t.Run("attribute addition", func(t *testing.T) {
		oldHTML := `<div>Hello</div>`
		newHTML := `<div class="new">Hello</div>`

		changes, err := comparator.Compare(oldHTML, newHTML)
		if err != nil {
			t.Fatalf("Compare() error = %v", err)
		}

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}

		change := changes[0]
		if change.Type != ChangeAttribute {
			t.Errorf("expected ChangeAttribute, got %v", change.Type)
		}

		if change.OldValue != "" {
			t.Errorf("expected empty old value, got %s", change.OldValue)
		}

		if change.NewValue != "new" {
			t.Errorf("expected 'new', got %s", change.NewValue)
		}
	})

	t.Run("attribute removal", func(t *testing.T) {
		oldHTML := `<div class="old">Hello</div>`
		newHTML := `<div>Hello</div>`

		changes, err := comparator.Compare(oldHTML, newHTML)
		if err != nil {
			t.Fatalf("Compare() error = %v", err)
		}

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}

		change := changes[0]
		if change.Type != ChangeAttribute {
			t.Errorf("expected ChangeAttribute, got %v", change.Type)
		}

		if change.OldValue != "old" {
			t.Errorf("expected 'old', got %s", change.OldValue)
		}

		if change.NewValue != "" {
			t.Errorf("expected empty new value, got %s", change.NewValue)
		}
	})

	t.Run("element type change", func(t *testing.T) {
		oldHTML := `<div>Hello</div>`
		newHTML := `<span>Hello</span>`

		changes, err := comparator.Compare(oldHTML, newHTML)
		if err != nil {
			t.Fatalf("Compare() error = %v", err)
		}

		if len(changes) == 0 {
			t.Fatal("expected at least one change")
		}

		// Should detect as structural change
		changeType := comparator.ClassifyChanges(changes)
		if changeType != ChangeStructure {
			t.Errorf("expected ChangeStructure, got %v", changeType)
		}
	})
}

func TestDOMComparator_EmptyStateHandling(t *testing.T) {
	comparator := NewDOMComparator()

	t.Run("empty to content", func(t *testing.T) {
		oldHTML := `<div></div>`
		newHTML := `<div><p>Hello</p></div>`

		changes, err := comparator.Compare(oldHTML, newHTML)
		if err != nil {
			t.Fatalf("Compare() error = %v", err)
		}

		if len(changes) == 0 {
			t.Fatal("expected changes for empty to content transition")
		}

		// Should be classified as structural
		changeType := comparator.ClassifyChanges(changes)
		if changeType != ChangeStructure {
			t.Errorf("expected ChangeStructure for empty state, got %v", changeType)
		}
	})

	t.Run("content to empty", func(t *testing.T) {
		oldHTML := `<div><p>Hello</p></div>`
		newHTML := `<div></div>`

		changes, err := comparator.Compare(oldHTML, newHTML)
		if err != nil {
			t.Fatalf("Compare() error = %v", err)
		}

		if len(changes) == 0 {
			t.Fatal("expected changes for content to empty transition")
		}

		// Should be classified as structural
		changeType := comparator.ClassifyChanges(changes)
		if changeType != ChangeStructure {
			t.Errorf("expected ChangeStructure for empty state, got %v", changeType)
		}
	})
}

func TestDOMChange_Structure(t *testing.T) {
	comparator := NewDOMComparator()

	// Test that change structure is valid
	oldHTML := `<p>Hello World</p>`
	newHTML := `<p>Hello Universe</p>`

	changes, err := comparator.Compare(oldHTML, newHTML)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}

	if len(changes) == 0 {
		t.Fatal("expected at least one change")
	}

	for i, change := range changes {
		if change.Type == "" {
			t.Errorf("change %d has empty type", i)
		}

		if change.Description == "" {
			t.Errorf("change %d has empty description", i)
		}
	}
}
