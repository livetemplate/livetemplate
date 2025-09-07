# Template Construct Analysis - Low Level Design (LLD)

## Quick Reference Table

| Template Construct | Example | Tree-Based | Legacy Fallback | Selected Strategy | Client Structure |
|-------------------|---------|------------|------------------|-------------------|------------------|
| **Simple Fields** | `{{.Field}}` | ✅ OPTIMAL | ✅ AVAILABLE | **Tree-Based** | `{"s":["<p>","</p>"],"0":"value"}` |
| **Nested Fields** | `{{.User.Name}}` | ✅ OPTIMAL | ✅ AVAILABLE | **Tree-Based** | `{"s":["Hi ","!"],"0":"John"}` |
| **Method Calls** | `{{.GetName}}` | ✅ OPTIMAL | ✅ AVAILABLE | **Tree-Based** | `{"s":["Result: ",""],"0":"methodValue"}` |
| **Comments** | `{{/* comment */}}` | ✅ IGNORE | ✅ IGNORE | **Tree-Based** | `{"s":["content"]}` |
| **Template Definitions** | `{{define "name"}}` | ✅ IGNORE | ✅ IGNORE | **Tree-Based** | `(parse-time only)` |
| **Simple Conditionals** | `{{if .Show}}Hi{{end}}` | ✅ SUPPORTED | ⚠️ COMPLEX | **Tree-Based** | `{"s":["",""],"0":{"s":["Hi"]}}` |
| **If-Else** | `{{if .A}}X{{else}}Y{{end}}` | ✅ SUPPORTED | ⚠️ COMPLEX | **Tree-Based** | `{"s":["",""],"0":{"s":["X"]}}` |
| **Simple Ranges** | `{{range .Items}}{{.}}{{end}}` | ✅ SUPPORTED | ❌ DIFFICULT | **Tree-Based** | `{"s":["",""],"0":[{"s":["",""],"0":"A"}]}` |
| **Complex Conditionals** | `{{if eq .A .B}}...{{end}}` | ✅ SUPPORTED | ⚠️ COMPLEX | **Tree-Based** | `{"s":["",""],"0":{"s":["result"]}}` |
| **Variables** | `{{$var := .Field}}` | 🔄 PLANNED | ✅ SUITABLE | **Legacy Fallback** | `(full HTML replacement)` |
| **Variable Access** | `{{$var}}` | 🔄 PLANNED | ✅ SUITABLE | **Legacy Fallback** | `(full HTML replacement)` |
| **Loop Control** | `{{break}}`, `{{continue}}` | ❌ COMPLEX | ✅ SUITABLE | **Legacy Fallback** | `(full HTML replacement)` |
| **Context With** | `{{with .User}}...{{end}}` | ✅ SUPPORTED | ✅ SUITABLE | **Tree-Based** | `{"s":["",""],"0":{"s":["content"]}}` |
| **Template Invocation** | `{{template "name" .}}` | 🔄 FUTURE | ✅ SUITABLE | **Legacy Fallback** | `(full HTML replacement)` |
| **Block Definition** | `{{block "name" .}}...{{end}}` | 🔄 FUTURE | ✅ SUITABLE | **Legacy Fallback** | `(full HTML replacement)` |
| **Logical Functions** | `{{if and .A .B}}` | ✅ SUPPORTED | ✅ SUITABLE | **Tree-Based** | `{"s":["",""],"0":{"s":["result"]}}` |
| **Utility Functions** | `{{len .Items}}` | ✅ SUPPORTED | ✅ SUITABLE | **Tree-Based** | `{"s":["Count: ",""],"0":"3"}` |
| **Pipelines** | `{{.Name \| upper}}` | 🔄 PLANNED | ✅ SUITABLE | **Legacy Fallback** | `(full HTML replacement)` |

### Legend
- ✅ **OPTIMAL**: Most efficient tree-based optimization
- ✅ **SUPPORTED**: Tree-based optimization available
- ✅ **AVAILABLE**: Legacy fallback works well
- 🔄 **PLANNED**: Implementation planned for future phases
- 🔄 **FUTURE**: Advanced feature for later versions
- ⚠️ **COMPLEX**: Requires complex processing
- ❌ **DIFFICULT**: Challenging for legacy approaches
- **IGNORE**: Construct ignored during processing

## Overview

This document provides a comprehensive analysis of ALL Go template constructs from `text/template` package and determines their suitability for tree-based optimization. Each construct is evaluated for the unified tree-based strategy with appropriate fallback handling.

