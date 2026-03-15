package parse

import (
	"strings"
	"text/template/parse"
)

// varContext holds variable bindings for template execution.
type varContext struct {
	parent interface{} // Original root data (for $ access)
	vars   orderedVars // Variable bindings ($index, $value, etc.)
	dot    interface{} // Current dot context
}

// varPair represents a single variable binding.
type varPair struct {
	key   string
	value interface{}
}

// orderedVars is a deterministic ordered map that preserves insertion order.
type orderedVars struct {
	pairs []varPair
	index map[string]int
}

const defaultVarCapacity = 2

func newOrderedVars() orderedVars {
	return orderedVars{
		pairs: make([]varPair, 0, defaultVarCapacity),
		index: make(map[string]int, defaultVarCapacity),
	}
}

func (ov *orderedVars) Set(key string, value interface{}) {
	if key == "" {
		return
	}
	if pos, exists := ov.index[key]; exists {
		ov.pairs[pos].value = value
		return
	}
	pos := len(ov.pairs)
	ov.pairs = append(ov.pairs, varPair{key: key, value: value})
	ov.index[key] = pos
}

func (ov orderedVars) Get(key string) (interface{}, bool) {
	pos, exists := ov.index[key]
	if !exists {
		return nil, false
	}
	return ov.pairs[pos].value, true
}

func (ov orderedVars) Len() int {
	return len(ov.pairs)
}

func (ov orderedVars) Range(fn func(key string, value interface{})) {
	for _, pair := range ov.pairs {
		fn(pair.key, pair.value)
	}
}

// registerVarDeclaration evaluates a variable declaration and registers it in varCtx.
func registerVarDeclaration(eval *evaluator, actionNode *parse.ActionNode, varCtx *varContext, ctx *Context) error {
	for _, decl := range actionNode.Pipe.Decl {
		varName := strings.TrimPrefix(decl.Ident[0], "$")
		if len(actionNode.Pipe.Cmds) == 0 {
			return &ParseError{
				Phase: "eval", NodeType: "var",
				Expr: "$" + varName,
				Msg:  "variable declaration has no right-hand side expression",
			}
		}
		// Evaluate the RHS pipe against the current dot context
		value, err := eval.evalPipe(actionNode.Pipe, varCtx.dot, varCtx)
		if err != nil {
			return &ParseError{
				Phase: "eval", NodeType: "var",
				Expr: "$" + varName,
				Err:  err,
			}
		}
		varCtx.vars.Set(varName, value)
	}
	return nil
}
