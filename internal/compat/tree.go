package compat

import (
	"fmt"
	htmltemplate "html/template"

	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/keys"
	"github.com/livetemplate/livetemplate/internal/parse"
	"github.com/livetemplate/livetemplate/internal/render"
)

// Wrapper functions for backward compatibility

// generateRandomID wraps internal/build.GenerateRandomID for backward compatibility
func GenerateRandomID() string {
	return build.GenerateRandomID()
}

// injectWrapperDiv wraps internal/build.InjectWrapperDiv for backward compatibility
func InjectWrapperDiv(htmlDoc string, wrapperID string, loadingDisabled bool) string {
	return build.InjectWrapperDiv(htmlDoc, wrapperID, loadingDisabled)
}

// extractTemplateBodyContent wraps internal/build.ExtractTemplateBodyContent for backward compatibility
func ExtractTemplateBodyContent(templateStr string) string {
	return build.ExtractTemplateBodyContent(templateStr)
}

// ExtractTemplateBodyContentSliced wraps internal/build.ExtractTemplateBodyContentSliced.
func ExtractTemplateBodyContentSliced(templateStr string) (body string, sliced bool) {
	return build.ExtractTemplateBodyContentSliced(templateStr)
}

// extractTemplateContent wraps internal/build.ExtractTemplateContent for backward compatibility
func ExtractTemplateContent(input string, wrapperID string) string {
	return build.ExtractTemplateContent(input, wrapperID)
}

// normalizeTemplateSpacing wraps internal/build.NormalizeTemplateSpacing for backward compatibility
func NormalizeTemplateSpacing(templateStr string) string {
	return build.NormalizeTemplateSpacing(templateStr)
}

// Rendering wrappers for backward compatibility

// renderTreeToHTML wraps internal/render.TreeToHTML for backward compatibility (used in tests)
func RenderTreeToHTML(tree map[string]interface{}) (string, error) {
	return render.TreeToHTML(tree)
}

// detectIDKey wraps internal/keys.DetectIDKey for backward compatibility
func DetectIDKey(statics []string) string {
	return keys.DetectIDKey(statics)
}

// parseTemplateToTree parses a template using the internal/parse package
// templateName is used for expression caching
// ctx is optional - if nil, defaults to first-render context (includes statics)
func ParseTemplateToTree(templateName, templateStr string, data interface{}, ctx ...*build.Context) (tree *build.TreeNode, err error) {
	// Get or create context
	var genCtx *build.Context
	if len(ctx) > 0 {
		genCtx = ctx[0]
	}
	if genCtx == nil {
		genCtx = build.NewContext()
	}

	// Set template name for expression caching
	genCtx.TemplateName = templateName

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

	return parse.BuildTree(tmpl, data, genCtx)
}

// ParseAndCacheTemplate parses a template string into a reusable *parse.Template
// that can be passed to BuildTreeFromCached on subsequent renders to skip re-parsing.
func ParseAndCacheTemplate(templateStr string, funcMap htmltemplate.FuncMap) (*parse.Template, error) {
	return parse.Parse(templateStr, funcMap)
}

// BuildTreeFromCached builds a tree from a previously parsed template, skipping
// the parse step entirely. This avoids re-parsing the same template on every render.
func BuildTreeFromCached(tmpl *parse.Template, data interface{}, ctx *build.Context) (tree *build.TreeNode, err error) {
	genCtx := ctx
	if genCtx == nil {
		genCtx = build.NewContext()
	}

	if !genCtx.DevMode {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("template execution panic: %v", r)
			}
		}()
	}

	return parse.BuildTree(tmpl, data, genCtx)
}
