package parse

import (
	"strings"
	"testing"
)

// TestBuildExecData_UTF8Capitalization tests UTF8-safe capitalization of variable names.
func TestBuildExecData_UTF8Capitalization(t *testing.T) {
	tests := []struct {
		name     string
		varName  string
		wantKey  string
		varValue interface{}
	}{
		{"ascii lowercase", "name", "Name", "John"},
		{"ascii single char", "a", "A", 1},
		{"utf8 ñ", "ñame", "Ñame", "José"},
		{"utf8 ü", "über", "Über", "value"},
		{"already upper", "Name", "Name", "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varCtx := &varContext{
				parent: map[string]interface{}{},
				vars:   newOrderedVars(),
				dot:    map[string]interface{}{},
			}
			varCtx.vars.Set(tt.varName, tt.varValue)

			expr := "{{$" + tt.varName + "}}"
			transformed, execData, err := buildExecData(expr, varCtx)
			if err != nil {
				t.Fatalf("buildExecData failed: %v", err)
			}

			if _, ok := execData[tt.wantKey]; !ok {
				t.Errorf("execData missing key %q, got keys: %v", tt.wantKey, execData)
			}

			if execData[tt.wantKey] != tt.varValue {
				t.Errorf("execData[%q] = %v, want %v", tt.wantKey, execData[tt.wantKey], tt.varValue)
			}

			if strings.Contains(transformed, "$"+tt.varName) {
				t.Errorf("transformed still contains $%s: %s", tt.varName, transformed)
			}
		})
	}
}

// TestBuildExecData_PartialMatchPrevention tests that longer variable names are
// replaced before shorter ones to prevent partial matches.
func TestBuildExecData_PartialMatchPrevention(t *testing.T) {
	tests := []struct {
		name      string
		vars      map[string]interface{}
		expr      string
		wantInMap []string
	}{
		{
			name:      "$c and $col",
			vars:      map[string]interface{}{"c": "short", "col": "long"},
			expr:      "{{$col}}",
			wantInMap: []string{"Col"},
		},
		{
			name:      "$item and $itemCount",
			vars:      map[string]interface{}{"item": "x", "itemCount": 5},
			expr:      "{{$itemCount}}",
			wantInMap: []string{"ItemCount"},
		},
		{
			name:      "both $c and $col used",
			vars:      map[string]interface{}{"c": "short", "col": "long"},
			expr:      "{{$c}} {{$col}}",
			wantInMap: []string{"C", "Col"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varCtx := &varContext{
				parent: map[string]interface{}{},
				vars:   newOrderedVars(),
				dot:    map[string]interface{}{},
			}
			for k, v := range tt.vars {
				varCtx.vars.Set(k, v)
			}

			transformed, execData, err := buildExecData(tt.expr, varCtx)
			if err != nil {
				t.Fatalf("buildExecData failed: %v", err)
			}

			for _, key := range tt.wantInMap {
				if _, ok := execData[key]; !ok {
					t.Errorf("execData missing key %q, got: %v", key, execData)
				}
			}

			// Transformed expression should not contain any $-prefixed variable references
			for varName := range tt.vars {
				if strings.Contains(transformed, "$"+varName) {
					t.Errorf("transformed still contains $%s: %s", varName, transformed)
				}
			}
		})
	}
}

// TestBuildExecData_RootVariable tests root variable ($.) handling.
func TestBuildExecData_RootVariable(t *testing.T) {
	parentData := map[string]interface{}{"RootField": "rootValue"}

	varCtx := &varContext{
		parent: parentData,
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}

	transformed, execData, err := buildExecData("{{$.RootField}}", varCtx)
	if err != nil {
		t.Fatalf("buildExecData failed: %v", err)
	}

	if execData["RootData"] == nil {
		t.Error("execData[RootData] should be set to parent data")
	}

	if !strings.Contains(transformed, ".RootData.") {
		t.Errorf("transformed should contain .RootData., got: %s", transformed)
	}
}

// TestBuildExecData_MixedRootAndVars tests expressions with both root and named variables.
func TestBuildExecData_MixedRootAndVars(t *testing.T) {
	varCtx := &varContext{
		parent: map[string]interface{}{"Title": "Root Title"},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}
	varCtx.vars.Set("x", 42)

	_, execData, err := buildExecData("{{$x}} {{$.Title}}", varCtx)
	if err != nil {
		t.Fatalf("buildExecData failed: %v", err)
	}

	if execData["X"] != 42 {
		t.Errorf("execData[X] = %v, want 42", execData["X"])
	}
	if execData["RootData"] == nil {
		t.Error("execData[RootData] should be set")
	}
}

