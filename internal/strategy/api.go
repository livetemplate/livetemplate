package strategy

// Differ provides the unified API for template tree generation
type Differ interface {
	GenerateTree(data interface{}) ([]byte, error)
	Reset()
}

// NewDiffer creates a new differ with the best available implementation
// This is the single entry point for all template diffing functionality
func NewDiffer(templateStr string) (Differ, error) {
	// Use tree-based optimization with intelligent caching
	// providing the best overall performance and adaptive strategies
	return newInternalDiffer(templateStr)
}