## Strategy Selection Framework

### Tree-Based Optimization Capability
1. **Tree-Based Strategy**: Single unified approach using hierarchical template parsing
   - Template boundary analysis for static/dynamic separation  
   - Client-side caching for static content
   - Phoenix LiveView compatible structures
   - Handles simple fields, conditionals, ranges, and nested constructs
   - Falls back to full re-rendering for unsupported edge cases

### Decision Matrix

## Template Construct Analysis

### 1. Comments (`{{/* comment */}}`)

**Construct**: `{{/* comment */}}`

**Analysis**:
- Comments do not appear in rendered HTML
- No impact on fragment generation
- Should be ignored during parsing

**Strategy Decision**: 
- ✅ **Tree-Based**: IGNORE - Comments don't affect output and don't impact tree structure

**Implementation**: Skip comments during boundary parsing.

### 2. Simple Pipeline Output (`{{.Field}}`)

**Construct**: `{{.Field}}`, `{{.Nested.Field}}`, `{{.Method}}`

**Analysis**:
- Direct field/method access with predictable output
- Static HTML boundaries around dynamic values
- Deterministic evaluation from data context

**Strategy Decision**:
- ✅ **Tree-Based**: OPTIMAL - Perfect fit for static/dynamic boundary separation with client-side caching

**Implementation**: Primary target for static/dynamic strategy.

### 3. Variables (`{{$var := .Field}}`, `{{$var}}`)

**Construct**: 
- `{{$var := pipeline}}` - Declaration
- `{{$var = pipeline}}` - Reassignment
- `{{$var}}` - Access

**Analysis**:
- Variable declaration adds complexity to evaluation
- Variable scope affects template context
- Multiple assignments create stateful evaluation

**Strategy Decision**:
- 🔄 **Tree-Based**: PLANNED - Variable scoping planned for future implementation  
- ✅ **Fallback**: SUITABLE - Full template re-rendering handles variables correctly

**Reasoning**: Variable scoping and reassignment make static boundary detection unreliable. Variables create state that affects subsequent template evaluation.

### 4. Conditional Blocks (`{{if}}`, `{{else}}`, `{{end}}`)

**Constructs**:
- `{{if pipeline}} T1 {{end}}`
- `{{if pipeline}} T1 {{else}} T0 {{end}}`
- `{{if pipeline}} T1 {{else if pipeline}} T2 {{else}} T0 {{end}}`

**Analysis**:
- Conditional rendering creates variable HTML structure
- Content presence/absence depends on data evaluation
- Multiple code paths with different HTML outputs

**Strategy Decision**:
- ✅ **Tree-Based**: SUPPORTED - Hierarchical parsing handles conditional branching with nested tree structures

**Reasoning**: Conditionals create variable HTML structure that cannot be pre-determined for static boundaries. The presence/absence of HTML elements requires dynamic structural changes.

### 5. Range Iteration (`{{range}}`)

**Constructs**:
- `{{range pipeline}} T1 {{end}}`
- `{{range pipeline}} T1 {{else}} T0 {{end}}`
- `{{range $index, $element := pipeline}} T1 {{end}}`

**Analysis**:
- Dynamic repetition creates variable HTML structure
- Number of iterations depends on data
- Creates new variable scope for each iteration

**Strategy Decision**:
- ✅ **Tree-Based**: SUPPORTED - Hierarchical parsing handles range iteration with array structures and item tracking

**Reasoning**: Range loops create variable numbers of HTML elements that cannot be predicted for static fragment generation. The iteration creates new scoping contexts.

### 6. Loop Control (`{{break}}`, `{{continue}}`)

**Constructs**:
- `{{break}}` - Exit innermost range
- `{{continue}}` - Skip to next iteration

**Analysis**:
- Control flow affects range iteration behavior
- Creates conditional execution within loops
- Only valid within range contexts

**Strategy Decision**:
- 🔄 **Tree-Based**: FUTURE - Control flow statements planned for advanced implementation
- ✅ **Fallback**: SUITABLE - Full template re-rendering handles control flow correctly

**Reasoning**: Control flow statements make execution unpredictable and incompatible with static analysis.

### 7. Context Manipulation (`{{with}}`)

**Constructs**:
- `{{with pipeline}} T1 {{end}}`
- `{{with pipeline}} T1 {{else}} T0 {{end}}`

