package parse

import (
	"fmt"
	"text/template/parse"
)

// defaultMaxInvocationDepth bounds recursive {{template}} invocation when the
// caller has not set Context.MaxInvocationDepth. It sits well below Go's build
// stack limit, so it trips before a self-referential data graph or a
// pathologically deep tree can overflow walkAST.
const defaultMaxInvocationDepth = 128

// maxInvocationDepth returns the effective recursion cap: the caller's override
// if positive, otherwise the built-in default. ctx is always non-nil here
// (invokeTemplate dereferences it before calling this).
func maxInvocationDepth(ctx *Context) int {
	if ctx.MaxInvocationDepth > 0 {
		return ctx.MaxInvocationDepth
	}
	return defaultMaxInvocationDepth
}

// invokeTemplate evaluates a recursive {{template "name" pipe}} node at build
// time, returning the invoked body as a nested TreeNode occupying a single
// dynamic slot in the caller's tree (mirroring how handleIf wraps a branch).
//
// Non-recursive {{template}} calls never reach here — they are inlined during
// flattening. Only names left un-inlined by FlattenTemplate (the cycle members)
// are present in eval.templates and routed through this path.
func invokeTemplate(n *parse.TemplateNode, eval *evaluator, data interface{}, varCtx *varContext, ctx *Context) (*TreeNode, error) {
	body, ok := eval.templates[n.Name]
	if !ok {
		// A {{template}} that survived flattening but has no registered body means
		// detection and flattening disagreed — treat as a build error rather than
		// silently rendering nothing.
		return nil, &ParseError{
			Phase: "build", NodeType: "template", Expr: n.Name,
			Msg: fmt.Sprintf("template %q invoked at runtime but not found in the recursion registry", n.Name),
		}
	}

	limit := maxInvocationDepth(ctx)
	if ctx.InvocationDepth >= limit {
		return nil, &ParseError{
			Phase: "build", NodeType: "template", Expr: n.Name,
			Msg: fmt.Sprintf("recursive template %q exceeded the maximum invocation depth of %d "+
				"(possible infinite recursion in the data; raise the limit with WithMaxTemplateDepth if the "+
				"nesting is legitimately deeper)", n.Name, limit),
		}
	}

	// Go rebinds dot on a template call to the pipe value; a no-argument
	// {{template "name"}} rebinds dot to nil (NOT the caller's dot). Matching
	// that keeps the tree path byte-identical to html/template Execute. The pipe,
	// when present, still resolves against the caller's dot (e.g. {{template "x"
	// .Field}} reads .Field from the caller).
	var invocationDot interface{}
	if n.Pipe != nil {
		v, err := eval.evalPipe(n.Pipe, dot(data, varCtx), varCtx)
		if err != nil {
			return nil, &ParseError{Phase: "eval", NodeType: "template", Expr: n.Name, Err: err}
		}
		invocationDot = v
	}

	// The invoked template gets a fresh scope: dot = the pipe value, and none of
	// the caller's $variables carry over (Go hands an invoked template only its
	// argument). A nil varCtx is exactly that clean scope — walkList lazily
	// re-inits one rooted at invocationDot if the body declares its own vars.
	childCtx := *ctx
	childCtx.InvocationDepth++
	childTree, err := walkAST(body.Root, eval, invocationDot, nil, &childCtx)
	if err != nil {
		return nil, err
	}
	// Wrap the invoked body in its own dynamic slot. The WrapperInvocation tag is
	// what lets range keying recognise this later: the node is otherwise
	// indistinguishable from a conditional wrapper or a field node (issue #497).
	// This isolation is why an invocation wraps where {{with}} does not: a
	// recursive subtree's depth/shape varies with the data, so it must occupy one
	// slot to keep those changes from restructuring the parent's statics — the
	// same diffing rationale {{if}} wraps for.
	return createWrapper(childTree, ctx, WrapperInvocation), nil
}
