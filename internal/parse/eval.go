package parse

import (
	"fmt"
	"html"
	htmltemplate "html/template"
	"reflect"
	"strings"
	"text/template/parse"
)

// sentinel distinguishes "no pipe argument" from an explicit nil.
// Identity-compared via pointer address; never written to; package-internal only.
var sentinel = &struct{}{}

// evaluator walks parse.PipeNode and parse.CommandNode directly via reflection,
// replacing the old "serialize → re-parse → execute" pattern.
type evaluator struct {
	builtins map[string]reflect.Value

	// templates holds the ASTs of recursively-invoked templates, keyed by name.
	// Populated from a Template's registry (see Parse). A {{template "x" .}} node
	// whose name is present here is evaluated at build time via invokeTemplate,
	// producing a nested TreeNode. Non-recursive {{template}} calls never reach
	// the evaluator — they are inlined during flattening — so this is empty for
	// templates that don't use recursion.
	templates map[string]*parse.Tree
}

// cachedBuiltins holds pre-reflected builtin functions, computed once.
var cachedBuiltins = func() map[string]reflect.Value {
	m := make(map[string]reflect.Value)
	for name, fn := range defaultBuiltins() {
		m[name] = reflect.ValueOf(fn)
	}
	return m
}()

func newEvaluator(funcMap htmltemplate.FuncMap) *evaluator {
	return &evaluator{builtins: precomputeBuiltins(funcMap)}
}

// precomputeBuiltins merges cachedBuiltins with the user-supplied FuncMap
// into a single immutable map. Used at parse time to avoid per-render map copies.
func precomputeBuiltins(funcMap htmltemplate.FuncMap) map[string]reflect.Value {
	merged := make(map[string]reflect.Value, len(cachedBuiltins)+len(funcMap))
	for name, fn := range cachedBuiltins {
		merged[name] = fn
	}
	for name, fn := range funcMap {
		merged[name] = reflect.ValueOf(fn)
	}
	return merged
}

// evalPipe evaluates a pipeline, threading results between commands.
// dot is the current context; data is the root data.
// Returns the raw value (not HTML-escaped).
func (e *evaluator) evalPipe(pipe *parse.PipeNode, dot interface{}, varCtx *varContext) (interface{}, error) {
	if pipe == nil {
		return nil, nil
	}

	var val interface{} = sentinel
	for _, cmd := range pipe.Cmds {
		result, err := e.evalCommand(cmd, dot, varCtx, val)
		if err != nil {
			return nil, err
		}
		val = unwrapInterface(result)
	}
	if val == sentinel {
		return nil, nil
	}
	return val, nil
}

// evalCommand evaluates a single command within a pipeline.
func (e *evaluator) evalCommand(cmd *parse.CommandNode, dot interface{}, varCtx *varContext, pipeArg interface{}) (interface{}, error) {
	if len(cmd.Args) == 0 {
		return nil, &ParseError{Phase: "eval", NodeType: "command", Msg: "empty command"}
	}

	firstWord := cmd.Args[0]
	switch n := firstWord.(type) {
	case *parse.FieldNode:
		return e.evalFieldNode(dot, n, cmd.Args[1:], varCtx, pipeArg)

	case *parse.ChainNode:
		return e.evalChainNode(dot, n, cmd.Args[1:], varCtx, pipeArg)

	case *parse.IdentifierNode:
		return e.evalFunctionCall(n.Ident, cmd.Args[1:], dot, varCtx, pipeArg)

	case *parse.PipeNode:
		// Parenthesized pipeline: (expr)
		return e.evalPipe(n, dot, varCtx)

	case *parse.VariableNode:
		return e.evalVariableNode(n, cmd.Args[1:], dot, varCtx, pipeArg)

	case *parse.DotNode:
		return dot, nil

	case *parse.BoolNode:
		return n.True, nil

	case *parse.NumberNode:
		return evalNumber(n), nil

	case *parse.StringNode:
		return n.Text, nil

	case *parse.NilNode:
		return nil, &ParseError{Phase: "eval", NodeType: "nil", Msg: "nil is not a command"}

	default:
		return nil, &ParseError{Phase: "eval", NodeType: fmt.Sprintf("%T", n), Msg: "unsupported node type"}
	}
}