**Analysis**:
- Changes dot context for contained template
- Conditional execution based on pipeline truthiness
- Creates new evaluation scope

**Strategy Decision**:
- ✅ **Tree-Based**: SUPPORTED - Context changes handled through hierarchical boundary parsing with proper scoping

**Reasoning**: The with construct is now fully implemented in tree-based optimization. It evaluates the with field and changes the data context for nested content, with proper handling of else cases for falsy values.

### 8. Template Invocation (`{{template "name"}}`)

**Constructs**:
- `{{template "name"}}`
- `{{template "name" pipeline}}`

**Analysis**:
- Invokes external template definitions
- May pass different data contexts
- Creates template composition and reuse

**Strategy Decision**:
- 🔄 **Tree-Based**: FUTURE - Template composition planned for advanced implementation  
- ✅ **Fallback**: SUITABLE - Full template re-rendering handles template calls correctly

**Reasoning**: Template invocation creates dependencies on external templates that cannot be analyzed statically within current template boundaries.

### 9. Block Definition (`{{block "name" pipeline}}`)

**Constructs**:
- `{{block "name" pipeline}} T1 {{end}}`

**Analysis**:
- Defines overridable template sections
- Combines template definition with invocation
- Used for template inheritance patterns

**Strategy Decision**:
- ❌ **Static/Dynamic**: REJECT - Template inheritance too complex
- ❌ **Markers**: REJECT - Cannot handle inheritance hierarchies
- ⚠️  **Granular**: COMPLEX - Need inheritance resolution
- ✅ **Replacement**: SUITABLE - Full re-evaluation handles blocks

**Reasoning**: Block definitions create template inheritance patterns that require complex resolution incompatible with simple fragment strategies.

### 10. Template Definition (`{{define "name"}}`)

**Constructs**:
- `{{define "name"}} T1 {{end}}`

**Analysis**:
- Parse-time template definition
- Creates named templates for reuse
- Does not directly output content

**Strategy Decision**:
- ✅ **Static/Dynamic**: IGNORE - Definition doesn't generate content
- ✅ **Markers**: IGNORE - No direct output
- ✅ **Granular**: IGNORE - Parse-time only
- ✅ **Replacement**: IGNORE - No impact on fragments

**Reasoning**: Template definitions occur at parse time and don't generate HTML content, so they don't affect fragment generation.

### 11. Built-in Functions

#### Comparison Functions (`eq`, `ne`, `lt`, etc.)

**Constructs**: `{{if eq .A .B}}`, `{{gt .Count 5}}`

**Analysis**:
- Used within conditionals and complex expressions
- Require evaluation of multiple data fields
- Create complex boolean logic

**Strategy Decision**:
- ❌ **Static/Dynamic**: REJECT - Complex expressions break simple field access
- ⚠️  **Markers**: COMPLEX - Need expression evaluation
- ⚠️  **Granular**: COMPLEX - Boolean logic processing
- ✅ **Replacement**: SUITABLE - Full evaluation handles functions

#### Logical Functions (`and`, `or`, `not`)

**Analysis**: Similar to comparison functions - add logical complexity.

**Strategy Decision**: Same as comparison functions.

#### Utility Functions (`call`, `index`, `slice`, `len`)

**Constructs**: `{{index .Map "key"}}`, `{{len .Items}}`

**Analysis**:
- Dynamic data access and manipulation
- Results depend on data structure and content
- May return different types or fail

**Strategy Decision**:
- ❌ **Static/Dynamic**: REJECT - Dynamic access patterns too complex
- ⚠️  **Markers**: COMPLEX - Need runtime evaluation
- ⚠️  **Granular**: COMPLEX - Dynamic data handling required
- ✅ **Replacement**: SUITABLE - Full evaluation handles utility functions

### 12. Pipelines (`{{.Field | func}}`)

**Constructs**: `{{.Name | printf "Hello %s"}}`, `{{.Items | len}}`

**Analysis**:
- Chain operations for data transformation
- Multiple functions in sequence
- Create complex evaluation dependencies

**Strategy Decision**:
- ❌ **Static/Dynamic**: REJECT - Pipeline complexity breaks simple field access
- ⚠️  **Markers**: COMPLEX - Need pipeline evaluation
- ⚠️  **Granular**: COMPLEX - Multi-step processing
- ✅ **Replacement**: SUITABLE - Full evaluation handles pipelines

## Tree-Based Strategy Algorithm

