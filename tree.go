package livetemplate

import (
	"fmt"

	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/keys"
	"github.com/livetemplate/livetemplate/internal/parse"
	"github.com/livetemplate/livetemplate/internal/render"
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

// renderTreeToHTML wraps internal/render.TreeToHTML for backward compatibility (used in tests)
func renderTreeToHTML(tree map[string]interface{}) (string, error) {
	return render.TreeToHTML(tree)
}

// Key generation wrappers for backward compatibility

// keyGenerator is a type alias for backward compatibility
type keyGenerator = keys.Generator

// newKeyGenerator wraps internal/keys.NewGenerator for backward compatibility
func newKeyGenerator() *keyGenerator {
	return keys.NewGenerator()
}

// keyGeneratorAdapter adapts keyGenerator to parse.KeyGenerator interface.
//
// Trade-off: This adapter panics on error instead of propagating errors because
// the parse.KeyGenerator interface doesn't support error returns. This is acceptable
// because:
// 1. Key generation errors only occur on counter overflow (after 2^63-1 keys on 64-bit systems)
// 2. This is effectively impossible in practice - would require generating billions of keys per second for decades
// 3. Updating the interface would break backward compatibility in v0.2.0
//
// Future consideration: If error handling is critical for your use case, the parse.KeyGenerator
// interface could be updated in a future major version to return (string, error).
type keyGeneratorAdapter struct {
	kg *keyGenerator
}

// Next implements parse.KeyGenerator interface.
// Panics on key generation failure (counter overflow), which should never occur in practice.
func (kga *keyGeneratorAdapter) Next() string {
	key, err := kga.kg.NextKey()
	if err != nil {
		// Counter overflow - extremely unlikely (requires 2^63-1 keys)
		// Panic since interface doesn't support error returns
		panic(fmt.Sprintf("key generation failed: %v", err))
	}
	return key
}

// detectIDKey wraps internal/keys.DetectIDKey for backward compatibility
func detectIDKey(statics []string) string {
	return keys.DetectIDKey(statics)
}

// generateWrapperKey generates a wrapper key using the key generator.
//
// Trade-off: Panics on error for backward compatibility with callers expecting
// a simple string return. Key generation errors only occur on counter overflow
// (after 2^63-1 keys), which is effectively impossible in real-world usage.
func generateWrapperKey(keyGen *keyGenerator) string {
	key, err := keyGen.NextKey()
	if err != nil {
		// Counter overflow - extremely unlikely (requires 2^63-1 keys)
		// Panic for backward compatibility with existing callers
		panic(fmt.Sprintf("key generation failed: %v", err))
	}
	return key
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