// TestBuildExecData_DotContextMerging tests that dot context fields are merged
// into execData so expressions like {{$c.Method .Type}} can access .Type.
func TestBuildExecData_DotContextMerging(t *testing.T) {
	t.Run("map dot context", func(t *testing.T) {
		varCtx := &varContext{
			parent: map[string]interface{}{},
			vars:   newOrderedVars(),
			dot:    map[string]interface{}{"Type": "button", "Size": "large"},
		}
		varCtx.vars.Set("c", "controller")

		_, execData, err := buildExecData("{{$c}} {{.Type}}", varCtx)
		if err != nil {
			t.Fatalf("buildExecData failed: %v", err)
		}

		if execData["C"] != "controller" {
			t.Errorf("execData[C] = %v, want 'controller'", execData["C"])
		}
		if execData["Type"] != "button" {
			t.Errorf("execData[Type] = %v, want 'button'", execData["Type"])
		}
		if execData["Size"] != "large" {
			t.Errorf("execData[Size] = %v, want 'large'", execData["Size"])
		}
	})

	t.Run("struct dot context", func(t *testing.T) {
		type DotData struct {
			Type string
			Size string
		}
		varCtx := &varContext{
			parent: map[string]interface{}{},
			vars:   newOrderedVars(),
			dot:    DotData{Type: "input", Size: "small"},
		}
		varCtx.vars.Set("c", "controller")

		_, execData, err := buildExecData("{{$c}} {{.Type}}", varCtx)
		if err != nil {
			t.Fatalf("buildExecData failed: %v", err)
		}

		if execData["Type"] != "input" {
			t.Errorf("execData[Type] = %v, want 'input'", execData["Type"])
		}
	})

	t.Run("variables take precedence over dot", func(t *testing.T) {
		varCtx := &varContext{
			parent: map[string]interface{}{},
			vars:   newOrderedVars(),
			dot:    map[string]interface{}{"Name": "from-dot"},
		}
		// Variable "name" capitalizes to "Name", same as dot field
		varCtx.vars.Set("name", "from-var")

		_, execData, err := buildExecData("{{$name}}", varCtx)
		if err != nil {
			t.Fatalf("buildExecData failed: %v", err)
		}

		// Variable should take precedence
		if execData["Name"] != "from-var" {
			t.Errorf("execData[Name] = %v, want 'from-var' (variable should win)", execData["Name"])
		}
	})
}

// TestBuildExecData_NoVarsNoRoot tests expression with no variables and no root.
func TestBuildExecData_NoVarsNoRoot(t *testing.T) {
	varCtx := &varContext{
		parent: map[string]interface{}{},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{"Field": "value"},
	}
	varCtx.vars.Set("unused", "nope")

	transformed, execData, err := buildExecData("{{.Field}}", varCtx)
	if err != nil {
		t.Fatalf("buildExecData failed: %v", err)
	}

	// Expression unchanged since no $ references
	if transformed != "{{.Field}}" {
		t.Errorf("transformed = %q, want unchanged expression", transformed)
	}

	// Dot fields should still be merged
	if execData["Field"] != "value" {
		t.Errorf("execData[Field] = %v, want 'value'", execData["Field"])
	}
}

// TestBuildExecData_RangeVarPrefixConvention tests that the range variable prefix
// convention is preserved: vars stored with "$" prefix are NOT matched.
func TestBuildExecData_RangeVarPrefixConvention(t *testing.T) {
	varCtx := &varContext{
		parent: map[string]interface{}{},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}
	// Stored with $ prefix (as executeRangeBodyWithVars does)
	varCtx.vars.Set("$v", "value")

	transformed, execData, err := buildExecData("{{$v}}", varCtx)
	if err != nil {
		t.Fatalf("buildExecData failed: %v", err)
	}

	// "$v" stored with $ means search pattern is "$$v" which won't match "{{$v}}"
	// So the expression should remain unchanged and execData should not have "$v" mapped
	if _, ok := execData["$V"]; ok {
		t.Error("execData should NOT contain key for $-prefixed var (would match $$v)")
	}
	if transformed != "{{$v}}" {
		t.Errorf("expression should be unchanged for $-prefixed vars, got: %s", transformed)
	}
}

// TestEvaluateActionWithVars_ErrorPropagation tests that errors are properly propagated
// instead of being embedded as "ERROR: ..." strings in the output.
func TestEvaluateActionWithVars_ErrorPropagation(t *testing.T) {
	varCtx := &varContext{
		parent: map[string]interface{}{},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}
	varCtx.vars.Set("x", "value")
	ctx := &Context{}

	// Use an expression that will fail template parsing (unbalanced braces)
	_, err := evaluateActionWithVars("{{$x", varCtx, ctx)

	// The old code would return "ERROR: ..." as a string with nil error.
	// The new code should return an actual error.
	if err == nil {
		t.Error("expected error for malformed template, got nil")
	}

	// Verify no "ERROR:" prefix in error message (that was the old pattern)
	if err != nil && strings.HasPrefix(err.Error(), "ERROR:") {
		t.Errorf("error should not use old ERROR: prefix, got: %v", err)
	}
}

// TestEvaluateActionWithVars_DotContextAccess tests that after the fix,
// expressions mixing variables and dot fields work correctly.
func TestEvaluateActionWithVars_DotContextAccess(t *testing.T) {
	varCtx := &varContext{
		parent: map[string]interface{}{},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{"Type": "button"},
	}
	varCtx.vars.Set("cls", "primary")
	ctx := &Context{}

	result, err := evaluateActionWithVars("{{$cls}} {{.Type}}", varCtx, ctx)
	if err != nil {
		t.Fatalf("evaluateActionWithVars failed: %v", err)
	}

	if result != "primary button" {
		t.Errorf("Expected 'primary button', got: %v", result)
	}
}