// evalFieldNode evaluates a field access chain on dot: .Name, .User.Name
func (e *evaluator) evalFieldNode(dot interface{}, node *parse.FieldNode, args []parse.Node, varCtx *varContext, pipeArg interface{}) (interface{}, error) {
	val, err := resolveFieldChain(dot, node.Ident)
	if err != nil {
		return nil, &ParseError{Phase: "eval", NodeType: "field", Expr: node.String(), Err: err}
	}
	// If there are extra args or a pipe arg, this is a method call
	if len(args) > 0 || pipeArg != sentinel {
		if val == nil {
			return nil, nil // nil receiver with args — return nil gracefully
		}
		return e.callMethodOrFieldWithArgs(val, args, dot, varCtx, pipeArg)
	}
	return finalizeBareValue(val)
}

// evalChainNode evaluates a chain: (pipeline).Field1.Field2
func (e *evaluator) evalChainNode(dot interface{}, node *parse.ChainNode, args []parse.Node, varCtx *varContext, pipeArg interface{}) (interface{}, error) {
	// node.Node is a parse.Node interface — evaluate it based on its concrete type
	var pipeResult interface{}
	var err error
	switch inner := node.Node.(type) {
	case *parse.PipeNode:
		pipeResult, err = e.evalPipe(inner, dot, varCtx)
	case *parse.VariableNode:
		pipeResult, err = e.evalVariableNode(inner, nil, dot, varCtx, sentinel)
	default:
		pipeResult, err = e.evalArg(node.Node, dot, varCtx)
	}
	if err != nil {
		return nil, err
	}
	if len(node.Field) == 0 {
		return pipeResult, nil
	}
	val, err := resolveFieldChain(pipeResult, node.Field)
	if err != nil {
		return nil, &ParseError{Phase: "eval", NodeType: "chain", Expr: node.String(), Err: err}
	}
	if len(args) > 0 || pipeArg != sentinel {
		return e.callMethodOrFieldWithArgs(val, args, dot, varCtx, pipeArg)
	}
	return finalizeBareValue(val)
}

// evalVariableNode evaluates a variable reference: $var, $var.Field
func (e *evaluator) evalVariableNode(node *parse.VariableNode, args []parse.Node, dot interface{}, varCtx *varContext, pipeArg interface{}) (interface{}, error) {
	varName := node.Ident[0] // e.g. "$v"

	var val interface{}
	if varName == "$" {
		// Root variable - resolve to parent data
		if varCtx != nil {
			val = varCtx.parent
		} else {
			val = dot
		}
	} else {
		// Named variable - strip $ prefix and look up
		name := strings.TrimPrefix(varName, "$")
		if varCtx == nil {
			return nil, &ParseError{Phase: "eval", NodeType: "variable", Expr: varName, Msg: "variable referenced but no variable context"}
		}
		v, ok := varCtx.vars.Get(name)
		if !ok {
			return nil, &ParseError{Phase: "eval", NodeType: "variable", Expr: varName, Msg: fmt.Sprintf("undefined variable %q", varName)}
		}
		val = v
	}

	// Resolve field chain after variable: $var.Field1.Field2
	if len(node.Ident) > 1 {
		resolved, err := resolveFieldChain(val, node.Ident[1:])
		if err != nil {
			return nil, &ParseError{Phase: "eval", NodeType: "variable", Expr: node.String(), Err: err}
		}
		val = resolved
	}

	// If there are extra args or a pipe arg, call as method
	if len(args) > 0 || pipeArg != sentinel {
		return e.callMethodOrFieldWithArgs(val, args, dot, varCtx, pipeArg)
	}
	return finalizeBareValue(val)
}

// evalFunctionCall evaluates a function call: funcName arg1 arg2 ...
func (e *evaluator) evalFunctionCall(name string, args []parse.Node, dot interface{}, varCtx *varContext, pipeArg interface{}) (interface{}, error) {
	fn, ok := e.builtins[name]
	if !ok {
		return nil, &ParseError{Phase: "eval", NodeType: "function", Expr: name, Msg: fmt.Sprintf("function %q not defined", name)}
	}

	// Evaluate arguments
	evalArgs, err := e.evalArgs(args, dot, varCtx)
	if err != nil {
		return nil, err
	}

	// Add pipe arg as last argument if present
	if pipeArg != sentinel {
		evalArgs = append(evalArgs, pipeArg)
	}

	return callFunc(fn, evalArgs)
}