### Phase 1: Template Boundary Analysis
1. Parse template to identify all constructs using hierarchical boundary detection
2. Classify each construct for tree-based compatibility
3. Build template boundary tree structure

### Phase 2: Tree-Based Processing
```go
func ProcessWithTreeBasedStrategy(templateSource string, data interface{}) (*SimpleTreeData, error) {
    // Parse template into hierarchical boundaries
    boundaries, err := parseTemplateBoundaries(templateSource)
    if err != nil {
        return nil, err
    }
    
    // Generate tree structure with static/dynamic separation
    treeData := &SimpleTreeData{
        S:        []string{},
        Dynamics: make(map[string]interface{}),
    }
    
    // Process boundaries recursively
    err = processBoundaries(boundaries, data, treeData)
    if err != nil {
        // Fallback to full template rendering for unsupported cases
        return handleFallbackRendering(templateSource, data)
    }
    
    return treeData, nil
}

func processBoundaries(boundaries []TemplateBoundary, data interface{}, treeData *SimpleTreeData) error {
    for _, boundary := range boundaries {
        switch boundary.Type {
        case StaticContent:
            treeData.S = append(treeData.S, boundary.Content)
            
        case SimpleField:
            value, err := evaluateFieldPath(boundary.FieldPath, data)
            if err != nil {
                return err
            }
            dynamicKey := fmt.Sprintf("%d", len(treeData.Dynamics))
            treeData.Dynamics[dynamicKey] = value
            
        case ConditionalIf:
            // Handle conditional branching with nested tree structure
            nestedTree, err := processConditional(boundary, data)
            if err != nil {
                return err
            }
            dynamicKey := fmt.Sprintf("%d", len(treeData.Dynamics))
            treeData.Dynamics[dynamicKey] = nestedTree
            
        case RangeLoop:
            // Handle range iteration with array of tree structures
            rangeData, err := processRange(boundary, data)
            if err != nil {
                return err
            }
            dynamicKey := fmt.Sprintf("%d", len(treeData.Dynamics))
            treeData.Dynamics[dynamicKey] = rangeData
            
        default:
            // Fallback for unsupported constructs
            return fmt.Errorf("unsupported construct: %v", boundary.Type)
        }
    }
    
    return nil
}
```

## Implementation Guidelines

### Tree-Based Strategy (Primary)
**FULLY SUPPORTED**:
- `{{.Field}}` - Simple field access with static/dynamic separation
- `{{.Nested.Field}}` - Nested field access with reflection-based evaluation
- `{{.Method}}` - Method calls with result caching
- `{{if .Condition}}...{{else}}...{{end}}` - Conditional branching with nested tree structures
- `{{range .Items}}...{{end}}` - Range iteration with array structures
- `{{with .Object}}...{{else}}...{{end}}` - Context manipulation with proper scoping and else case handling
- `{{/* comment */}}` - Comments (ignored during parsing)
- `{{define "name"}}` - Template definitions (parse-time only)

**PLANNED FOR FUTURE**:
- `{{$var := .Field}}` - Variable assignment and scoping
- `{{template "name"}}` - Template composition and includes
- `{{break}}`, `{{continue}}` - Loop control statements
- `{{.Field | func}}` - Pipeline operations

**FALLBACK HANDLING**:
- Unsupported constructs trigger full template re-rendering
- Graceful degradation maintains functionality
- Error handling with detailed context

## Testing Requirements

### Comprehensive Construct Coverage
Tree-based strategy implementation must include tests for:
1. All supported constructs (positive tests with tree structure validation)
2. All planned constructs (fallback tests ensuring graceful degradation)
3. Edge cases and nested combinations
4. Performance validation for tree generation and bandwidth savings
5. Phoenix LiveView client compatibility validation

### Tree-Based Processing Validation
- Test template boundary parsing accuracy for all Go template constructs
- Validate hierarchical tree structure generation
- Test static content caching and client-side optimization
- Ensure consistent behavior across template complexity levels
- Validate fallback behavior for unsupported edge cases

## Conclusion

This analysis provides definitive guidance for implementing tree-based optimization. The unified tree-based strategy provides comprehensive support for Go template constructs while maintaining optimal performance through static/dynamic separation and client-side caching.

**Key Principles**: 
- Single unified strategy adapts to all template patterns
- Hierarchical boundary parsing supports nested constructs
- Static content cached client-side for maximum bandwidth efficiency
- Graceful fallback maintains functionality for edge cases
- Phoenix LiveView compatibility ensures seamless client integration