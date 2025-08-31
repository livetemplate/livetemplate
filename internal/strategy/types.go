package strategy

// TemplateBoundaryType represents different types of template constructs
type TemplateBoundaryType int

const (
	// StaticContent represents literal HTML content
	StaticContent TemplateBoundaryType = iota

	// SimpleField represents a simple field access like {{.Name}}
	SimpleField

	// Comment represents template comments {{/* */}}
	Comment

	// ConditionalIf represents {{if}} blocks
	ConditionalIf

	// ConditionalElse represents {{else}} in conditional blocks
	ConditionalElse

	// ConditionalEnd represents {{end}} for conditional blocks
	ConditionalEnd

	// RangeLoop represents {{range}} blocks
	RangeLoop

	// RangeElse represents {{else}} in range blocks
	RangeElse

	// RangeEnd represents {{end}} for range blocks
	RangeEnd

	// ContextWith represents {{with}} blocks
	ContextWith

	// WithElse represents {{else}} in with blocks
	WithElse

	// WithEnd represents {{end}} for with blocks
	WithEnd

	// Variable represents variable declarations and assignments
	Variable

	// TemplateInvocation represents {{template}} calls
	TemplateInvocation

	// TemplateDefinition represents {{define}} blocks
	TemplateDefinition

	// BlockDefinition represents {{block}} definitions
	BlockDefinition

	// Pipeline represents pipeline operations
	Pipeline

	// Function represents function calls
	Function

	// LoopControl represents {{break}} and {{continue}}
	LoopControl

	// Complex represents complex constructs that don't fit other categories
	Complex
)

// TemplateBoundary represents a boundary in template parsing
type TemplateBoundary struct {
	Type      TemplateBoundaryType `json:"type"`
	Content   string               `json:"content"`
	Start     int                  `json:"start"`
	End       int                  `json:"end"`
	FieldPath string               `json:"field_path,omitempty"` // For SimpleField types

	// For structured constructs (conditionals, ranges, with blocks)
	TrueBlock  []TemplateBoundary `json:"true_block,omitempty"`
	FalseBlock []TemplateBoundary `json:"false_block,omitempty"`
	Condition  string             `json:"condition,omitempty"`
}

// String returns string representation of boundary type
func (t TemplateBoundaryType) String() string {
	switch t {
	case StaticContent:
		return "StaticContent"
	case SimpleField:
		return "SimpleField"
	case Comment:
		return "Comment"
	case ConditionalIf:
		return "ConditionalIf"
	case ConditionalElse:
		return "ConditionalElse"
	case ConditionalEnd:
		return "ConditionalEnd"
	case RangeLoop:
		return "RangeLoop"
	case RangeElse:
		return "RangeElse"
	case RangeEnd:
		return "RangeEnd"
	case ContextWith:
		return "ContextWith"
	case WithElse:
		return "WithElse"
	case WithEnd:
		return "WithEnd"
	case Variable:
		return "Variable"
	case TemplateInvocation:
		return "TemplateInvocation"
	case TemplateDefinition:
		return "TemplateDefinition"
	case BlockDefinition:
		return "BlockDefinition"
	case Pipeline:
		return "Pipeline"
	case Function:
		return "Function"
	case LoopControl:
		return "LoopControl"
	case Complex:
		return "Complex"
	default:
		return "Unknown"
	}
}