// evalArgs evaluates a slice of AST nodes into values.
func (e *evaluator) evalArgs(args []parse.Node, dot interface{}, varCtx *varContext) ([]interface{}, error) {
	result := make([]interface{}, 0, len(args))
	for _, arg := range args {
		val, err := e.evalArg(arg, dot, varCtx)
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}
	return result, nil
}

// evalArg evaluates a single argument node.
func (e *evaluator) evalArg(node parse.Node, dot interface{}, varCtx *varContext) (interface{}, error) {
	switch n := node.(type) {
	case *parse.DotNode:
		return dot, nil
	case *parse.FieldNode:
		val, err := resolveFieldChain(dot, n.Ident)
		if err != nil {
			return nil, err
		}
		return finalizeBareValue(val)
	case *parse.VariableNode:
		return e.evalVariableNode(n, nil, dot, varCtx, sentinel)
	case *parse.StringNode:
		return n.Text, nil
	case *parse.NumberNode:
		return evalNumber(n), nil
	case *parse.BoolNode:
		return n.True, nil
	case *parse.NilNode:
		return nil, nil
	case *parse.PipeNode:
		return e.evalPipe(n, dot, varCtx)
	case *parse.IdentifierNode:
		// Bare identifier as argument — look up as function
		fn, ok := e.builtins[n.Ident]
		if ok {
			return fn.Interface(), nil
		}
		return nil, &ParseError{Phase: "eval", NodeType: "arg", Expr: n.Ident, Msg: "undefined function"}
	default:
		return nil, &ParseError{Phase: "eval", NodeType: "arg", Msg: fmt.Sprintf("unsupported arg type: %T", node)}
	}
}

// callMethodOrFieldWithArgs handles the case where a field/variable result
// is followed by arguments — meaning it's a method call.
func (e *evaluator) callMethodOrFieldWithArgs(val interface{}, args []parse.Node, dot interface{}, varCtx *varContext, pipeArg interface{}) (interface{}, error) {
	// An uncalledMethod (a method resolveFieldChain left uncalled because it
	// needs args) carries the bound method to invoke with these trailing args.
	fn := reflect.ValueOf(val)
	if um, ok := val.(uncalledMethod); ok {
		fn = um.fn
	}
	if fn.Kind() != reflect.Func {
		return nil, &ParseError{Phase: "eval", NodeType: "call", Msg: fmt.Sprintf("can't call non-function value of type %T", val)}
	}
	evalArgs, err := e.evalArgs(args, dot, varCtx)
	if err != nil {
		return nil, err
	}
	if pipeArg != sentinel {
		evalArgs = append(evalArgs, pipeArg)
	}
	return callFunc(fn, evalArgs)
}

// resolveFieldChain resolves a chain of field accesses against a value.
// Supports struct fields, map keys, and methods (zero-arg or one return).
func resolveFieldChain(value interface{}, fields []string) (interface{}, error) {
	v := reflect.ValueOf(value)
	for _, field := range fields {
		// A prior field resolved to a method that needs args (uncalledMethod).
		// Taking a further field on it forces evaluation now: a variadic method
		// calls with no args (text/template parity), any other errors.
		if um, ok := value.(uncalledMethod); ok {
			called, err := um.callBare()
			if err != nil {
				return nil, err
			}
			value = called
			v = reflect.ValueOf(value)
		}

		// Try method on pre-deref value first (for pointer receivers)
		if v.IsValid() {
			if method := v.MethodByName(field); method.IsValid() {
				result, err := callMethod(method, field)
				if err != nil {
					return nil, err
				}
				value = result
				v = reflect.ValueOf(value)
				continue
			}
		}

		v = deref(v)
		if !v.IsValid() {
			// Nil mid-chain — return nil gracefully to match Go template behavior
			return nil, nil
		}

		// Try method on dereferenced value
		if method := v.MethodByName(field); method.IsValid() {
			result, err := callMethod(method, field)
			if err != nil {
				return nil, err
			}
			value = result
			v = reflect.ValueOf(value)
			continue
		}

		switch v.Kind() {
		case reflect.Struct:
			fv := v.FieldByName(field)
			if !fv.IsValid() {
				// Missing struct field — return nil to match Go template behavior
				// where {{if .MissingField}} evaluates as falsy
				return nil, nil
			}
			value = fv.Interface()
			v = fv

		case reflect.Map:
			key := reflect.ValueOf(field)
			if key.Type() != v.Type().Key() {
				// Try converting
				if key.Type().ConvertibleTo(v.Type().Key()) {
					key = key.Convert(v.Type().Key())
				} else {
					return nil, fmt.Errorf("map key type mismatch: %v vs %v", key.Type(), v.Type().Key())
				}
			}
			mv := v.MapIndex(key)
			if !mv.IsValid() {
				// Map key not found — return zero value
				value = nil
				v = reflect.Value{}
			} else {
				value = mv.Interface()
				v = reflect.ValueOf(value)
			}

		default:
			return nil, fmt.Errorf("can't evaluate field %q on type %v", field, v.Type())
		}
	}
	return value, nil
}

