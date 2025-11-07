package livetemplate

import (
	"fmt"

	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/parse"
)

// Fingerprinting wrappers for backward compatibility

// calculateFingerprint wraps internal/build.CalculateFingerprint for backward compatibility
func calculateFingerprint(tree *TreeNode) string {
	return build.CalculateFingerprint(tree)
}

// addFingerprintToTree wraps internal/build.AddFingerprintToTree for backward compatibility
func addFingerprintToTree(tree *TreeNode) *TreeNode {
	return build.AddFingerprintToTree(tree)
}

// Wrapper functions for backward compatibility

// generateRandomID wraps internal/build.GenerateRandomID for backward compatibility
func generateRandomID() string {
	return build.GenerateRandomID()
}

// injectWrapperDiv wraps internal/build.InjectWrapperDiv for backward compatibility
func injectWrapperDiv(htmlDoc string, wrapperID string, loadingDisabled bool) string {
	return build.InjectWrapperDiv(htmlDoc, wrapperID, loadingDisabled)
}

// extractTemplateBodyContent wraps internal/build.ExtractTemplateBodyContent for backward compatibility
func extractTemplateBodyContent(templateStr string) string {
	return build.ExtractTemplateBodyContent(templateStr)
}

// extractTemplateContent wraps internal/build.ExtractTemplateContent for backward compatibility
func extractTemplateContent(input string, wrapperID string) string {
	return build.ExtractTemplateContent(input, wrapperID)
}

// normalizeTemplateSpacing wraps internal/build.NormalizeTemplateSpacing for backward compatibility
func normalizeTemplateSpacing(templateStr string) string {
	return build.NormalizeTemplateSpacing(templateStr)
}

// Rendering wrappers for backward compatibility

// renderTreeToHTML wraps internal/build.RenderTreeToHTML for backward compatibility (used in tests)
func renderTreeToHTML(tree map[string]interface{}) (string, error) {
	return build.RenderTreeToHTML(tree)
}

// Key generation wrappers for backward compatibility

// keyGenerator is a type alias for backward compatibility
type keyGenerator = build.KeyGenerator

// newKeyGenerator wraps internal/build.NewKeyGenerator for backward compatibility
func newKeyGenerator() *keyGenerator {
	return build.NewKeyGenerator()
}

// keyGeneratorAdapter adapts keyGenerator to parse.KeyGenerator interface
type keyGeneratorAdapter struct {
	kg *keyGenerator
}

// Next implements parse.KeyGenerator interface
func (kga *keyGeneratorAdapter) Next() string {
	return kga.kg.NextKey()
}

// detectIDKey wraps internal/build.DetectIDKey for backward compatibility
func detectIDKey(statics []string) string {
	return build.DetectIDKey(statics)
}

// generateWrapperKey generates a wrapper key using the key generator
func generateWrapperKey(keyGen *keyGenerator) string {
	return keyGen.NextKey()
}

// parseTemplateToTree parses a template using the internal/parse package
// ctx is optional - if nil, defaults to first-render context (includes statics)
func parseTemplateToTree(templateStr string, data interface{}, keyGen *keyGenerator, ctx ...*TreeGenerationContext) (tree *TreeNode, err error) {
	// Get or create context
	var genCtx *TreeGenerationContext
	if len(ctx) > 0 {
		genCtx = ctx[0]
	}
	if genCtx == nil {
		genCtx = NewTreeGenerationContext()
	}

	// Recover from panics in template execution (can happen with fuzz-generated templates)
	// In DevMode, panics are not caught to aid debugging
	if !genCtx.DevMode {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("template execution panic: %v", r)
			}
		}()
	}

	// Parse template
	tmpl, err := parse.Parse(templateStr, genCtx.FuncMap)
	if err != nil {
		return nil, err
	}

	// Build tree using internal/parse package
	// Create adapter for keyGenerator to match parse.KeyGenerator interface
	keyGenAdapter := &keyGeneratorAdapter{kg: keyGen}
	return parse.BuildTree(tmpl, data, keyGenAdapter, genCtx)
}
