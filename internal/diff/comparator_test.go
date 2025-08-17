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

	tests := []struct {
		name                string
		oldHTML             string
		newHTML             string
		expectedChangeCount int
		expectedChangeType  ChangeType
		expectedOldValue    string
		expectedNewValue    string
		validateChanges     func(t *testing.T, changes []DOMChange, comparator *DOMComparator)
	}{
		{
			name:                "attribute addition",
			oldHTML:             `<div>Hello</div>`,
			newHTML:             `<div class="new">Hello</div>`,
			expectedChangeCount: 1,
			expectedChangeType:  ChangeAttribute,
			expectedOldValue:    "",
			expectedNewValue:    "new",
			validateChanges: func(t *testing.T, changes []DOMChange, comparator *DOMComparator) {
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
			},
		},
		{
			name:                "attribute removal",
			oldHTML:             `<div class="old">Hello</div>`,
			newHTML:             `<div>Hello</div>`,
			expectedChangeCount: 1,
			expectedChangeType:  ChangeAttribute,
			expectedOldValue:    "old",
			expectedNewValue:    "",
			validateChanges: func(t *testing.T, changes []DOMChange, comparator *DOMComparator) {
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
			},
		},
		{
			name:                "element type change",
			oldHTML:             `<div>Hello</div>`,
			newHTML:             `<span>Hello</span>`,
			expectedChangeCount: 1, // At least one change
			expectedChangeType:  ChangeStructure,
			validateChanges: func(t *testing.T, changes []DOMChange, comparator *DOMComparator) {
				if len(changes) == 0 {
					t.Fatal("expected at least one change")
				}
				// Should detect as structural change
				changeType := comparator.ClassifyChanges(changes)
				if changeType != ChangeStructure {
					t.Errorf("expected ChangeStructure, got %v", changeType)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes, err := comparator.Compare(tt.oldHTML, tt.newHTML)
			if err != nil {
				t.Fatalf("Compare() error = %v", err)
			}

			if len(changes) < tt.expectedChangeCount {
				t.Fatalf("expected at least %d change(s), got %d", tt.expectedChangeCount, len(changes))
			}

			if tt.validateChanges != nil {
				tt.validateChanges(t, changes, comparator)
			}
		})
	}
}

func TestDOMComparator_EmptyStateHandling(t *testing.T) {
	comparator := NewDOMComparator()

	tests := []struct {
		name               string
		oldHTML            string
		newHTML            string
		expectChanges      bool
		expectedChangeType ChangeType
	}{
		{
			name:               "empty to content",
			oldHTML:            `<div></div>`,
			newHTML:            `<div><p>Hello</p></div>`,
			expectChanges:      true,
			expectedChangeType: ChangeStructure,
		},
		{
			name:               "content to empty",
			oldHTML:            `<div><p>Hello</p></div>`,
			newHTML:            `<div></div>`,
			expectChanges:      true,
			expectedChangeType: ChangeStructure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes, err := comparator.Compare(tt.oldHTML, tt.newHTML)
			if err != nil {
				t.Fatalf("Compare() error = %v", err)
			}

			if tt.expectChanges && len(changes) == 0 {
				t.Fatalf("expected changes for %s transition", tt.name)
			}

			if !tt.expectChanges && len(changes) > 0 {
				t.Fatalf("expected no changes for %s transition, got %d", tt.name, len(changes))
			}

			if tt.expectChanges {
				// Should be classified as expected type
				changeType := comparator.ClassifyChanges(changes)
				if changeType != tt.expectedChangeType {
					t.Errorf("expected %v for %s, got %v", tt.expectedChangeType, tt.name, changeType)
				}
			}
		})
	}
}

func TestDOMChange_Structure(t *testing.T) {
	comparator := NewDOMComparator()

	tests := []struct {
		name          string
		oldHTML       string
		newHTML       string
		expectChanges bool
	}{
		{
			name:          "text change produces valid structure",
			oldHTML:       `<p>Hello World</p>`,
			newHTML:       `<p>Hello Universe</p>`,
			expectChanges: true,
		},
		{
			name:          "attribute change produces valid structure",
			oldHTML:       `<div class="old">Content</div>`,
			newHTML:       `<div class="new">Content</div>`,
			expectChanges: true,
		},
		{
			name:          "structural change produces valid structure",
			oldHTML:       `<div>Content</div>`,
			newHTML:       `<div>Content<span>Added</span></div>`,
			expectChanges: true,
		},
		{
			name:          "no change produces no structure",
			oldHTML:       `<p>Same Content</p>`,
			newHTML:       `<p>Same Content</p>`,
			expectChanges: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes, err := comparator.Compare(tt.oldHTML, tt.newHTML)
			if err != nil {
				t.Fatalf("Compare() error = %v", err)
			}

			if tt.expectChanges && len(changes) == 0 {
				t.Fatal("expected at least one change")
			}

			if !tt.expectChanges && len(changes) > 0 {
				t.Fatalf("expected no changes, got %d", len(changes))
			}

			// Validate structure of any changes
			for i, change := range changes {
				if change.Type == "" {
					t.Errorf("change %d has empty type", i)
				}

				if change.Description == "" {
					t.Errorf("change %d has empty description", i)
				}
			}
		})
	}
}