// callMethod invokes a zero-argument method and returns its result. A method
// that requires arguments is returned wrapped as an [uncalledMethod] rather than
// invoked — see that type for why, and for how the caller finalizes it.
func callMethod(method reflect.Value, name string) (interface{}, error) {
	if method.Type().NumIn() != 0 {
		return uncalledMethod{fn: method, name: name}, nil
	}
	return invokeNoArgs(method)
}

// invokeNoArgs calls method with no arguments and unpacks a T or (T, error)
// return, mapping a non-nil trailing error to a Go error.
func invokeNoArgs(method reflect.Value) (interface{}, error) {
	results := method.Call(nil)
	if len(results) == 0 {
		return nil, nil
	}
	if len(results) == 2 && isErrorResult(results[1]) {
		return nil, results[1].Interface().(error)
	}
	return results[0].Interface(), nil
}

// uncalledMethod holds a bound method that resolveFieldChain resolved but did
// not call because it requires arguments. Wrapping it (rather than returning the
// raw reflect method value) lets the eval nodes tell a method awaiting trailing
// args apart from a genuinely bare reference, and keeps such a value from
// leaking to valueToString as a stringified func.
type uncalledMethod struct {
	fn   reflect.Value
	name string
}

// callBare evaluates the method as a bare reference (no trailing args). A
// variadic method with only its variadic parameter is called with an empty
// variadic ({{.Tags}} calls Tags()); a method that genuinely requires arguments
// returns a ParseError matching text/template's "wrong number of args" text —
// "want N" for a fixed arity, "want at least N" for a variadic one.
func (m uncalledMethod) callBare() (interface{}, error) {
	mt := m.fn.Type()
	if mt.IsVariadic() {
		fixed := mt.NumIn() - 1 // drop the trailing variadic slot
		if fixed == 0 {
			return invokeNoArgs(m.fn)
		}
		return nil, m.argCountError(fmt.Sprintf("want at least %d", fixed))
	}
	return nil, m.argCountError(fmt.Sprintf("want %d", mt.NumIn()))
}

// argCountError builds the bare-reference "wrong number of args" ParseError,
// mirroring text/template's message ("...: <want> got 0") for method X.
func (m uncalledMethod) argCountError(want string) error {
	return &ParseError{
		Phase:    "eval",
		NodeType: "method",
		Expr:     m.name,
		Msg:      fmt.Sprintf("wrong number of args for %s: %s got 0", m.name, want),
	}
}

// finalizeBareValue converts a bare (no-trailing-args) resolution result into a
// final value: an uncalledMethod is evaluated via callBare, anything else is
// returned unchanged.
func finalizeBareValue(val interface{}) (interface{}, error) {
	if um, ok := val.(uncalledMethod); ok {
		return um.callBare()
	}
	return val, nil
}

// isErrorResult checks if a reflect.Value contains a non-nil error.
// Handles both nilable and non-nilable types safely.
var errorType = reflect.TypeOf((*error)(nil)).Elem()

func isErrorResult(v reflect.Value) bool {
	if !v.IsValid() || !v.Type().Implements(errorType) {
		return false
	}
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		return !v.IsNil()
	}
	return true
}

// callFunc calls a function with the given arguments using reflection.
// Handles variadic functions and (T, error) return patterns.
// Per Go template conventions, functions must return either T or (T, error).
// If a two-value return's second value doesn't implement error, it is ignored.
func callFunc(fn reflect.Value, args []interface{}) (result interface{}, err error) {
	// Recover from panics in user-provided functions (type mismatches, nil derefs, etc.)
	defer func() {
		if r := recover(); r != nil {
			err = &ParseError{Phase: "eval", NodeType: "call", Msg: fmt.Sprintf("panic calling function: %v", r)}
		}
	}()

	ft := fn.Type()
	numIn := ft.NumIn()
	isVariadic := ft.IsVariadic()

	// Validate argument count for non-variadic functions
	if !isVariadic && len(args) != numIn {
		return nil, &ParseError{
			Phase: "eval", NodeType: "call",
			Msg: fmt.Sprintf("wrong number of args: got %d, want %d", len(args), numIn),
		}
	}
	if isVariadic && len(args) < numIn-1 {
		return nil, &ParseError{
			Phase: "eval", NodeType: "call",
			Msg: fmt.Sprintf("not enough args: got %d, want at least %d", len(args), numIn-1),
		}
	}

	// Build reflect.Value arguments
	in := make([]reflect.Value, 0, len(args))
	if isVariadic {
		fixedCount := numIn - 1
		for i, arg := range args {
			if i < fixedCount {
				in = append(in, convertArg(arg, ft.In(i)))
			} else {
				elemType := ft.In(numIn - 1).Elem()
				in = append(in, convertArg(arg, elemType))
			}
		}
	} else {
		for i, arg := range args {
			in = append(in, convertArg(arg, ft.In(i)))
		}
	}

	results := fn.Call(in)

	if len(results) == 0 {
		return nil, nil
	}
	if len(results) == 2 && isErrorResult(results[1]) {
		return nil, results[1].Interface().(error)
	}
	return results[0].Interface(), nil
}

// convertArg converts an argument to the expected type.
func convertArg(arg interface{}, expectedType reflect.Type) reflect.Value {
	if arg == nil {
		return reflect.Zero(expectedType)
	}
	v := reflect.ValueOf(arg)
	if !v.IsValid() {
		return reflect.Zero(expectedType)
	}
	// If the type matches or is assignable, use directly
	if v.Type().AssignableTo(expectedType) {
		return v
	}
	// If convertible, convert
	if v.Type().ConvertibleTo(expectedType) {
		return v.Convert(expectedType)
	}
	// If expected is interface{}, wrap
	if expectedType.Kind() == reflect.Interface {
		return v
	}
	// Last resort: return as-is and let reflect.Call panic with a clear message
	return v
}

// evalNumber extracts the appropriate numeric value from a NumberNode.
func evalNumber(n *parse.NumberNode) interface{} {
	if n.IsComplex {
		return n.Complex128
	}
	if n.IsFloat && strings.ContainsAny(n.Text, ".eEpP") {
		return n.Float64
	}
	if n.IsInt {
		return int(n.Int64)
	}
	if n.IsUint {
		return n.Uint64
	}
	return n.Float64
}

// deref dereferences a reflect.Value through pointers and interfaces.
func deref(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

// unwrapInterface unwraps an interface value to its underlying value.
func unwrapInterface(v interface{}) interface{} {
	rv := reflect.ValueOf(v)
	if rv.IsValid() && rv.Kind() == reflect.Interface && rv.Type().NumMethod() == 0 {
		return rv.Elem().Interface()
	}
	return v
}

// isTrue reports whether a value is "truthy" following Go template semantics.
// Matches text/template.IsTrue exactly.
func isTrue(val interface{}) bool {
	if val == nil {
		return false
	}
	v := reflect.ValueOf(val)
	if !v.IsValid() {
		return false
	}
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() > 0
	case reflect.Bool:
		return v.Bool()
	case reflect.Complex64, reflect.Complex128:
		return v.Complex() != 0
	case reflect.Chan, reflect.Func, reflect.Pointer, reflect.UnsafePointer, reflect.Interface:
		return !v.IsNil()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() != 0
	case reflect.Float32, reflect.Float64:
		return v.Float() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() != 0
	case reflect.Struct:
		return true
	default:
		return false
	}
}

// valueToString converts a value to its HTML-escaped string representation
// for inclusion in tree dynamics. Matches html/template escaping behavior.
func valueToString(val interface{}) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case htmltemplate.HTML:
		return string(v)
	case htmltemplate.URL:
		return string(v)
	case htmltemplate.JS:
		return string(v)
	case htmltemplate.CSS:
		return string(v)
	case htmltemplate.HTMLAttr:
		return string(v)
	case string:
		return html.EscapeString(v)
	case fmt.Stringer:
		return html.EscapeString(v.String())
	default:
		return html.EscapeString(fmt.Sprint(v))
	}
}

// defaultBuiltins returns the built-in template functions matching
// text/template/funcs.go builtins.
func defaultBuiltins() htmltemplate.FuncMap {
	return htmltemplate.FuncMap{
		"and":      builtinAnd,
		"or":       builtinOr,
		"not":      builtinNot,
		"len":      builtinLen,
		"print":    fmt.Sprint,
		"printf":   fmt.Sprintf,
		"println":  fmt.Sprintln,
		"eq":       builtinEq,
		"ne":       builtinNe,
		"lt":       builtinLt,
		"le":       builtinLe,
		"gt":       builtinGt,
		"ge":       builtinGe,
		"index":    builtinIndex,
		"html":     htmltemplate.HTMLEscaper,
		"js":       htmltemplate.JSEscaper,
		"urlquery": htmltemplate.URLQueryEscaper,
		"call":     builtinCall,
		"slice":    builtinSlice,
	}
}

// builtinAnd returns the first falsy argument, or the last argument.
func builtinAnd(args ...interface{}) interface{} {
	var last interface{}
	for _, arg := range args {
		last = arg
		if !isTrue(arg) {
			return arg
		}
	}
	return last
}

// builtinOr returns the first truthy argument, or the last argument.
func builtinOr(args ...interface{}) interface{} {
	var last interface{}
	for _, arg := range args {
		last = arg
		if isTrue(arg) {
			return arg
		}
	}
	return last
}

func builtinNot(arg interface{}) bool {
	return !isTrue(arg)
}

func builtinLen(item interface{}) (int, error) {
	v := reflect.ValueOf(item)
	if !v.IsValid() {
		return 0, fmt.Errorf("len of nil")
	}
	switch v.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return v.Len(), nil
	default:
		return 0, fmt.Errorf("len of type %s", v.Type())
	}
}

// builtinEq matches Go template's eq semantics: eq x y [y2 y3 ...]
// Returns true if x equals any of the y values.
func builtinEq(args ...interface{}) (bool, error) {
	if len(args) < 2 {
		return false, fmt.Errorf("eq requires at least 2 arguments")
	}
	x := args[0]
	for _, y := range args[1:] {
		if reflect.DeepEqual(x, y) {
			return true, nil
		}
		// Try numeric comparison for mixed types
		if numericEqual(x, y) {
			return true, nil
		}
	}
	return false, nil
}

func builtinNe(x, y interface{}) (bool, error) {
	eq, err := builtinEq(x, y)
	return !eq, err
}

func builtinLt(x, y interface{}) (bool, error) {
	return compareValues(x, y, func(c int) bool { return c < 0 })
}

func builtinLe(x, y interface{}) (bool, error) {
	return compareValues(x, y, func(c int) bool { return c <= 0 })
}

func builtinGt(x, y interface{}) (bool, error) {
	return compareValues(x, y, func(c int) bool { return c > 0 })
}

func builtinGe(x, y interface{}) (bool, error) {
	return compareValues(x, y, func(c int) bool { return c >= 0 })
}

// compareValues compares two values and returns the comparison result.
// Works for ints, floats, and strings.
func compareValues(x, y interface{}, check func(int) bool) (bool, error) {
	vx := reflect.ValueOf(x)
	vy := reflect.ValueOf(y)

	if !vx.IsValid() || !vy.IsValid() {
		return false, fmt.Errorf("can't compare nil values")
	}

	// Numeric comparison
	switch {
	case isInt(vx) && isInt(vy):
		return check(intCompare(toInt64(vx), toInt64(vy))), nil
	case isUint(vx) && isUint(vy):
		return check(uintCompare(toUint64(vx), toUint64(vy))), nil
	case isFloat(vx) || isFloat(vy):
		return check(floatCompare(toFloat64(vx), toFloat64(vy))), nil
	case isInt(vx) && isUint(vy):
		return check(intCompare(toInt64(vx), int64(toUint64(vy)))), nil
	case isUint(vx) && isInt(vy):
		return check(intCompare(int64(toUint64(vx)), toInt64(vy))), nil
	}

	// String comparison
	if vx.Kind() == reflect.String && vy.Kind() == reflect.String {
		return check(strings.Compare(vx.String(), vy.String())), nil
	}

	return false, fmt.Errorf("can't compare %v (%T) with %v (%T)", x, x, y, y)
}

func isInt(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	}
	return false
}

func isUint(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	}
	return false
}

func isFloat(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func toInt64(v reflect.Value) int64   { return v.Int() }
func toUint64(v reflect.Value) uint64 { return v.Uint() }

func toFloat64(v reflect.Value) float64 {
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		return v.Float()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint())
	}
	return 0
}

func intCompare(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func uintCompare(a, b uint64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func floatCompare(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func numericEqual(x, y interface{}) bool {
	vx := reflect.ValueOf(x)
	vy := reflect.ValueOf(y)
	if !vx.IsValid() || !vy.IsValid() {
		return false
	}
	// Both numeric — compare as float64
	if (isInt(vx) || isUint(vx) || isFloat(vx)) && (isInt(vy) || isUint(vy) || isFloat(vy)) {
		return toFloat64(vx) == toFloat64(vy)
	}
	return false
}

func builtinIndex(item interface{}, indices ...interface{}) (interface{}, error) {
	v := reflect.ValueOf(item)
	for _, idx := range indices {
		v = deref(v)
		if !v.IsValid() {
			return nil, fmt.Errorf("index of nil pointer")
		}
		switch v.Kind() {
		case reflect.Array, reflect.Slice:
			i, err := toIntIndex(idx)
			if err != nil {
				return nil, err
			}
			if i < 0 || i >= v.Len() {
				return nil, fmt.Errorf("index out of range: %d", i)
			}
			v = v.Index(i)
		case reflect.Map:
			key := reflect.ValueOf(idx)
			if key.Type() != v.Type().Key() && key.Type().ConvertibleTo(v.Type().Key()) {
				key = key.Convert(v.Type().Key())
			}
			result := v.MapIndex(key)
			if !result.IsValid() {
				v = reflect.Zero(v.Type().Elem())
			} else {
				v = result
			}
		default:
			return nil, fmt.Errorf("can't index item of type %s", v.Type())
		}
	}
	return v.Interface(), nil
}

func toIntIndex(idx interface{}) (int, error) {
	v := reflect.ValueOf(idx)
	if isInt(v) {
		return int(v.Int()), nil
	}
	if isUint(v) {
		return int(v.Uint()), nil
	}
	return 0, fmt.Errorf("can't use %v (%T) as array index", idx, idx)
}

func builtinCall(fn interface{}, args ...interface{}) (interface{}, error) {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		return nil, fmt.Errorf("call of non-function")
	}
	return callFunc(v, args)
}

func builtinSlice(item interface{}, indices ...interface{}) (interface{}, error) {
	v := reflect.ValueOf(item)
	if !v.IsValid() {
		return nil, fmt.Errorf("slice of nil")
	}
	switch len(indices) {
	case 0:
		return v.Slice(0, v.Len()).Interface(), nil
	case 1:
		i, err := toIntIndex(indices[0])
		if err != nil {
			return nil, err
		}
		return v.Slice(i, v.Len()).Interface(), nil
	case 2:
		i, err := toIntIndex(indices[0])
		if err != nil {
			return nil, err
		}
		j, err := toIntIndex(indices[1])
		if err != nil {
			return nil, err
		}
		return v.Slice(i, j).Interface(), nil
	case 3:
		i, err := toIntIndex(indices[0])
		if err != nil {
			return nil, err
		}
		j, err := toIntIndex(indices[1])
		if err != nil {
			return nil, err
		}
		k, err := toIntIndex(indices[2])
		if err != nil {
			return nil, err
		}
		// 3-index slicing (x[i:j:k], capacity bound) is only valid on slices and
		// arrays; reflect.Slice3 panics on a string, so reject it like text/template.
		if v.Kind() == reflect.String {
			return nil, fmt.Errorf("cannot 3-index slice a string")
		}
		return v.Slice3(i, j, k).Interface(), nil
	default:
		return nil, fmt.Errorf("too many indices for slice")
	}
}
